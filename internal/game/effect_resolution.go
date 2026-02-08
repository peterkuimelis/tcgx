package game

import "github.com/peterkuimelis/tcgx/internal/log"

// collectTriggers scans the field for trigger effects matching the given event type.
// Also includes any PendingTriggers that were queued (e.g. flip effects).
func (d *Duel) collectTriggers(eventType log.EventType) []PendingTrigger {
	gs := d.State
	var triggers []PendingTrigger

	// First, collect any pre-queued pending triggers (e.g. flip effects)
	triggers = append(triggers, gs.PendingTriggers...)
	gs.PendingTriggers = nil

	for p := 0; p < 2; p++ {
		// Check set traps in tech zone
		for _, card := range gs.Players[p].FaceDownTech() {
			if card.TurnPlaced >= gs.Turn {
				continue // can't activate card set this turn
			}
			for _, eff := range card.Card.Effects {
				if !eff.IsTrigger {
					continue
				}
				// Check if trigger matches: either exact event type match or TriggerFilter
				matches := eff.TriggerEvent == eventType
				if !matches && eff.TriggerFilter != nil {
					matches = eff.TriggerFilter(d, card, log.GameEvent{Type: eventType})
				}
				if !matches {
					continue
				}
				if eff.CanActivate != nil && !eff.CanActivate(d, card, p) {
					continue
				}
				triggers = append(triggers, PendingTrigger{
					Card:       card,
					Effect:     eff,
					Controller: p,
				})
			}
		}

		// Check face-up agents for trigger effects
		for _, card := range gs.Players[p].FaceUpAgents() {
			if card.Card.CardType != CardTypeAgent || !card.Card.IsEffect {
				continue
			}
			for _, eff := range card.Card.Effects {
				if eff.EffectType != EffectTrigger || !eff.IsTrigger {
					continue
				}
				matches := eff.TriggerEvent == eventType
				if !matches && eff.TriggerFilter != nil {
					matches = eff.TriggerFilter(d, card, log.GameEvent{Type: eventType})
				}
				if !matches {
					continue
				}
				if eff.CanActivate != nil && !eff.CanActivate(d, card, p) {
					continue
				}
				// Don't double-add if already in pending triggers
				alreadyQueued := false
				for _, t := range triggers {
					if t.Card.ID == card.ID && t.Effect == eff {
						alreadyQueued = true
						break
					}
				}
				if alreadyQueued {
					continue
				}
				triggers = append(triggers, PendingTrigger{
					Card:       card,
					Effect:     eff,
					Controller: p,
				})
			}
		}
	}

	return triggers
}

// processEffectSerialization handles simultaneous effect serialization after a game action.
// It collects trigger effects, orders them (TP mandatory → NTP mandatory → TP optional → NTP optional),
// builds a chain, opens response window, and resolves.
func (d *Duel) processEffectSerialization(eventType log.EventType) error {
	gs := d.State
	if gs.Over {
		return nil
	}

	triggers := d.collectTriggers(eventType)
	if len(triggers) == 0 {
		return nil
	}

	// Order: TP mandatory, NTP mandatory, TP optional, NTP optional
	tp := gs.TurnPlayer
	ntp := gs.Opponent(tp)
	var ordered []PendingTrigger

	// TP mandatory
	for _, t := range triggers {
		if t.Controller == tp && t.Effect.IsMandatory {
			ordered = append(ordered, t)
		}
	}
	// NTP mandatory
	for _, t := range triggers {
		if t.Controller == ntp && t.Effect.IsMandatory {
			ordered = append(ordered, t)
		}
	}
	// TP optional
	for _, t := range triggers {
		if t.Controller == tp && !t.Effect.IsMandatory {
			ordered = append(ordered, t)
		}
	}
	// NTP optional
	for _, t := range triggers {
		if t.Controller == ntp && !t.Effect.IsMandatory {
			ordered = append(ordered, t)
		}
	}

	// For optional triggers, ask the player if they want to activate
	var chainTriggers []PendingTrigger
	for _, t := range ordered {
		if t.Effect.IsMandatory {
			chainTriggers = append(chainTriggers, t)
		} else {
			// Ask controller if they want to activate
			yes, err := d.Controllers[t.Controller].ChooseYesNo(
				d.ctx, gs,
				"Activate "+t.Card.Card.Name+"?",
			)
			if err != nil {
				return err
			}
			if yes {
				chainTriggers = append(chainTriggers, t)
			}
		}
	}

	if len(chainTriggers) == 0 {
		return nil
	}

	// Build chain from triggers
	for i, t := range chainTriggers {
		// Flip trap face-up
		if t.Card.Zone == ZoneTech && t.Card.Face == FaceDown {
			t.Card.Face = FaceUp
		}

		d.log(log.NewActivateEvent(gs.Turn, gs.Phase.String(), t.Controller, t.Card.Card.Name))

		// Handle targeting
		var targets []*CardInstance
		if t.Effect.Target != nil {
			var err error
			targets, err = t.Effect.Target(d, t.Card, t.Controller)
			if err != nil {
				return err
			}
		}

		if i == 0 {
			if err := d.startChain(t.Card, t.Effect, t.Controller, targets); err != nil {
				return err
			}
		} else {
			if err := d.addToChain(t.Card, t.Effect, t.Controller, targets); err != nil {
				return err
			}
		}
	}

	// Open response window for other players to chain
	// Give priority to opponent of the last chain link's controller
	lastController := chainTriggers[len(chainTriggers)-1].Controller
	if err := d.openResponseWindow(gs.Opponent(lastController)); err != nil {
		return err
	}

	// Resolve the chain
	return d.resolveChain()
}
