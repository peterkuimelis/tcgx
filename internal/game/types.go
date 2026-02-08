package game

import "fmt"

// --- Enums ---

type Phase int

const (
	PhaseNone Phase = iota
	PhaseDraw
	PhaseStandby
	PhaseMain1
	PhaseBattle
	PhaseMain2
	PhaseEnd
)

func (p Phase) String() string {
	switch p {
	case PhaseDraw:
		return "Draw Phase"
	case PhaseStandby:
		return "Standby Phase"
	case PhaseMain1:
		return "Main Phase 1"
	case PhaseBattle:
		return "Battle Phase"
	case PhaseMain2:
		return "Main Phase 2"
	case PhaseEnd:
		return "End Phase"
	default:
		return "None"
	}
}

type BattleStep int

const (
	BattleStepNone BattleStep = iota
	BattleStepStart
	BattleStepBattle
	BattleStepDamage
	BattleStepEnd
)

func (s BattleStep) String() string {
	switch s {
	case BattleStepStart:
		return "Start Step"
	case BattleStepBattle:
		return "Battle Step"
	case BattleStepDamage:
		return "Damage Step"
	case BattleStepEnd:
		return "End Step"
	default:
		return ""
	}
}

type Position int

const (
	PositionATK Position = iota
	PositionDEF
)

func (p Position) String() string {
	if p == PositionATK {
		return "ATK"
	}
	return "DEF"
}

type FaceStatus int

const (
	FaceUp FaceStatus = iota
	FaceDown
)

func (f FaceStatus) String() string {
	if f == FaceUp {
		return "face-up"
	}
	return "face-down"
}

type CardType int

const (
	CardTypeAgent CardType = iota
	CardTypeProgram
	CardTypeTrap
)

func (ct CardType) String() string {
	switch ct {
	case CardTypeAgent:
		return "Agent"
	case CardTypeProgram:
		return "Program"
	case CardTypeTrap:
		return "Trap"
	default:
		return "Unknown"
	}
}

type ProgramSubtype int

const (
	ProgramNormal ProgramSubtype = iota
	ProgramQuickPlay
	ProgramContinuous
	ProgramEquip
	ProgramOS
)

type TrapSubtype int

const (
	TrapNormal TrapSubtype = iota
	TrapContinuous
	TrapCounter
)

type Attribute int

const (
	AttrNone Attribute = iota
	AttrLIGHT
	AttrDARK
	AttrEARTH
	AttrWATER
	AttrFIRE
	AttrWIND
	AttrDIVINE
)

func (a Attribute) String() string {
	switch a {
	case AttrLIGHT:
		return "LIGHT"
	case AttrDARK:
		return "DARK"
	case AttrEARTH:
		return "EARTH"
	case AttrWATER:
		return "WATER"
	case AttrFIRE:
		return "FIRE"
	case AttrWIND:
		return "WIND"
	case AttrDIVINE:
		return "DIVINE"
	default:
		return ""
	}
}

type ExecSpeed int

const (
	ExecSpeed1 ExecSpeed = 1
	ExecSpeed2 ExecSpeed = 2
	ExecSpeed3 ExecSpeed = 3
)

// --- Card definition (static, from DB/scripts) ---

type Card struct {
	Name        string
	Description string
	CardType    CardType
	Level       int
	Attribute   Attribute
	AgentType   string // e.g. "Enforcer", "Hacker"
	ATK         int
	DEF         int
	IsEffect    bool
	ProgramSub  ProgramSubtype
	TrapSub     TrapSubtype
	Effects     []*CardEffect
}

func (c *Card) String() string {
	return c.Name
}

// SacrificesRequired returns the number of sacrifices needed to normal summon/set this agent.
func (c *Card) SacrificesRequired() int {
	if c.Level <= 4 {
		return 0
	}
	if c.Level <= 6 {
		return 1
	}
	return 2
}

// --- Stat Modifiers ---

// StatModifier represents an ATK/DEF modification from an effect.
type StatModifier struct {
	Source     int // card ID of the source
	ATKMod     int
	DEFMod     int
	Permanent  bool // survives source leaving the field
	Continuous bool // recalculated by continuous effects system
}

// --- CardInstance (runtime card on field/hand/scrapheap) ---

type CardInstance struct {
	Card       *Card
	ID         int // unique instance ID within a duel
	Owner      int // player index (0 or 1) who owns this card
	Controller int // player index currently controlling this card

	// Field state
	Face      FaceStatus
	Position  Position
	Zone      ZoneType
	ZoneIndex int // index within the zone (0-4 for agent/ST)

	// Per-turn tracking
	TurnPlaced              int
	TurnControlChanged      int
	AttackedThisTurn        bool
	PositionChangedThisTurn bool
	Counters                map[string]int

	// Stat modifiers
	Modifiers   []StatModifier
	OriginalATK int // for effects that "set ATK to X" (0 = use Card.ATK)
	OriginalDEF int // for effects that "set DEF to X" (0 = use Card.DEF)

	// Equip tracking
	EquippedTo *CardInstance   // if this is an equip card, what it's attached to
	Equips     []*CardInstance // equip cards attached to this agent
}

func (ci *CardInstance) String() string {
	if ci == nil {
		return "(empty)"
	}
	if ci.Face == FaceDown {
		return fmt.Sprintf("face-down %s", ci.Position)
	}
	return fmt.Sprintf("%s (%s %d, %s %s)", ci.Card.Name, ci.Position, ci.CurrentATK(), ci.Face, ci.Position)
}

// DisplayString returns a human-readable description for the event log.
func (ci *CardInstance) DisplayString() string {
	if ci == nil {
		return "(empty)"
	}
	if ci.Card.CardType == CardTypeAgent {
		if ci.Face == FaceDown {
			return fmt.Sprintf("%s (%s %s)", ci.Card.Name, ci.Face, ci.Position)
		}
		return fmt.Sprintf("%s (ATK %d/DEF %d)", ci.Card.Name, ci.CurrentATK(), ci.CurrentDEF())
	}
	return ci.Card.Name
}

// CurrentATK returns the effective ATK (base + all modifiers).
func (ci *CardInstance) CurrentATK() int {
	base := ci.Card.ATK
	if ci.OriginalATK != 0 {
		base = ci.OriginalATK
	}
	for _, mod := range ci.Modifiers {
		base += mod.ATKMod
	}
	if base < 0 {
		base = 0
	}
	return base
}

// CurrentDEF returns the effective DEF (base + all modifiers).
func (ci *CardInstance) CurrentDEF() int {
	base := ci.Card.DEF
	if ci.OriginalDEF != 0 {
		base = ci.OriginalDEF
	}
	for _, mod := range ci.Modifiers {
		base += mod.DEFMod
	}
	if base < 0 {
		base = 0
	}
	return base
}

// AddModifier adds a stat modifier to this card.
func (ci *CardInstance) AddModifier(mod StatModifier) {
	ci.Modifiers = append(ci.Modifiers, mod)
}

// RemoveModifiersBySource removes all modifiers from the given source card.
func (ci *CardInstance) RemoveModifiersBySource(sourceID int) {
	filtered := ci.Modifiers[:0]
	for _, mod := range ci.Modifiers {
		if mod.Source != sourceID {
			filtered = append(filtered, mod)
		}
	}
	ci.Modifiers = filtered
}

// --- Zone types ---

type ZoneType int

const (
	ZoneDeck ZoneType = iota
	ZoneHand
	ZoneAgent
	ZoneTech
	ZoneOS
	ZoneScrapheap
	ZonePurged
)

func (z ZoneType) String() string {
	switch z {
	case ZoneDeck:
		return "Deck"
	case ZoneHand:
		return "Hand"
	case ZoneAgent:
		return "Agent Zone"
	case ZoneTech:
		return "Tech Zone"
	case ZoneOS:
		return "OS Zone"
	case ZoneScrapheap:
		return "Scrapheap"
	case ZonePurged:
		return "Purged"
	default:
		return "Unknown"
	}
}

// --- Action types ---

type ActionType int

const (
	ActionNormalSummon ActionType = iota
	ActionNormalSet
	ActionSacrificeSummon
	ActionSacrificeSet
	ActionFlipSummon
	ActionChangePosition
	ActionAttack
	ActionDirectAttack
	ActionSetTech
	ActionActivate
	ActionEnterBattlePhase
	ActionEnterMainPhase2
	ActionEndTurn
	ActionEndBattlePhase
	ActionPass // explicitly pass priority
)

func (a ActionType) String() string {
	switch a {
	case ActionNormalSummon:
		return "Normal Summon"
	case ActionNormalSet:
		return "Normal Set"
	case ActionSacrificeSummon:
		return "Sacrifice Summon"
	case ActionSacrificeSet:
		return "Sacrifice Set"
	case ActionFlipSummon:
		return "Flip Summon"
	case ActionChangePosition:
		return "Change Position"
	case ActionAttack:
		return "Attack"
	case ActionDirectAttack:
		return "Direct Attack"
	case ActionSetTech:
		return "Set Tech"
	case ActionActivate:
		return "Activate"
	case ActionEnterBattlePhase:
		return "Enter Battle Phase"
	case ActionEnterMainPhase2:
		return "Enter Main Phase 2"
	case ActionEndTurn:
		return "End Turn"
	case ActionEndBattlePhase:
		return "End Battle Phase"
	case ActionPass:
		return "Pass"
	default:
		return "Unknown"
	}
}

// Action represents a player action with all necessary details.
type Action struct {
	Type        ActionType
	Player      int
	Card        *CardInstance   // card being played/used
	Zone        int             // target zone index
	Targets     []*CardInstance // sacrifice targets, attack target, etc.
	EffectIndex int             // which effect on the card is being activated
	Desc        string          // human-readable description
}

func (a Action) String() string {
	if a.Desc != "" {
		return a.Desc
	}
	return a.Type.String()
}
