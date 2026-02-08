package game

import (
	"context"
	"fmt"
	"testing"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// ScriptedController is a PlayerController that follows a predefined script of actions.
// Used in tests to deterministically drive the game.
type ScriptedController struct {
	t       *testing.T
	name    string
	actions []ScriptedAction
	pos     int

	// For ChooseCards prompts
	cardChoices []ScriptedCardChoice
	cardPos     int

	// For ChooseYesNo prompts
	yesNoChoices []bool
	yesNoPos     int
}

type ScriptedAction struct {
	// Match by ActionType — picks the first action of this type
	Type ActionType
	// Optional: match by card name as well
	CardName string
	// Optional: match by target card name
	TargetName string
}

type ScriptedCardChoice struct {
	// Choose cards by name
	Names []string
}

func NewScriptedController(t *testing.T, name string) *ScriptedController {
	return &ScriptedController{t: t, name: name}
}

func (sc *ScriptedController) AddAction(actionType ActionType, cardName string) *ScriptedController {
	sc.actions = append(sc.actions, ScriptedAction{Type: actionType, CardName: cardName})
	return sc
}

func (sc *ScriptedController) AddAttack(attackerName, targetName string) *ScriptedController {
	sc.actions = append(sc.actions, ScriptedAction{Type: ActionAttack, CardName: attackerName, TargetName: targetName})
	return sc
}

func (sc *ScriptedController) AddDirectAttack(attackerName string) *ScriptedController {
	sc.actions = append(sc.actions, ScriptedAction{Type: ActionDirectAttack, CardName: attackerName})
	return sc
}

func (sc *ScriptedController) AddCardChoice(names ...string) *ScriptedController {
	sc.cardChoices = append(sc.cardChoices, ScriptedCardChoice{Names: names})
	return sc
}

func (sc *ScriptedController) AddYesNo(answer bool) *ScriptedController {
	sc.yesNoChoices = append(sc.yesNoChoices, answer)
	return sc
}

func (sc *ScriptedController) AddPass() *ScriptedController {
	sc.actions = append(sc.actions, ScriptedAction{Type: ActionPass})
	return sc
}

func (sc *ScriptedController) ChooseAction(ctx context.Context, state *GameState, actions []Action) (Action, error) {
	if sc.pos >= len(sc.actions) {
		// Default priority: Pass > EndTurn > EnterMainPhase2 > last action
		// Pass is preferred because response windows offer it and we want to auto-pass
		for _, a := range actions {
			if a.Type == ActionPass {
				return a, nil
			}
		}
		for _, a := range actions {
			if a.Type == ActionEndTurn {
				return a, nil
			}
		}
		for _, a := range actions {
			if a.Type == ActionEnterMainPhase2 {
				return a, nil
			}
		}
		return actions[len(actions)-1], nil
	}

	// Peek at next scripted action — only consume it if it matches an available action.
	// This allows scripts to span multiple turns without needing to explicitly script "EndTurn".
	scripted := sc.actions[sc.pos]

	for _, a := range actions {
		if a.Type != scripted.Type {
			continue
		}
		if scripted.CardName != "" && (a.Card == nil || a.Card.Card.Name != scripted.CardName) {
			continue
		}
		if scripted.TargetName != "" {
			if len(a.Targets) == 0 || a.Targets[0].Card.Name != scripted.TargetName {
				continue
			}
		}
		// Found match — consume and return
		sc.pos++
		return a, nil
	}

	// Scripted action not yet available (probably a future turn) — default: Pass > EndTurn > MP2
	for _, a := range actions {
		if a.Type == ActionPass {
			return a, nil
		}
	}
	for _, a := range actions {
		if a.Type == ActionEndTurn || a.Type == ActionEnterMainPhase2 {
			return a, nil
		}
	}
	return actions[len(actions)-1], nil
}

func (sc *ScriptedController) ChooseCards(ctx context.Context, state *GameState, prompt string, candidates []*CardInstance, min, max int) ([]*CardInstance, error) {
	if sc.cardPos >= len(sc.cardChoices) {
		// Default: choose the first min candidates
		if min > len(candidates) {
			min = len(candidates)
		}
		return candidates[:min], nil
	}

	choice := sc.cardChoices[sc.cardPos]
	sc.cardPos++

	var result []*CardInstance
	for _, name := range choice.Names {
		for _, c := range candidates {
			if c.Card.Name == name {
				result = append(result, c)
				break
			}
		}
	}

	if len(result) < min {
		return nil, fmt.Errorf("[%s] card choice: wanted %v but only found %d in candidates", sc.name, choice.Names, len(result))
	}
	return result, nil
}

func (sc *ScriptedController) ChooseYesNo(ctx context.Context, state *GameState, prompt string) (bool, error) {
	if sc.yesNoPos >= len(sc.yesNoChoices) {
		return false, nil
	}
	answer := sc.yesNoChoices[sc.yesNoPos]
	sc.yesNoPos++
	return answer, nil
}

func (sc *ScriptedController) Notify(ctx context.Context, event log.GameEvent) error {
	return nil
}

// --- Test card helpers ---

func vanillaAgent(name string, level int, atk, def int, attr Attribute) *Card {
	return &Card{
		Name:      name,
		CardType:  CardTypeAgent,
		Level:     level,
		Attribute: attr,
		ATK:       atk,
		DEF:       def,
	}
}

func normalProgram(name string, effects ...*CardEffect) *Card {
	return &Card{
		Name:       name,
		CardType:   CardTypeProgram,
		ProgramSub: ProgramNormal,
		Effects:    effects,
	}
}

func quickPlayProgram(name string, effects ...*CardEffect) *Card {
	return &Card{
		Name:       name,
		CardType:   CardTypeProgram,
		ProgramSub: ProgramQuickPlay,
		Effects:    effects,
	}
}

func normalTrap(name string, effects ...*CardEffect) *Card {
	return &Card{
		Name:     name,
		CardType: CardTypeTrap,
		TrapSub:  TrapNormal,
		Effects:  effects,
	}
}

func counterTrap(name string, effects ...*CardEffect) *Card {
	return &Card{
		Name:     name,
		CardType: CardTypeTrap,
		TrapSub:  TrapCounter,
		Effects:  effects,
	}
}

func makeDeck(cards ...*Card) []*Card {
	deck := make([]*Card, 0, len(cards))
	deck = append(deck, cards...)
	return deck
}

// makePaddedDeck creates a deck with specified cards on top (drawn first) and filler to reach a minimum size.
// topCards are ordered so that index 0 is drawn first.
func makePaddedDeck(topCards []*Card, minSize int) []*Card {
	filler := vanillaAgent("Filler Token", 1, 0, 0, AttrLIGHT)
	deck := make([]*Card, 0, minSize)

	// Filler goes at bottom (drawn last)
	for i := 0; i < minSize-len(topCards); i++ {
		deck = append(deck, filler)
	}

	// Top cards go at end of slice (drawn first) — reverse order so index 0 is drawn first
	for i := len(topCards) - 1; i >= 0; i-- {
		deck = append(deck, topCards[i])
	}

	return deck
}

// runDuelToCompletion runs a duel and returns the logger for inspection.
func runDuelToCompletion(t *testing.T, cfg DuelConfig, p0, p1 *ScriptedController) *log.MemoryLogger {
	t.Helper()
	logger := log.NewMemoryLogger()
	cfg.Logger = logger
	cfg.NoShuffle = true // deterministic tests
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = 100 // reasonable default for tests
	}

	duel := NewDuel(cfg, p0, p1)

	winner, err := duel.Run(context.Background())
	if err != nil {
		t.Logf("Event log:\n%s", log.FormatAll(logger.Events()))
		t.Fatalf("Duel error: %v", err)
	}

	// Always print event log for visibility (tests are run with -v)
	t.Logf("Duel result: winner=%d (%s)", winner, duel.State.Result)
	t.Logf("Event log:\n%s", log.FormatAll(logger.Events()))

	return logger
}
