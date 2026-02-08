package game

import "github.com/peterkuimelis/tcgx/internal/log"

// EffectType categorizes agent effects.
type EffectType int

const (
	EffectNone       EffectType = iota
	EffectFlip                  // FLIP: activates when flipped face-up
	EffectIgnition              // requires manual activation in main phase
	EffectTrigger               // activates in response to a game event
	EffectContinuous            // passive/applied while face-up on field
	EffectQuick                 // can chain during opponent's turn (SS2)
)

// CardEffect represents a single activatable effect on a card.
type CardEffect struct {
	Name       string
	ExecSpeed  ExecSpeed
	EffectType EffectType

	// CanActivate checks whether this effect can currently be activated.
	CanActivate func(d *Duel, card *CardInstance, player int) bool

	// Cost pays any costs (e.g. HP, discard). Returns false if cancelled.
	Cost func(d *Duel, card *CardInstance, player int) (bool, error)

	// Target selects and locks targets at activation time.
	Target func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error)

	// Resolve applies the effect when the chain link resolves.
	Resolve func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error

	// Trigger effect fields
	IsTrigger    bool
	IsMandatory  bool
	TriggerEvent log.EventType

	// TriggerFilter checks if a specific event matches this trigger.
	TriggerFilter func(d *Duel, card *CardInstance, event log.GameEvent) bool

	// OnFieldEffect is called when a continuous card is face-up on the field.
	// Used for passive/ongoing effects (e.g. continuous programs/traps).
	OnFieldEffect func(d *Duel, card *CardInstance, player int)

	// OnLeaveField is called when this card leaves the field. Used for cleanup.
	OnLeaveField func(d *Duel, card *CardInstance, player int)

	// SpecialSummonCondition checks if a agent can be special summoned from hand/scrapheap.
	SpecialSummonCondition func(d *Duel, card *CardInstance, player int) bool

	// ContinuousApply is called by recalculateContinuousEffects to apply field-wide
	// stat/rule modifiers. These are stripped and reapplied whenever the board changes.
	ContinuousApply func(d *Duel, card *CardInstance, player int)

	// HasPiercing indicates this effect grants piercing battle damage.
	HasPiercing bool

	// CanDirectAttack checks if this agent can attack directly even when opponent has agents.
	CanDirectAttack func(d *Duel, card *CardInstance, player int) bool

	// AttackRestriction returns false if the given attacker is not allowed to attack
	// while this card's effect is active.
	AttackRestriction func(d *Duel, attacker *CardInstance) bool

	// TargetRestriction returns false if this agent cannot be targeted for an attack.
	TargetRestriction func(d *Duel, card *CardInstance, player int) bool

	// OnBattleDamage is called when this agent deals battle damage.
	OnBattleDamage func(d *Duel, card *CardInstance, player int)

	// OnDestroyByBattle is called when this agent destroys another agent by battle.
	OnDestroyByBattle func(d *Duel, card *CardInstance, player int)

	// OnBattleDestruction is called when this agent is destroyed by battle (from scrapheap).
	OnBattleDestruction func(d *Duel, card *CardInstance, player int)
}

// EffectExecSpeed derives the execution speed from a card's type and subtype.
func EffectExecSpeed(card *Card) ExecSpeed {
	switch card.CardType {
	case CardTypeProgram:
		if card.ProgramSub == ProgramQuickPlay {
			return ExecSpeed2
		}
		return ExecSpeed1
	case CardTypeTrap:
		if card.TrapSub == TrapCounter {
			return ExecSpeed3
		}
		return ExecSpeed2
	default:
		return ExecSpeed1
	}
}
