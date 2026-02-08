package game

import (
	"fmt"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// executeSetTech sets a program/trap from hand face-down in the tech zone.
func (d *Duel) executeSetTech(action Action) error {
	gs := d.State
	p := gs.Players[action.Player]

	card := action.Card
	zone := action.Zone

	p.RemoveFromHand(card)
	card.Face = FaceDown
	card.Position = PositionATK // Tech cards don't really have position, but default
	card.TurnPlaced = gs.Turn
	card.Controller = action.Player
	p.PlaceTech(card, zone)

	d.log(log.NewSetTechEvent(gs.Turn, gs.Phase.String(), action.Player, zone))

	return nil
}

// executeActivateEffect routes activation to the correct handler based on card type.
func (d *Duel) executeActivateEffect(action Action) error {
	card := action.Card
	switch card.Card.CardType {
	case CardTypeAgent:
		return d.executeActivateAgentEffect(action)
	default:
		return d.executeActivateProgram(action)
	}
}

// executeActivateAgentEffect activates a agent's ignition effect.
func (d *Duel) executeActivateAgentEffect(action Action) error {
	gs := d.State
	card := action.Card
	effect := card.Card.Effects[action.EffectIndex]

	d.log(log.NewActivateEvent(gs.Turn, gs.Phase.String(), action.Player, card.Card.Name+" effect"))

	// Handle targeting
	var targets []*CardInstance
	if effect.Target != nil {
		var err error
		targets, err = effect.Target(d, card, action.Player)
		if err != nil {
			return err
		}
	}

	// Pay costs
	if effect.Cost != nil {
		ok, err := effect.Cost(d, card, action.Player)
		if err != nil {
			return err
		}
		if !ok {
			return nil // cost cancelled
		}
	}

	// Start chain
	return d.startChain(card, effect, action.Player, targets)
}

// executeActivateProgram activates a program card (from hand or field).
func (d *Duel) executeActivateProgram(action Action) error {
	gs := d.State
	p := gs.Players[action.Player]
	card := action.Card
	effect := card.Card.Effects[action.EffectIndex]

	// If activating from hand, place on field face-up
	if card.Zone == ZoneHand {
		p.RemoveFromHand(card)
		card.Face = FaceUp
		card.TurnPlaced = gs.Turn
		card.Controller = action.Player

		if card.Card.ProgramSub == ProgramOS {
			// Operating system: place in OS zone
			// If opponent has a OS, destroy it first (Goat rule: only one total)
			opp := gs.Opponent(action.Player)
			if gs.Players[opp].OS != nil {
				d.destroyOS(opp)
			}
			// If we already have a OS, destroy it
			if p.OS != nil {
				d.destroyOS(action.Player)
			}
			p.OS = card
			card.Zone = ZoneOS
		} else {
			zone := p.FreeTechZone()
			if zone == -1 {
				return fmt.Errorf("no free tech zone")
			}
			p.PlaceTech(card, zone)
		}
	} else {
		// Already on field (set quick-play) — flip face-up
		card.Face = FaceUp
	}

	d.log(log.NewActivateEvent(gs.Turn, gs.Phase.String(), action.Player, card.Card.Name))

	// Handle targeting
	var targets []*CardInstance
	if effect.Target != nil {
		var err error
		targets, err = effect.Target(d, card, action.Player)
		if err != nil {
			return err
		}
	}

	// Pay costs
	if effect.Cost != nil {
		ok, err := effect.Cost(d, card, action.Player)
		if err != nil {
			return err
		}
		if !ok {
			return nil // cost cancelled
		}
	}

	// Start chain
	if err := d.startChain(card, effect, action.Player, targets); err != nil {
		return err
	}

	return nil
}

// executeActivateTrap activates a trap card from the field.
func (d *Duel) executeActivateTrap(action Action) error {
	gs := d.State
	card := action.Card
	effect := card.Card.Effects[action.EffectIndex]

	// Flip face-up
	card.Face = FaceUp

	d.log(log.NewActivateEvent(gs.Turn, gs.Phase.String(), action.Player, card.Card.Name))

	// Handle targeting
	var targets []*CardInstance
	if effect.Target != nil {
		var err error
		targets, err = effect.Target(d, card, action.Player)
		if err != nil {
			return err
		}
	}

	// Pay costs
	if effect.Cost != nil {
		ok, err := effect.Cost(d, card, action.Player)
		if err != nil {
			return err
		}
		if !ok {
			return nil // cost cancelled
		}
	}

	// Add to chain (start if no chain, add if chain exists)
	if gs.Chain == nil {
		if err := d.startChain(card, effect, action.Player, targets); err != nil {
			return err
		}
	} else {
		if err := d.addToChain(card, effect, action.Player, targets); err != nil {
			return err
		}
	}

	return nil
}

// --- Effect helper functions used by card closures ---

// destroyByEffect removes a card from the field and sends it to scrapheap.
func (d *Duel) destroyByEffect(card *CardInstance, reason string) {
	gs := d.State
	controller := card.Controller

	d.log(log.NewDestroyEvent(gs.Turn, gs.Phase.String(), controller, card.Card.Name, reason))

	// Trigger OnLeaveField handlers before detaching/removing
	d.triggerOnLeaveField(card)

	switch card.Zone {
	case ZoneAgent:
		// Destroy equips attached to this agent
		d.destroyEquips(card)
		gs.Players[controller].RemoveAgent(card)
	case ZoneTech:
		// If this is an equip card, detach from its target
		if card.EquippedTo != nil {
			d.detachEquip(card)
		}
		gs.Players[controller].RemoveFromTech(card)
	case ZoneOS:
		gs.Players[controller].OS = nil
	}

	gs.Players[card.Owner].SendToScrapheap(card)
	d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), card.Owner, card.Card.Name, "destroyed by "+reason))
	d.recalculateContinuousEffects()
}

// destroyAllAgents destroys all agents on the field (Void Purge / Cascade Failure).
func (d *Duel) destroyAllAgents(reason string) {
	gs := d.State
	for p := 0; p < 2; p++ {
		for _, m := range gs.Players[p].Agents() {
			d.destroyByEffect(m, reason)
		}
	}
}

// destroyAllTech destroys all program/trap cards on the field (EMP Cascade).
func (d *Duel) destroyAllTech(reason string) {
	gs := d.State
	for p := 0; p < 2; p++ {
		for _, st := range gs.Players[p].TechCards() {
			d.destroyByEffect(st, reason)
		}
	}
}

// flipFaceDown flips a face-up agent to face-down DEF (Blackout Patch).
func (d *Duel) flipFaceDown(card *CardInstance) {
	gs := d.State
	// Destroy equips when flipped face-down
	d.destroyEquips(card)
	card.Face = FaceDown
	card.Position = PositionDEF
	d.log(log.NewFlipFaceDownEvent(gs.Turn, gs.Phase.String(), card.Controller, card.Card.Name))
}
