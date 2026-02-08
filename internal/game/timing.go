package game

import (
	"fmt"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// openResponseWindow gives both players the chance to chain fast effects.
// startingPlayer gets priority first. Players alternate until both pass consecutively.
func (d *Duel) openResponseWindow(startingPlayer int) error {
	gs := d.State
	if gs.Over {
		return nil
	}

	gs.InResponseWindow = true
	defer func() { gs.InResponseWindow = false }()

	passCount := 0
	currentPlayer := startingPlayer

	for passCount < 2 {
		if gs.Over {
			return nil
		}

		actions := d.computeFastEffectActions(currentPlayer)
		if len(actions) == 0 {
			// No fast effects available — auto-pass
			passCount++
			currentPlayer = gs.Opponent(currentPlayer)
			continue
		}

		chosen, err := d.Controllers[currentPlayer].ChooseAction(d.ctx, gs, actions)
		if err != nil {
			return err
		}

		if chosen.Type == ActionPass {
			passCount++
			currentPlayer = gs.Opponent(currentPlayer)
			continue
		}

		// Player activated something — add to chain
		if chosen.Type == ActionActivate {
			card := chosen.Card
			effect := card.Card.Effects[chosen.EffectIndex]

			// Handle targeting
			var targets []*CardInstance
			if effect.Target != nil {
				targets, err = effect.Target(d, card, currentPlayer)
				if err != nil {
					return err
				}
			}

			// Pay costs
			if effect.Cost != nil {
				ok, costErr := effect.Cost(d, card, currentPlayer)
				if costErr != nil {
					return costErr
				}
				if !ok {
					continue // cost cancelled, try again
				}
			}

			// If activating from field (set trap), flip face-up
			if card.Zone == ZoneTech && card.Face == FaceDown {
				card.Face = FaceUp
			}
			// If activating from hand (quick-play), place in tech zone
			if card.Zone == ZoneHand {
				p := gs.Players[currentPlayer]
				zone := p.FreeTechZone()
				if zone == -1 {
					return fmt.Errorf("no free tech zone for quick-play activation")
				}
				p.RemoveFromHand(card)
				card.Face = FaceUp
				card.TurnPlaced = gs.Turn
				card.Controller = currentPlayer
				p.PlaceTech(card, zone)
			}

			d.log(newActivateEventFromDuel(d, currentPlayer, card.Card.Name))

			if gs.Chain == nil {
				if err := d.startChain(card, effect, currentPlayer, targets); err != nil {
					return err
				}
			} else {
				if err := d.addToChain(card, effect, currentPlayer, targets); err != nil {
					return err
				}
			}

			// Reset pass count and give priority to opponent
			passCount = 0
			currentPlayer = gs.Opponent(currentPlayer)
		}
	}

	return nil
}

// computeFastEffectActions returns activatable fast effects (SS2+) for a player.
func (d *Duel) computeFastEffectActions(player int) []Action {
	gs := d.State
	p := gs.Players[player]
	var actions []Action

	topSS := d.topChainExecSpeed()

	// Set traps on field (not set this turn, SS2+)
	for _, card := range p.FaceDownTech() {
		if card.TurnPlaced >= gs.Turn {
			continue
		}
		for ei, eff := range card.Card.Effects {
			if eff.ExecSpeed < ExecSpeed2 {
				continue
			}
			if topSS > 0 && !canChainWith(topSS, eff.ExecSpeed) {
				continue
			}
			if eff.CanActivate != nil && !eff.CanActivate(d, card, player) {
				continue
			}
			actions = append(actions, Action{
				Type:        ActionActivate,
				Player:      player,
				Card:        card,
				EffectIndex: ei,
				Desc:        fmt.Sprintf("Activate %s", card.Card.Name),
			})
		}
	}

	// Quick-Play programs from hand (during own turn only in main phase, any phase from set field)
	// For simplicity: allow from hand if it's their turn or they have it set
	if player == gs.TurnPlayer {
		for _, card := range p.Hand {
			if card.Card.CardType != CardTypeProgram || card.Card.ProgramSub != ProgramQuickPlay {
				continue
			}
			if len(card.Card.Effects) == 0 {
				continue
			}
			if p.FreeTechZone() == -1 {
				continue
			}
			for ei, eff := range card.Card.Effects {
				if topSS > 0 && !canChainWith(topSS, eff.ExecSpeed) {
					continue
				}
				if eff.CanActivate != nil && !eff.CanActivate(d, card, player) {
					continue
				}
				actions = append(actions, Action{
					Type:        ActionActivate,
					Player:      player,
					Card:        card,
					EffectIndex: ei,
					Desc:        fmt.Sprintf("Activate %s from hand", card.Card.Name),
				})
			}
		}
	}

	// Always offer pass
	actions = append(actions, Action{
		Type:   ActionPass,
		Player: player,
		Desc:   "Pass",
	})

	return actions
}

// newActivateEventFromDuel creates an activate event using current duel state.
func newActivateEventFromDuel(d *Duel, player int, cardName string) log.GameEvent {
	gs := d.State
	return log.NewActivateEvent(gs.Turn, gs.Phase.String(), player, cardName)
}
