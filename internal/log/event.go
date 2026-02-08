package log

// EventType enumerates all observable game events.
type EventType int

const (
	EventPhaseChange EventType = iota
	EventDraw
	EventNormalSummon
	EventSacrificeSummon
	EventFlipSummon
	EventSpecialSummon
	EventSetAgent
	EventSetTech
	EventChangePosition
	EventAttackDeclare
	EventDirectAttackDeclare
	EventDamageCalc
	EventBattleDestroy
	EventDirectAttack
	EventReplay
	EventActivate
	EventChainLink
	EventChainResolve
	EventDestroy
	EventSendToScrapheap
	EventPurge
	EventAddToHand
	EventDiscard
	EventHPChange
	EventFlipFaceDown
	EventChangeControl
	EventTriggerQueued
	EventWin
	EventDraw_Tie
	EventShuffle
	EventNewTurn
	EventHandSizeDiscard
	EventFlipNoSummon  // flipped face-up by attack, not a flip summon
	EventAttackStopped // attack cannot proceed due to restriction (e.g. Gravity Clamp)
)

func (e EventType) String() string {
	switch e {
	case EventPhaseChange:
		return "PhaseChange"
	case EventDraw:
		return "Draw"
	case EventNormalSummon:
		return "NormalSummon"
	case EventSacrificeSummon:
		return "SacrificeSummon"
	case EventFlipSummon:
		return "FlipSummon"
	case EventSpecialSummon:
		return "SpecialSummon"
	case EventSetAgent:
		return "SetAgent"
	case EventSetTech:
		return "SetTech"
	case EventChangePosition:
		return "ChangePosition"
	case EventAttackDeclare:
		return "AttackDeclare"
	case EventDirectAttackDeclare:
		return "DirectAttackDeclare"
	case EventDamageCalc:
		return "DamageCalc"
	case EventBattleDestroy:
		return "BattleDestroy"
	case EventDirectAttack:
		return "DirectAttack"
	case EventReplay:
		return "Replay"
	case EventActivate:
		return "Activate"
	case EventChainLink:
		return "ChainLink"
	case EventChainResolve:
		return "ChainResolve"
	case EventDestroy:
		return "Destroy"
	case EventSendToScrapheap:
		return "SendToScrapheap"
	case EventPurge:
		return "Purge"
	case EventAddToHand:
		return "AddToHand"
	case EventDiscard:
		return "Discard"
	case EventHPChange:
		return "HPChange"
	case EventFlipFaceDown:
		return "FlipFaceDown"
	case EventChangeControl:
		return "ChangeControl"
	case EventTriggerQueued:
		return "TriggerQueued"
	case EventWin:
		return "Win"
	case EventDraw_Tie:
		return "Draw(tie)"
	case EventShuffle:
		return "Shuffle"
	case EventNewTurn:
		return "NewTurn"
	case EventHandSizeDiscard:
		return "HandSizeDiscard"
	case EventFlipNoSummon:
		return "FlipNoSummon"
	case EventAttackStopped:
		return "AttackStopped"
	default:
		return "Unknown"
	}
}

// GameEvent represents a single observable event in a duel.
type GameEvent struct {
	Seq     int       // monotonic sequence number
	Turn    int       // which turn (1-based)
	Phase   string    // current phase name (e.g. "Main Phase 1")
	Player  int       // acting player (0 or 1)
	Type    EventType // event type
	Card    string    // card name (if applicable)
	Details string    // human-readable detail string
}
