package game

import (
	"fmt"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// computeMainPhaseActions returns all legal actions for the turn player in a Main Phase.
func (d *Duel) computeMainPhaseActions(player int) []Action {
	gs := d.State
	p := gs.Players[player]
	var actions []Action

	freeZones := p.FreeAgentZones()
	hasFreeZone := len(freeZones) > 0

	// Normal Summon / Normal Set (once per turn)
	if !gs.NormalSummonUsed {
		for _, card := range p.Hand {
			if card.Card.CardType != CardTypeAgent {
				continue
			}
			sacrifices := card.Card.SacrificesRequired()

			if sacrifices == 0 && hasFreeZone {
				// Normal Summon (L1-4)
				actions = append(actions, Action{
					Type:   ActionNormalSummon,
					Player: player,
					Card:   card,
					Zone:   freeZones[0],
					Desc:   fmt.Sprintf("Normal Summon %s (ATK %d) to Zone %d", card.Card.Name, card.Card.ATK, freeZones[0]+1),
				})
				// Normal Set (L1-4)
				actions = append(actions, Action{
					Type:   ActionNormalSet,
					Player: player,
					Card:   card,
					Zone:   freeZones[0],
					Desc:   fmt.Sprintf("Set %s in Zone %d", card.Card.Name, freeZones[0]+1),
				})
			} else if sacrifices > 0 && p.AgentCount() >= sacrifices {
				// Sacrifice Summon/Set — need enough agents to sacrifice
				// We check if there's a zone available after sacrificing.
				// (Sacrificing opens a zone, so we always have space if we can sacrifice.)
				actions = append(actions, Action{
					Type:   ActionSacrificeSummon,
					Player: player,
					Card:   card,
					Desc:   fmt.Sprintf("Sacrifice Summon %s (requires %d sacrifice(s))", card.Card.Name, sacrifices),
				})
				actions = append(actions, Action{
					Type:   ActionSacrificeSet,
					Player: player,
					Card:   card,
					Desc:   fmt.Sprintf("Sacrifice Set %s (requires %d sacrifice(s))", card.Card.Name, sacrifices),
				})
			}
		}
	}

	// Flip Summon: face-down DEF agents that weren't set this turn
	for _, m := range p.AgentZones {
		if m != nil && m.Face == FaceDown && m.Position == PositionDEF && m.TurnPlaced < gs.Turn {
			actions = append(actions, Action{
				Type:   ActionFlipSummon,
				Player: player,
				Card:   m,
				Desc:   fmt.Sprintf("Flip Summon %s in Zone %d", m.Card.Name, m.ZoneIndex+1),
			})
		}
	}

	// Change position: face-up agents that haven't changed position, weren't summoned this turn, didn't attack
	for _, m := range p.AgentZones {
		if m == nil || m.Face == FaceDown {
			continue
		}
		if m.TurnPlaced >= gs.Turn && m.TurnControlChanged < gs.Turn {
			continue // can't change position of agent placed this turn (unless control changed this turn)
		}
		if m.PositionChangedThisTurn {
			continue
		}
		if m.AttackedThisTurn {
			continue
		}
		newPos := PositionDEF
		if m.Position == PositionDEF {
			newPos = PositionATK
		}
		actions = append(actions, Action{
			Type:   ActionChangePosition,
			Player: player,
			Card:   m,
			Desc:   fmt.Sprintf("Change %s to %s position", m.Card.Name, newPos),
		})
	}

	// Tech set actions: for each program/trap in hand, if free tech zone
	freeTechZones := p.FreeTechZones()
	hasFreeTechZone := len(freeTechZones) > 0
	if hasFreeTechZone {
		for _, card := range p.Hand {
			if card.Card.CardType == CardTypeProgram || card.Card.CardType == CardTypeTrap {
				actions = append(actions, Action{
					Type:   ActionSetTech,
					Player: player,
					Card:   card,
					Zone:   freeTechZones[0],
					Desc:   fmt.Sprintf("Set %s in Tech Zone %d", card.Card.Name, freeTechZones[0]+1),
				})
			}
		}
	}

	// Program activation from hand (SS1 normal programs, SS2 quick-play during own turn)
	for _, card := range p.Hand {
		if card.Card.CardType != CardTypeProgram {
			continue
		}
		if len(card.Card.Effects) == 0 {
			continue
		}
		for ei, eff := range card.Card.Effects {
			if eff.CanActivate != nil && !eff.CanActivate(d, card, player) {
				continue
			}
			// OS programs don't need tech zone, they use the OS zone
			if card.Card.ProgramSub == ProgramOS {
				actions = append(actions, Action{
					Type:        ActionActivate,
					Player:      player,
					Card:        card,
					EffectIndex: ei,
					Desc:        fmt.Sprintf("Activate %s", card.Card.Name),
				})
				continue
			}
			// Need a free tech zone to activate from hand
			if !hasFreeTechZone {
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

	// Trap/quick-play activation from field (set cards not set this turn)
	for _, card := range p.FaceDownTech() {
		if card.TurnPlaced >= gs.Turn {
			continue // can't activate card set this turn
		}
		if len(card.Card.Effects) == 0 {
			continue
		}
		for ei, eff := range card.Card.Effects {
			if eff.CanActivate != nil && !eff.CanActivate(d, card, player) {
				continue
			}
			// Skip trigger effects — they activate in response windows
			if eff.IsTrigger {
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

	// Agent ignition effects (face-up agents with ignition effects)
	for _, m := range p.FaceUpAgents() {
		if m.Card.CardType != CardTypeAgent || !m.Card.IsEffect {
			continue
		}
		for ei, eff := range m.Card.Effects {
			if eff.EffectType != EffectIgnition {
				continue
			}
			if eff.CanActivate != nil && !eff.CanActivate(d, m, player) {
				continue
			}
			actions = append(actions, Action{
				Type:        ActionActivate,
				Player:      player,
				Card:        m,
				EffectIndex: ei,
				Desc:        fmt.Sprintf("Activate %s effect", m.Card.Name),
			})
		}
	}

	// Special summon actions (agents with special summon conditions)
	actions = d.addSpecialSummonActions(player, actions)

	// Phase transitions
	if gs.Phase == PhaseMain1 {
		// Can enter battle phase (but not on turn 1)
		if gs.Turn > 1 || gs.TurnPlayer == 1 {
			actions = append(actions, Action{
				Type: ActionEnterBattlePhase,
				Desc: "Enter Battle Phase",
			})
		}
	}

	// End turn (always available)
	actions = append(actions, Action{
		Type: ActionEndTurn,
		Desc: "End Turn",
	})

	return actions
}

// executeNormalSummon performs a normal summon (L1-4, no sacrifice).
func (d *Duel) executeNormalSummon(action Action) error {
	gs := d.State
	p := gs.Players[action.Player]

	card := action.Card
	zone := action.Zone

	p.RemoveFromHand(card)
	card.Face = FaceUp
	card.Position = PositionATK
	card.TurnPlaced = gs.Turn
	card.Controller = action.Player
	p.PlaceAgent(card, zone)
	gs.NormalSummonUsed = true

	d.log(log.NewNormalSummonEvent(gs.Turn, gs.Phase.String(), action.Player, card.Card.Name, card.CurrentATK(), zone))

	// Store summon info for trigger effects
	gs.LastSummonEvent = &SummonEventInfo{Card: card, Player: action.Player}

	d.recalculateContinuousEffects()

	// Post-summon response window (e.g. Cascade Failure)
	if err := d.processEffectSerialization(log.EventNormalSummon); err != nil {
		return err
	}

	return nil
}

// executeNormalSet performs a normal set (face-down DEF).
func (d *Duel) executeNormalSet(action Action) error {
	gs := d.State
	p := gs.Players[action.Player]

	card := action.Card
	zone := action.Zone

	p.RemoveFromHand(card)
	card.Face = FaceDown
	card.Position = PositionDEF
	card.TurnPlaced = gs.Turn
	card.Controller = action.Player
	p.PlaceAgent(card, zone)
	gs.NormalSummonUsed = true

	d.log(log.NewSetAgentEvent(gs.Turn, gs.Phase.String(), action.Player, zone))

	return nil
}

// executeSacrificeSummon performs a sacrifice summon.
func (d *Duel) executeSacrificeSummon(action Action) error {
	gs := d.State
	p := gs.Players[action.Player]
	card := action.Card
	sacCount := card.Card.SacrificesRequired()

	// Ask player to choose sacrifice targets
	candidates := p.Agents()
	sacrifices, err := d.Controllers[action.Player].ChooseCards(
		d.ctx, gs,
		fmt.Sprintf("Choose %d agent(s) to sacrifice for %s", sacCount, card.Card.Name),
		candidates, sacCount, sacCount,
	)
	if err != nil {
		return err
	}

	// Send sacrifices to scrapheap and track which zone to use
	var sacrificeNames []string
	freeZone := -1
	for _, sac := range sacrifices {
		sacrificeNames = append(sacrificeNames, sac.Card.Name)
		zoneIdx := sac.ZoneIndex
		p.RemoveAgent(sac)
		p.SendToScrapheap(sac)
		d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), action.Player, sac.Card.Name, "sacrificed"))
		if freeZone == -1 {
			freeZone = zoneIdx
		}
	}

	// Place in freed zone (or first free zone)
	if freeZone == -1 {
		freeZone = p.FreeAgentZone()
	}

	p.RemoveFromHand(card)
	card.Face = FaceUp
	card.Position = PositionATK
	card.TurnPlaced = gs.Turn
	card.Controller = action.Player
	p.PlaceAgent(card, freeZone)
	gs.NormalSummonUsed = true

	d.log(log.NewSacrificeSummonEvent(gs.Turn, gs.Phase.String(), action.Player, card.Card.Name, card.CurrentATK(), freeZone, sacrificeNames))

	// Store summon info for trigger effects
	gs.LastSummonEvent = &SummonEventInfo{Card: card, Player: action.Player}

	d.recalculateContinuousEffects()

	// Post-summon response window
	if err := d.processEffectSerialization(log.EventSacrificeSummon); err != nil {
		return err
	}

	return nil
}

// executeSacrificeSet performs a sacrifice set (face-down DEF).
func (d *Duel) executeSacrificeSet(action Action) error {
	gs := d.State
	p := gs.Players[action.Player]
	card := action.Card
	sacCount := card.Card.SacrificesRequired()

	candidates := p.Agents()
	sacrifices, err := d.Controllers[action.Player].ChooseCards(
		d.ctx, gs,
		fmt.Sprintf("Choose %d agent(s) to sacrifice for setting %s", sacCount, card.Card.Name),
		candidates, sacCount, sacCount,
	)
	if err != nil {
		return err
	}

	freeZone := -1
	for _, sac := range sacrifices {
		zoneIdx := sac.ZoneIndex
		p.RemoveAgent(sac)
		p.SendToScrapheap(sac)
		d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), action.Player, sac.Card.Name, "sacrificed"))
		if freeZone == -1 {
			freeZone = zoneIdx
		}
	}

	if freeZone == -1 {
		freeZone = p.FreeAgentZone()
	}

	p.RemoveFromHand(card)
	card.Face = FaceDown
	card.Position = PositionDEF
	card.TurnPlaced = gs.Turn
	card.Controller = action.Player
	p.PlaceAgent(card, freeZone)
	gs.NormalSummonUsed = true

	d.log(log.NewSetAgentEvent(gs.Turn, gs.Phase.String(), action.Player, freeZone))

	return nil
}

// executeFlipSummon flips a face-down DEF agent to face-up ATK.
func (d *Duel) executeFlipSummon(action Action) error {
	gs := d.State
	card := action.Card

	card.Face = FaceUp
	card.Position = PositionATK
	card.PositionChangedThisTurn = true

	d.log(log.NewFlipSummonEvent(gs.Turn, gs.Phase.String(), action.Player, card.Card.Name, card.CurrentATK(), card.ZoneIndex))

	// Store summon info for trigger effects
	gs.LastSummonEvent = &SummonEventInfo{Card: card, Player: action.Player}

	// Check for flip effects on this agent
	d.queueFlipEffects(card, action.Player)

	// Post-summon response window (includes flip effects via effect serialization)
	return d.processEffectSerialization(log.EventFlipSummon)
}

// queueFlipEffects queues FLIP effects from a agent that was just flipped face-up.
func (d *Duel) queueFlipEffects(card *CardInstance, controller int) {
	if card.Card.CardType != CardTypeAgent || !card.Card.IsEffect {
		return
	}
	for _, eff := range card.Card.Effects {
		if eff.EffectType != EffectFlip {
			continue
		}
		if eff.CanActivate != nil && !eff.CanActivate(d, card, controller) {
			continue
		}
		d.State.PendingTriggers = append(d.State.PendingTriggers, PendingTrigger{
			Card:       card,
			Effect:     eff,
			Controller: controller,
		})
	}
}

// executeChangePosition changes a agent's battle position.
func (d *Duel) executeChangePosition(action Action) {
	gs := d.State
	card := action.Card

	if card.Position == PositionATK {
		card.Position = PositionDEF
	} else {
		card.Position = PositionATK
	}
	card.PositionChangedThisTurn = true

	d.log(log.NewChangePositionEvent(gs.Turn, gs.Phase.String(), action.Player, card.Card.Name, card.Position.String()))
}
