package game

import (
	"fmt"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// ActionSpecialSummon is handled as part of ActionActivate for agent effects,
// but the actual placement is done via executeSpecialSummon.

// executeSpecialSummon places a agent on the field via special summon.
func (d *Duel) executeSpecialSummon(card *CardInstance, player int, position Position, face FaceStatus) error {
	gs := d.State
	p := gs.Players[player]

	zone := p.FreeAgentZone()
	if zone == -1 {
		return fmt.Errorf("no free agent zone for special summon")
	}

	card.Face = face
	card.Position = position
	card.TurnPlaced = gs.Turn
	card.Controller = player
	p.PlaceAgent(card, zone)

	d.log(log.NewSpecialSummonEvent(gs.Turn, gs.Phase.String(), player, card.Card.Name, card.CurrentATK(), zone))

	// Store summon info for trigger effects
	gs.LastSummonEvent = &SummonEventInfo{Card: card, Player: player}

	d.recalculateContinuousEffects()

	// Post-special-summon effect serialization
	if err := d.processEffectSerialization(log.EventSpecialSummon); err != nil {
		return err
	}

	return nil
}

// addSpecialSummonActions adds main phase actions for agents with special summon conditions.
// This is called from computeMainPhaseActions.
func (d *Duel) addSpecialSummonActions(player int, actions []Action) []Action {
	gs := d.State
	p := gs.Players[player]

	if p.FreeAgentZone() == -1 {
		return actions
	}

	// Check hand for agents with special summon conditions
	for _, card := range p.Hand {
		if card.Card.CardType != CardTypeAgent {
			continue
		}
		for ei, eff := range card.Card.Effects {
			if eff.SpecialSummonCondition == nil {
				continue
			}
			if !eff.SpecialSummonCondition(d, card, player) {
				continue
			}
			actions = append(actions, Action{
				Type:        ActionActivate,
				Player:      player,
				Card:        card,
				EffectIndex: ei,
				Desc:        fmt.Sprintf("Special Summon %s", card.Card.Name),
			})
		}
	}

	return actions
}

// removeFromScrapheap removes a card from a player's scrapheap by instance ID.
func (d *Duel) removeFromScrapheap(player int, card *CardInstance) {
	p := d.State.Players[player]
	for i, c := range p.Scrapheap {
		if c.ID == card.ID {
			p.Scrapheap = append(p.Scrapheap[:i], p.Scrapheap[i+1:]...)
			return
		}
	}
}

// purgeFromScrapheap removes a card from scrapheap and moves it to purged zone.
func (d *Duel) purgeFromScrapheap(player int, card *CardInstance, reason string) {
	gs := d.State
	d.removeFromScrapheap(player, card)
	card.Zone = ZonePurged
	gs.Players[player].Purged = append(gs.Players[player].Purged, card)
	d.log(log.NewPurgeEvent(gs.Turn, gs.Phase.String(), player, card.Card.Name, reason))
}

// purgeFromField removes a card from the field and moves it to purged zone.
func (d *Duel) purgeFromField(card *CardInstance, reason string) {
	gs := d.State
	controller := card.Controller

	d.triggerOnLeaveField(card)

	switch card.Zone {
	case ZoneAgent:
		d.destroyEquips(card)
		gs.Players[controller].RemoveAgent(card)
	case ZoneTech:
		if card.EquippedTo != nil {
			d.detachEquip(card)
		}
		gs.Players[controller].RemoveFromTech(card)
	}
	card.Zone = ZonePurged
	gs.Players[card.Owner].Purged = append(gs.Players[card.Owner].Purged, card)
	d.log(log.NewPurgeEvent(gs.Turn, gs.Phase.String(), card.Owner, card.Card.Name, reason))
}

// changeControl moves a agent from one player's field to another's.
func (d *Duel) changeControl(card *CardInstance, newController int) error {
	gs := d.State
	oldController := card.Controller

	if oldController == newController {
		return nil
	}

	// Remove from old controller's zone
	gs.Players[oldController].RemoveAgent(card)

	// Place in new controller's zone
	zone := gs.Players[newController].FreeAgentZone()
	if zone == -1 {
		return fmt.Errorf("no free agent zone for control change")
	}

	card.Controller = newController
	card.TurnControlChanged = gs.Turn
	gs.Players[newController].PlaceAgent(card, zone)

	d.log(log.NewChangeControlEvent(gs.Turn, gs.Phase.String(), oldController, card.Card.Name, newController))

	return nil
}
