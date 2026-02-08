package game

import (
	"context"
	"fmt"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// PlayerController is the interface that both human (WebSocket) and AI (MCP) players implement.
type PlayerController interface {
	// ChooseAction presents available actions and waits for the player to pick one.
	ChooseAction(ctx context.Context, state *GameState, actions []Action) (Action, error)

	// ChooseCards asks the player to select cards from a list (e.g., sacrifice targets).
	ChooseCards(ctx context.Context, state *GameState, prompt string, candidates []*CardInstance, min, max int) ([]*CardInstance, error)

	// ChooseYesNo asks the player a yes/no question (e.g., "activate optional effect?").
	ChooseYesNo(ctx context.Context, state *GameState, prompt string) (bool, error)

	// Notify sends a game event notification (no response needed).
	Notify(ctx context.Context, event log.GameEvent) error
}

// DuelConfig holds configuration for creating a new duel.
type DuelConfig struct {
	Deck0     []*Card // Player 0's deck (card definitions)
	Deck1     []*Card // Player 1's deck (card definitions)
	Logger    log.EventLogger
	Seed      int64 // RNG seed (0 for random)
	NoShuffle bool  // skip deck shuffle (for deterministic tests)
	MaxTurns  int   // stop after this many turns (0 = no limit)
}

// Duel orchestrates an entire duel between two players.
type Duel struct {
	State       *GameState
	Controllers [2]PlayerController
	Logger      log.EventLogger
	ctx         context.Context
	noShuffle   bool
	maxTurns    int
}

// NewDuel creates a new duel from the given config and player controllers.
func NewDuel(cfg DuelConfig, p0, p1 PlayerController) *Duel {
	gs := NewGameState()
	logger := cfg.Logger
	if logger == nil {
		logger = log.NewMemoryLogger()
	}

	// Build player decks as CardInstances
	for _, card := range cfg.Deck0 {
		ci := gs.CreateCardInstance(card, 0)
		ci.Zone = ZoneDeck
		gs.Players[0].Deck = append(gs.Players[0].Deck, ci)
	}
	for _, card := range cfg.Deck1 {
		ci := gs.CreateCardInstance(card, 1)
		ci.Zone = ZoneDeck
		gs.Players[1].Deck = append(gs.Players[1].Deck, ci)
	}

	maxTurns := cfg.MaxTurns
	if maxTurns == 0 {
		maxTurns = 200 // safety limit
	}

	return &Duel{
		State:       gs,
		Controllers: [2]PlayerController{p0, p1},
		Logger:      logger,
		ctx:         context.Background(),
		noShuffle:   cfg.NoShuffle,
		maxTurns:    maxTurns,
	}
}

// Run executes the entire duel loop. Returns the winner (0, 1, or -1 for draw).
func (d *Duel) Run(ctx context.Context) (int, error) {
	d.ctx = ctx
	gs := d.State

	// Setup: shuffle decks (unless disabled for tests)
	if !d.noShuffle {
		gs.Players[0].ShuffleDeck()
		gs.Players[1].ShuffleDeck()
	}

	// Draw initial hands (5 cards each)
	for i := 0; i < InitialHandSize; i++ {
		for p := 0; p < 2; p++ {
			card := gs.Players[p].DrawCard()
			if card == nil {
				return -1, fmt.Errorf("player %d has insufficient cards for initial hand", p)
			}
		}
	}

	// Main duel loop
	for !gs.Over {
		if gs.Turn >= d.maxTurns {
			gs.Over = true
			gs.Winner = -1
			gs.Result = fmt.Sprintf("Turn limit reached (%d turns)", d.maxTurns)
			break
		}
		if err := d.runTurn(); err != nil {
			return gs.Winner, err
		}
		if err := d.ctx.Err(); err != nil {
			return -1, err
		}
	}

	return gs.Winner, nil
}

// runTurn executes a single turn for the current turn player.
func (d *Duel) runTurn() error {
	gs := d.State
	gs.Turn++
	gs.ResetTurnFlags()

	d.log(log.NewTurnEvent(gs.Turn, gs.TurnPlayer))

	// Draw Phase
	if err := d.drawPhase(); err != nil {
		return err
	}
	if gs.Over {
		return nil
	}

	// Standby Phase
	if err := d.standbyPhase(); err != nil {
		return err
	}
	if gs.Over {
		return nil
	}

	// Main Phase 1
	if err := d.mainPhase(PhaseMain1); err != nil {
		return err
	}
	if gs.Over {
		return nil
	}

	// Battle Phase (not on turn 1 for the first player)
	enteredBattle := false
	if gs.Phase == PhaseBattle {
		enteredBattle = true
		if err := d.battlePhase(); err != nil {
			return err
		}
		if gs.Over {
			return nil
		}
	}

	// Main Phase 2 (only if entered Battle Phase)
	if enteredBattle && !gs.Over {
		if err := d.mainPhase(PhaseMain2); err != nil {
			return err
		}
		if gs.Over {
			return nil
		}
	}

	// End Phase
	if err := d.endPhase(); err != nil {
		return err
	}

	// Switch turn player
	gs.TurnPlayer = gs.Opponent(gs.TurnPlayer)

	return nil
}

// drawPhase executes the Draw Phase.
func (d *Duel) drawPhase() error {
	gs := d.State
	gs.Phase = PhaseDraw
	d.log(log.NewPhaseChangeEvent(gs.Turn, gs.Phase.String()))

	// Goat rule: first player DOES draw on turn 1
	p := gs.CurrentPlayer()
	card := p.DrawCard()
	if card == nil {
		// Deck out — current player loses
		gs.Over = true
		gs.Winner = gs.Opponent(gs.TurnPlayer)
		gs.Result = fmt.Sprintf("P%d wins — P%d decked out", gs.Winner+1, gs.TurnPlayer+1)
		d.log(log.NewWinEvent(gs.Turn, gs.Phase.String(), gs.Winner, "deck out"))
		return nil
	}
	d.log(log.NewDrawEvent(gs.Turn, gs.Phase.String(), gs.TurnPlayer, card.Card.Name))

	return nil
}

// standbyPhase executes the Standby Phase.
func (d *Duel) standbyPhase() error {
	gs := d.State
	gs.Phase = PhaseStandby
	d.log(log.NewPhaseChangeEvent(gs.Turn, gs.Phase.String()))

	// Process standby phase triggers (e.g. Sinister Serpent, Snatch Steal HP gain)
	d.processStandbyTriggers()

	return nil
}

// processStandbyTriggers checks for effects that trigger during the Standby Phase.
func (d *Duel) processStandbyTriggers() {
	gs := d.State
	tp := gs.TurnPlayer

	// Check all face-up tech and agents for standby triggers
	for p := 0; p < 2; p++ {
		for _, card := range gs.Players[p].TechCards() {
			if card.Face != FaceUp {
				continue
			}
			for _, eff := range card.Card.Effects {
				if eff.OnFieldEffect != nil {
					eff.OnFieldEffect(d, card, p)
				}
			}
		}
		for _, card := range gs.Players[p].FaceUpAgents() {
			for _, eff := range card.Card.Effects {
				if eff.OnFieldEffect != nil {
					eff.OnFieldEffect(d, card, p)
				}
			}
		}
	}

	// Check scrapheap for standby phase recovery effects (e.g. Sinister Serpent)
	for _, card := range gs.Players[tp].Scrapheap {
		for _, eff := range card.Card.Effects {
			if eff.EffectType == EffectTrigger && eff.TriggerEvent == log.EventPhaseChange {
				if eff.CanActivate != nil && eff.CanActivate(d, card, tp) {
					if eff.IsMandatory {
						if eff.Resolve != nil {
							_ = eff.Resolve(d, card, tp, nil)
						}
					} else {
						yes, _ := d.Controllers[tp].ChooseYesNo(d.ctx, gs, "Activate "+card.Card.Name+" effect?")
						if yes && eff.Resolve != nil {
							_ = eff.Resolve(d, card, tp, nil)
						}
					}
				}
			}
		}
	}
}

// mainPhase executes a Main Phase (1 or 2).
func (d *Duel) mainPhase(phase Phase) error {
	gs := d.State
	gs.Phase = phase
	d.log(log.NewPhaseChangeEvent(gs.Turn, gs.Phase.String()))

	tp := gs.TurnPlayer

	for !gs.Over {
		actions := d.computeMainPhaseActions(tp)
		if len(actions) == 0 {
			break
		}

		chosen, err := d.Controllers[tp].ChooseAction(d.ctx, gs, actions)
		if err != nil {
			return err
		}

		switch chosen.Type {
		case ActionNormalSummon:
			if err := d.executeNormalSummon(chosen); err != nil {
				return err
			}
		case ActionNormalSet:
			if err := d.executeNormalSet(chosen); err != nil {
				return err
			}
		case ActionSacrificeSummon:
			if err := d.executeSacrificeSummon(chosen); err != nil {
				return err
			}
		case ActionSacrificeSet:
			if err := d.executeSacrificeSet(chosen); err != nil {
				return err
			}
		case ActionFlipSummon:
			if err := d.executeFlipSummon(chosen); err != nil {
				return err
			}
		case ActionChangePosition:
			d.executeChangePosition(chosen)
		case ActionSetTech:
			if err := d.executeSetTech(chosen); err != nil {
				return err
			}
		case ActionActivate:
			if err := d.executeActivateEffect(chosen); err != nil {
				return err
			}
			// Open response window for opponent to chain
			if err := d.openResponseWindow(gs.Opponent(tp)); err != nil {
				return err
			}
			// Resolve the chain
			if err := d.resolveChain(); err != nil {
				return err
			}
		case ActionEnterBattlePhase:
			gs.Phase = PhaseBattle
			return nil
		case ActionEndTurn:
			return nil
		}
	}

	return nil
}

// battlePhase executes the Battle Phase.
func (d *Duel) battlePhase() error {
	gs := d.State
	gs.BattleStep = BattleStepStart
	d.log(log.NewPhaseChangeEvent(gs.Turn, gs.Phase.String()))

	// Start Step — just advance for now (fast effects added in Phase 2)

	// Battle Step loop: attacks
	gs.BattleStep = BattleStepBattle

	for !gs.Over {
		actions := d.computeBattlePhaseActions()
		if len(actions) == 0 {
			break
		}

		chosen, err := d.Controllers[gs.TurnPlayer].ChooseAction(d.ctx, gs, actions)
		if err != nil {
			return err
		}

		switch chosen.Type {
		case ActionAttack:
			if err := d.executeAttack(chosen); err != nil {
				return err
			}
		case ActionDirectAttack:
			if err := d.executeDirectAttack(chosen); err != nil {
				return err
			}
		case ActionEndBattlePhase:
			gs.BattleStep = BattleStepEnd
			return nil
		case ActionEnterMainPhase2:
			gs.BattleStep = BattleStepEnd
			gs.Phase = PhaseMain2
			return nil
		}
	}

	gs.BattleStep = BattleStepEnd
	return nil
}

// endPhase executes the End Phase.
func (d *Duel) endPhase() error {
	gs := d.State
	gs.Phase = PhaseEnd
	d.log(log.NewPhaseChangeEvent(gs.Turn, gs.Phase.String()))

	// Process end phase triggers (Solar Flare Serpent, Ghost Process, Gaia Core, etc.)
	d.processEndPhaseTriggers()
	if gs.Over {
		return nil
	}

	// Hand size check: discard down to 6
	p := gs.CurrentPlayer()
	for len(p.Hand) > MaxHandSize {
		// Ask player which card to discard
		toDiscard, err := d.Controllers[gs.TurnPlayer].ChooseCards(
			d.ctx, gs,
			fmt.Sprintf("Discard to %d cards (you have %d)", MaxHandSize, len(p.Hand)),
			p.Hand, 1, 1,
		)
		if err != nil {
			return err
		}
		if len(toDiscard) > 0 {
			card := toDiscard[0]
			p.RemoveFromHand(card)
			p.SendToScrapheap(card)
			d.log(log.NewDiscardEvent(gs.Turn, gs.Phase.String(), gs.TurnPlayer, card.Card.Name))
		}
	}

	return nil
}

// processEndPhaseTriggers processes effects that activate during the End Phase.
func (d *Duel) processEndPhaseTriggers() {
	gs := d.State
	tp := gs.TurnPlayer

	// Check face-up agents for end phase effects
	for p := 0; p < 2; p++ {
		for _, m := range gs.Players[p].FaceUpAgents() {
			for _, eff := range m.Card.Effects {
				if eff.OnFieldEffect != nil && eff.EffectType == EffectTrigger && eff.TriggerEvent == log.EventPhaseChange {
					if eff.CanActivate != nil && eff.CanActivate(d, m, p) {
						if eff.Resolve != nil {
							_ = eff.Resolve(d, m, p, nil)
						}
						if gs.Over {
							return
						}
					}
				}
			}
		}
	}

	// Check face-up tech for end phase effects
	for p := 0; p < 2; p++ {
		for _, st := range gs.Players[p].TechCards() {
			if st.Face != FaceUp {
				continue
			}
			for _, eff := range st.Card.Effects {
				if eff.OnFieldEffect != nil && eff.EffectType == EffectTrigger && eff.TriggerEvent == log.EventPhaseChange {
					if eff.CanActivate != nil && eff.CanActivate(d, st, p) {
						if eff.Resolve != nil {
							_ = eff.Resolve(d, st, p, nil)
						}
						if gs.Over {
							return
						}
					}
				}
			}
		}
	}

	// Check scrapheap for end phase recovery effects (Ghost Process)
	for _, card := range gs.Players[tp].Scrapheap {
		for _, eff := range card.Card.Effects {
			if eff.EffectType == EffectTrigger && eff.TriggerEvent == log.EventPhaseChange {
				if eff.CanActivate != nil && eff.CanActivate(d, card, tp) {
					if gs.Phase == PhaseEnd {
						if eff.IsMandatory {
							if eff.Resolve != nil {
								_ = eff.Resolve(d, card, tp, nil)
							}
						} else {
							yes, _ := d.Controllers[tp].ChooseYesNo(d.ctx, gs, "Activate "+card.Card.Name+" effect?")
							if yes && eff.Resolve != nil {
								_ = eff.Resolve(d, card, tp, nil)
							}
						}
					}
				}
			}
		}
	}
}

// recalculateContinuousEffects strips and reapplies all continuous stat modifiers.
func (d *Duel) recalculateContinuousEffects() {
	gs := d.State

	// Strip all continuous modifiers from all agents
	for p := 0; p < 2; p++ {
		for _, m := range gs.Players[p].FaceUpAgents() {
			var keep []StatModifier
			for _, mod := range m.Modifiers {
				if !mod.Continuous {
					keep = append(keep, mod)
				}
			}
			m.Modifiers = keep
		}
	}

	// Re-apply from all face-up continuous sources
	for p := 0; p < 2; p++ {
		// Check OS cards
		if fs := gs.Players[p].OS; fs != nil && fs.Face == FaceUp {
			for _, eff := range fs.Card.Effects {
				if eff.ContinuousApply != nil {
					eff.ContinuousApply(d, fs, p)
				}
			}
		}
		// Check face-up agents
		for _, m := range gs.Players[p].FaceUpAgents() {
			for _, eff := range m.Card.Effects {
				if eff.ContinuousApply != nil {
					eff.ContinuousApply(d, m, m.Controller)
				}
			}
		}
		// Check face-up tech
		for _, st := range gs.Players[p].TechCards() {
			if st.Face != FaceUp {
				continue
			}
			for _, eff := range st.Card.Effects {
				if eff.ContinuousApply != nil {
					eff.ContinuousApply(d, st, p)
				}
			}
		}
	}
}

// log emits a game event through the logger and notifies both players.
func (d *Duel) log(event log.GameEvent) {
	d.Logger.Log(event)
	// Notify controllers (ignore errors for notifications)
	for i := 0; i < 2; i++ {
		_ = d.Controllers[i].Notify(d.ctx, event)
	}
}
