package log

import (
	"fmt"
	"io"
	"strings"
)

// EventLogger is the interface for logging game events.
type EventLogger interface {
	Log(event GameEvent)
	Events() []GameEvent
}

// --- MemoryLogger: stores events in memory for test assertions ---

type MemoryLogger struct {
	events []GameEvent
	seq    int
}

func NewMemoryLogger() *MemoryLogger {
	return &MemoryLogger{}
}

func (l *MemoryLogger) Log(event GameEvent) {
	l.seq++
	event.Seq = l.seq
	l.events = append(l.events, event)
}

func (l *MemoryLogger) Events() []GameEvent {
	return l.events
}

// EventsOfType returns all events matching the given type.
func (l *MemoryLogger) EventsOfType(t EventType) []GameEvent {
	var result []GameEvent
	for _, e := range l.events {
		if e.Type == t {
			result = append(result, e)
		}
	}
	return result
}

// LastEvent returns the most recent event, or a zero event if none.
func (l *MemoryLogger) LastEvent() GameEvent {
	if len(l.events) == 0 {
		return GameEvent{}
	}
	return l.events[len(l.events)-1]
}

// --- TextLogger: writes human-readable lines to an io.Writer ---

type TextLogger struct {
	MemoryLogger
	w io.Writer
}

func NewTextLogger(w io.Writer) *TextLogger {
	return &TextLogger{w: w}
}

func (l *TextLogger) Log(event GameEvent) {
	l.MemoryLogger.Log(event)
	fmt.Fprintln(l.w, FormatEvent(event))
}

// --- Formatting ---

// playerName returns "P1" or "P2" for display.
func playerName(p int) string {
	return fmt.Sprintf("P%d", p+1)
}

// FormatEvent formats a single event as a human-readable line.
func FormatEvent(e GameEvent) string {
	phase := e.Phase
	if phase == "" {
		phase = "          "
	}
	// Pad phase to 16 chars for alignment
	for len(phase) < 16 {
		phase += " "
	}

	return fmt.Sprintf("T%-2d %s| %s", e.Turn, phase, e.Details)
}

// FormatAll formats all events as a multi-line string.
func FormatAll(events []GameEvent) string {
	var sb strings.Builder
	for _, e := range events {
		sb.WriteString(FormatEvent(e))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// --- Helper constructors for common events ---

func NewPhaseChangeEvent(turn int, phase string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Type:    EventPhaseChange,
		Details: fmt.Sprintf("Phase → %s", phase),
	}
}

func NewTurnEvent(turn int, player int) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   "Draw Phase",
		Player:  player,
		Type:    EventNewTurn,
		Details: fmt.Sprintf("=== Turn %d (%s) ===", turn, playerName(player)),
	}
}

func NewDrawEvent(turn int, phase string, player int, cardName string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventDraw,
		Card:    cardName,
		Details: fmt.Sprintf("%s draws %s", playerName(player), cardName),
	}
}

func NewNormalSummonEvent(turn int, phase string, player int, cardName string, atk int, zone int) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventNormalSummon,
		Card:    cardName,
		Details: fmt.Sprintf("%s normal summons %s (ATK %d) to Agent Zone %d", playerName(player), cardName, atk, zone+1),
	}
}

func NewSacrificeSummonEvent(turn int, phase string, player int, cardName string, atk int, zone int, sacrifices []string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventSacrificeSummon,
		Card:    cardName,
		Details: fmt.Sprintf("%s sacrifice summons %s (ATK %d) to Agent Zone %d (sacrificed: %s)", playerName(player), cardName, atk, zone+1, strings.Join(sacrifices, ", ")),
	}
}

func NewSetAgentEvent(turn int, phase string, player int, zone int) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventSetAgent,
		Details: fmt.Sprintf("%s sets an agent in Agent Zone %d", playerName(player), zone+1),
	}
}

func NewFlipSummonEvent(turn int, phase string, player int, cardName string, atk int, zone int) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventFlipSummon,
		Card:    cardName,
		Details: fmt.Sprintf("%s flip summons %s (ATK %d) in Agent Zone %d", playerName(player), cardName, atk, zone+1),
	}
}

func NewChangePositionEvent(turn int, phase string, player int, cardName string, newPos string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventChangePosition,
		Card:    cardName,
		Details: fmt.Sprintf("%s changes %s to %s position", playerName(player), cardName, newPos),
	}
}

func NewAttackDeclareEvent(turn int, player int, attacker string, defender string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   "Battle Phase",
		Player:  player,
		Type:    EventAttackDeclare,
		Card:    attacker,
		Details: fmt.Sprintf("%s declares attack: %s → %s", playerName(player), attacker, defender),
	}
}

func NewDirectAttackDeclareEvent(turn int, player int, attacker string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   "Battle Phase",
		Player:  player,
		Type:    EventDirectAttackDeclare,
		Card:    attacker,
		Details: fmt.Sprintf("%s declares direct attack with %s", playerName(player), attacker),
	}
}

func NewDamageCalcEvent(turn int, player int, details string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   "Battle Phase",
		Player:  player,
		Type:    EventDamageCalc,
		Details: details,
	}
}

func NewBattleDestroyEvent(turn int, player int, cardName string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   "Battle Phase",
		Player:  player,
		Type:    EventBattleDestroy,
		Card:    cardName,
		Details: fmt.Sprintf("%s is destroyed by battle", cardName),
	}
}

func NewHPChangeEvent(turn int, phase string, player int, oldHP, newHP int, reason string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventHPChange,
		Details: fmt.Sprintf("%s HP: %d → %d (%s)", playerName(player), oldHP, newHP, reason),
	}
}

func NewWinEvent(turn int, phase string, winner int, reason string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  winner,
		Type:    EventWin,
		Details: fmt.Sprintf("%s wins! (%s)", playerName(winner), reason),
	}
}

func NewSendToScrapheapEvent(turn int, phase string, player int, cardName string, reason string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventSendToScrapheap,
		Card:    cardName,
		Details: fmt.Sprintf("%s is sent to %s's Scrapheap (%s)", cardName, playerName(player), reason),
	}
}

func NewDiscardEvent(turn int, phase string, player int, cardName string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventDiscard,
		Card:    cardName,
		Details: fmt.Sprintf("%s discards %s", playerName(player), cardName),
	}
}

func NewActivateEvent(turn int, phase string, player int, cardName string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventActivate,
		Card:    cardName,
		Details: fmt.Sprintf("%s activates %s", playerName(player), cardName),
	}
}

func NewChainLinkEvent(turn int, phase string, player int, cardName string, chainIndex int) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventChainLink,
		Card:    cardName,
		Details: fmt.Sprintf("Chain Link %d: %s activates %s", chainIndex, playerName(player), cardName),
	}
}

func NewChainResolveEvent(turn int, phase string, player int, cardName string, chainIndex int) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventChainResolve,
		Card:    cardName,
		Details: fmt.Sprintf("Chain Link %d resolves: %s", chainIndex, cardName),
	}
}

func NewSetTechEvent(turn int, phase string, player int, zone int) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventSetTech,
		Details: fmt.Sprintf("%s sets a card in Tech Zone %d", playerName(player), zone+1),
	}
}

func NewDestroyEvent(turn int, phase string, player int, cardName string, reason string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventDestroy,
		Card:    cardName,
		Details: fmt.Sprintf("%s is destroyed (%s)", cardName, reason),
	}
}

func NewFlipFaceDownEvent(turn int, phase string, player int, cardName string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventFlipFaceDown,
		Card:    cardName,
		Details: fmt.Sprintf("%s is flipped face-down", cardName),
	}
}

func NewFlipEvent(turn int, phase string, player int, cardName string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventFlipNoSummon,
		Card:    cardName,
		Details: fmt.Sprintf("%s is flipped face-up", cardName),
	}
}

func NewSpecialSummonEvent(turn int, phase string, player int, cardName string, atk int, zone int) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventSpecialSummon,
		Card:    cardName,
		Details: fmt.Sprintf("%s special summons %s (ATK %d) to Agent Zone %d", playerName(player), cardName, atk, zone+1),
	}
}

func NewPurgeEvent(turn int, phase string, player int, cardName string, reason string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventPurge,
		Card:    cardName,
		Details: fmt.Sprintf("%s is purged (%s)", cardName, reason),
	}
}

func NewAddToHandEvent(turn int, phase string, player int, cardName string, reason string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventAddToHand,
		Card:    cardName,
		Details: fmt.Sprintf("%s is added to %s's hand (%s)", cardName, playerName(player), reason),
	}
}

func NewChangeControlEvent(turn int, phase string, player int, cardName string, newController int) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventChangeControl,
		Card:    cardName,
		Details: fmt.Sprintf("%s control changes to %s", cardName, playerName(newController)),
	}
}

func NewReplayEvent(turn int, player int, attackerName string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   "Battle Phase",
		Player:  player,
		Type:    EventReplay,
		Card:    attackerName,
		Details: fmt.Sprintf("Battle replay: %s may choose a new target", attackerName),
	}
}

func NewAttackStoppedEvent(turn int, player int, attackerName string, reason string) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   "Battle Phase",
		Player:  player,
		Type:    EventAttackStopped,
		Card:    attackerName,
		Details: fmt.Sprintf("%s cannot continue attack (%s)", attackerName, reason),
	}
}

func NewShuffleEvent(turn int, phase string, player int) GameEvent {
	return GameEvent{
		Turn:    turn,
		Phase:   phase,
		Player:  player,
		Type:    EventShuffle,
		Details: fmt.Sprintf("P%d shuffled their deck", player+1),
	}
}
