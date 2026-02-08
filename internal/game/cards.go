package game

import (
	"fmt"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// GreedProtocol — SS1 Normal Program. Draw 2 cards.
func GreedProtocol() *Card {
	eff := &CardEffect{
		Name:      "Greed Protocol",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			return d.State.Players[player].DeckCount() >= 2
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			p := gs.Players[player]
			for i := 0; i < 2; i++ {
				drawn := p.DrawCard()
				if drawn != nil {
					d.log(log.NewDrawEvent(gs.Turn, gs.Phase.String(), player, drawn.Card.Name))
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Greed Protocol",
		Description: "Draw 2 cards.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// VoidPurge — SS1 Normal Program. Destroy all agents on the field.
func VoidPurge() *Card {
	eff := &CardEffect{
		Name:      "Void Purge",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			// Must be at least one agent on the field
			for p := 0; p < 2; p++ {
				if d.State.Players[p].AgentCount() > 0 {
					return true
				}
			}
			return false
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			d.destroyAllAgents("Void Purge")
			return nil
		},
	}
	return &Card{
		Name:        "Void Purge",
		Description: "Destroy all agents on the field.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// EMPCascade — SS1 Normal Program. Destroy all Program/Trap cards on the field.
func EMPCascade() *Card {
	eff := &CardEffect{
		Name:      "EMP Cascade",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			// Must be at least one other Tech on field
			for p := 0; p < 2; p++ {
				for _, st := range d.State.Players[p].TechCards() {
					if st.ID != card.ID {
						return true
					}
				}
			}
			return false
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			// Destroy all Tech on field except this card (it goes to Scrapheap via handlePostResolution)
			gs := d.State
			for p := 0; p < 2; p++ {
				for _, st := range gs.Players[p].TechCards() {
					if st.ID != card.ID {
						d.destroyByEffect(st, "EMP Cascade")
					}
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "EMP Cascade",
		Description: "Destroy all Program and Trap cards on the field.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// ICEBreaker — SS2 Quick-Play Program. Target 1 Tech; destroy it.
func ICEBreaker() *Card {
	eff := &CardEffect{
		Name:      "ICE Breaker",
		ExecSpeed: ExecSpeed2,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			// Must be a Tech on field to target (not itself)
			for p := 0; p < 2; p++ {
				for _, st := range d.State.Players[p].TechCards() {
					if st.ID != card.ID {
						return true
					}
				}
			}
			return false
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for p := 0; p < 2; p++ {
				for _, st := range d.State.Players[p].TechCards() {
					if st.ID != card.ID {
						candidates = append(candidates, st)
					}
				}
			}
			if len(candidates) == 0 {
				return nil, nil
			}
			chosen, err := d.Controllers[player].ChooseCards(
				d.ctx, d.State, "Choose 1 Program/Trap to destroy", candidates, 1, 1,
			)
			if err != nil {
				return nil, err
			}
			return chosen, nil
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			for _, t := range targets {
				if d.isOnField(t) {
					d.destroyByEffect(t, "ICE Breaker")
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "ICE Breaker",
		Description: "Target 1 Program or Trap card; destroy it.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramQuickPlay,
		Effects:     []*CardEffect{eff},
	}
}

// BlackoutPatch — SS2 Quick-Play Program. Target 1 face-up agent; flip it face-down.
func BlackoutPatch() *Card {
	eff := &CardEffect{
		Name:      "Blackout Patch",
		ExecSpeed: ExecSpeed2,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			for p := 0; p < 2; p++ {
				if len(d.State.Players[p].FaceUpAgents()) > 0 {
					return true
				}
			}
			return false
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for p := 0; p < 2; p++ {
				candidates = append(candidates, d.State.Players[p].FaceUpAgents()...)
			}
			chosen, err := d.Controllers[player].ChooseCards(
				d.ctx, d.State, "Choose 1 face-up agent to flip face-down", candidates, 1, 1,
			)
			if err != nil {
				return nil, err
			}
			return chosen, nil
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			for _, t := range targets {
				if d.isOnField(t) && t.Face == FaceUp {
					d.flipFaceDown(t)
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Blackout Patch",
		Description: "Target 1 face-up agent; flip it face-down.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramQuickPlay,
		Effects:     []*CardEffect{eff},
	}
}

// ReactivePlating — SS2 Normal Trap. When opponent's agent attacks: destroy it.
func ReactivePlating() *Card {
	eff := &CardEffect{
		Name:      "Reactive Plating",
		ExecSpeed: ExecSpeed2,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			gs := d.State
			// Can only activate when an opponent's agent declares an attack
			if gs.CurrentAttacker == nil {
				return false
			}
			return gs.CurrentAttacker.Controller != player
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			if gs.CurrentAttacker != nil && d.isOnField(gs.CurrentAttacker) {
				d.destroyByEffect(gs.CurrentAttacker, "Reactive Plating")
			}
			return nil
		},
	}
	return &Card{
		Name:        "Reactive Plating",
		Description: "When an opponent's agent declares an attack: Destroy the attacking agent.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapNormal,
		Effects:     []*CardEffect{eff},
	}
}

// ReflectorArray — SS2 Normal Trap. When opponent's agent attacks: destroy all ATK position agents.
func ReflectorArray() *Card {
	eff := &CardEffect{
		Name:      "Reflector Array",
		ExecSpeed: ExecSpeed2,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			gs := d.State
			if gs.CurrentAttacker == nil {
				return false
			}
			return gs.CurrentAttacker.Controller != player
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			opp := gs.Opponent(player)
			for _, m := range gs.Players[opp].FaceUpATKAgents() {
				d.destroyByEffect(m, "Reflector Array")
			}
			return nil
		},
	}
	return &Card{
		Name:        "Reflector Array",
		Description: "When an opponent's agent declares an attack: Destroy all ATK Position agents your opponent controls.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapNormal,
		Effects:     []*CardEffect{eff},
	}
}

// CascadeFailure — SS2 Normal Trap. When a agent is summoned: destroy all agents.
func CascadeFailure() *Card {
	eff := &CardEffect{
		Name:         "Cascade Failure",
		ExecSpeed:    ExecSpeed2,
		IsTrigger:    true,
		TriggerEvent: log.EventNormalSummon,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			// Can activate when any agent is summoned (tracked by LastSummonEvent)
			return d.State.LastSummonEvent != nil
		},
		TriggerFilter: func(d *Duel, card *CardInstance, event log.GameEvent) bool {
			return event.Type == log.EventNormalSummon || event.Type == log.EventSacrificeSummon ||
				event.Type == log.EventFlipSummon || event.Type == log.EventSpecialSummon
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			d.destroyAllAgents("Cascade Failure")
			return nil
		},
	}
	return &Card{
		Name:        "Cascade Failure",
		Description: "When a agent is Summoned: Destroy all agents on the field.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapNormal,
		Effects:     []*CardEffect{eff},
	}
}

// SelfDestructCircuit — SS2 Normal Trap. Target 1 face-up agent; destroy it, both players take damage equal to its ATK.
func SelfDestructCircuit() *Card {
	eff := &CardEffect{
		Name:      "Self-Destruct Circuit",
		ExecSpeed: ExecSpeed2,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			for p := 0; p < 2; p++ {
				if len(d.State.Players[p].FaceUpAgents()) > 0 {
					return true
				}
			}
			return false
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for p := 0; p < 2; p++ {
				candidates = append(candidates, d.State.Players[p].FaceUpAgents()...)
			}
			chosen, err := d.Controllers[player].ChooseCards(
				d.ctx, d.State, "Choose 1 face-up agent to destroy", candidates, 1, 1,
			)
			if err != nil {
				return nil, err
			}
			return chosen, nil
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			for _, t := range targets {
				if d.isOnField(t) {
					atk := t.CurrentATK()
					d.destroyByEffect(t, "Self-Destruct Circuit")
					// Both players take damage equal to its ATK
					// Goat format: turn player takes damage first
					gs := d.State
					tp := gs.TurnPlayer
					ntp := gs.Opponent(tp)
					d.applyDamage(tp, atk, "Self-Destruct Circuit")
					if !gs.Over {
						d.applyDamage(ntp, atk, "Self-Destruct Circuit")
					}
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Self-Destruct Circuit",
		Description: "Target 1 face-up agent; destroy it, then both players take damage equal to its ATK.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapNormal,
		Effects:     []*CardEffect{eff},
	}
}

// RootOverride — SS3 Counter Trap. Pay half HP; negate a summon or Tech activation and destroy it.
func RootOverride() *Card {
	eff := &CardEffect{
		Name:      "Root Override",
		ExecSpeed: ExecSpeed3,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			gs := d.State
			// Can activate when a summon or Tech activation is happening (chain exists)
			if gs.Chain == nil || len(gs.Chain.Links) == 0 {
				return false
			}
			// Check that the top chain link is something we can negate
			topLink := gs.Chain.Links[len(gs.Chain.Links)-1]
			topCard := topLink.Card
			// Can negate program/trap activations
			if topCard.Card.CardType == CardTypeProgram || topCard.Card.CardType == CardTypeTrap {
				return true
			}
			return false
		},
		Cost: func(d *Duel, card *CardInstance, player int) (bool, error) {
			gs := d.State
			p := gs.Players[player]
			cost := p.HP / 2
			if cost <= 0 {
				return false, nil
			}
			oldHP := p.HP
			p.HP -= cost
			d.log(log.NewHPChangeEvent(gs.Turn, gs.Phase.String(), player, oldHP, p.HP, "Root Override cost"))
			return true, nil
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			// Negate the previous chain link by removing the card
			// In our simplified model: the card being negated was already placed on field
			// We destroy it and its effect doesn't resolve (it was already resolved in LIFO before us,
			// so we need a different approach)
			// Actually in LIFO, Solemn resolves FIRST (it's higher CL).
			// So we need to mark the negated link. For simplicity, we destroy the CL1 card.
			gs := d.State
			if gs.Chain != nil && len(gs.Chain.Links) > 0 {
				// Find the link we're negating (the one below us)
				myIndex := -1
				for i, link := range gs.Chain.Links {
					if link.Card.ID == card.ID {
						myIndex = i
						break
					}
				}
				if myIndex > 0 {
					negated := gs.Chain.Links[myIndex-1]
					// Destroy the negated card
					if d.isOnField(negated.Card) {
						d.destroyByEffect(negated.Card, "negated by Root Override")
					}
					// Mark the link as negated by nilling out its resolve
					gs.Chain.Links[myIndex-1].Effect = &CardEffect{
						Name: "Negated",
						Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
							return nil // does nothing
						},
					}
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Root Override",
		Description: "Pay half your HP, rounded down; negate the activation of a Program or Trap card and destroy it.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapCounter,
		Effects:     []*CardEffect{eff},
	}
}

// --- Phase 3: Agents ---

// BreakerTheChromeWarrior — Effect Agent. On summon: gain 1 tech counter.
// Ignition: remove counter to destroy 1 Tech. 1600 ATK (+300 with counter).
func BreakerTheChromeWarrior() *Card {
	summonEffect := &CardEffect{
		Name:         "Breaker Tech Counter",
		ExecSpeed:    ExecSpeed1,
		EffectType:   EffectTrigger,
		IsTrigger:    true,
		IsMandatory:  true,
		TriggerEvent: log.EventNormalSummon,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			return d.State.LastSummonEvent != nil && d.State.LastSummonEvent.Card.ID == card.ID
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			card.Counters["tech"]++
			card.AddModifier(StatModifier{Source: card.ID, ATKMod: 300})
			return nil
		},
	}

	ignitionEffect := &CardEffect{
		Name:       "Breaker Destroy Tech",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectIgnition,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			if card.Counters["tech"] <= 0 {
				return false
			}
			for p := 0; p < 2; p++ {
				if len(d.State.Players[p].TechCards()) > 0 {
					return true
				}
			}
			return false
		},
		Cost: func(d *Duel, card *CardInstance, player int) (bool, error) {
			card.Counters["tech"]--
			card.RemoveModifiersBySource(card.ID)
			return true, nil
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for p := 0; p < 2; p++ {
				candidates = append(candidates, d.State.Players[p].TechCards()...)
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose 1 Tech to destroy", candidates, 1, 1)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			for _, t := range targets {
				if d.isOnField(t) {
					d.destroyByEffect(t, "Breaker the Chrome Warrior")
				}
			}
			return nil
		},
	}

	return &Card{
		Name:        "Breaker the Chrome Warrior",
		Description: "When this card is Summoned: Place 1 Tech Counter on it (max. 1). This card gains 300 ATK for each Tech Counter on it. You can remove 1 Tech Counter from this card to target 1 Program/Trap; destroy that target.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrDARK,
		AgentType:   "Hacker",
		ATK:         1600,
		DEF:         1000,
		IsEffect:    true,
		Effects:     []*CardEffect{summonEffect, ignitionEffect},
	}
}

// PolymorphicVirus — Effect Agent. Ignition: discard 1, declare type, destroy all face-up of that type.
func PolymorphicVirus() *Card {
	eff := &CardEffect{
		Name:       "Polymorphic Virus",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectIgnition,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			if len(d.State.Players[player].Hand) == 0 {
				return false
			}
			for p := 0; p < 2; p++ {
				if len(d.State.Players[p].FaceUpAgents()) > 0 {
					return true
				}
			}
			return false
		},
		Cost: func(d *Duel, card *CardInstance, player int) (bool, error) {
			gs := d.State
			p := gs.Players[player]
			if len(p.Hand) == 0 {
				return false, nil
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Discard 1 card", p.Hand, 1, 1)
			if err != nil {
				return false, err
			}
			p.RemoveFromHand(chosen[0])
			p.SendToScrapheap(chosen[0])
			d.log(log.NewDiscardEvent(gs.Turn, gs.Phase.String(), player, chosen[0].Card.Name))
			return true, nil
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			var faceUp []*CardInstance
			for p := 0; p < 2; p++ {
				faceUp = append(faceUp, d.State.Players[p].FaceUpAgents()...)
			}
			if len(faceUp) == 0 {
				return nil
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose a agent (all face-up of same type destroyed)", faceUp, 1, 1)
			if err != nil {
				return err
			}
			declaredType := chosen[0].Card.AgentType
			for p := 0; p < 2; p++ {
				for _, m := range d.State.Players[p].FaceUpAgents() {
					if m.Card.AgentType == declaredType {
						d.destroyByEffect(m, "Polymorphic Virus")
					}
				}
			}
			return nil
		},
	}

	return &Card{
		Name:        "Polymorphic Virus",
		Description: "Discard 1 card, then target 1 face-up agent on the field; destroy all face-up agents of the same type.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrWATER,
		AgentType:   "Wetware",
		ATK:         1600,
		DEF:         1000,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// RecursiveWorm — Effect Agent. During your Standby Phase, if in Scrapheap, add to hand.
func RecursiveWorm() *Card {
	eff := &CardEffect{
		Name:         "Recursive Worm",
		ExecSpeed:    ExecSpeed1,
		EffectType:   EffectTrigger,
		IsTrigger:    true,
		IsMandatory:  false,
		TriggerEvent: log.EventPhaseChange,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			return card.Zone == ZoneScrapheap && d.State.Phase == PhaseStandby && d.State.TurnPlayer == player
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			d.removeFromScrapheap(player, card)
			card.Zone = ZoneHand
			gs.Players[player].Hand = append(gs.Players[player].Hand, card)
			d.log(log.NewAddToHandEvent(gs.Turn, gs.Phase.String(), player, card.Card.Name, "Recursive Worm effect"))
			return nil
		},
	}

	return &Card{
		Name:        "Recursive Worm",
		Description: "During your Standby Phase, if this card is in your Scrapheap: Add it to your hand.",
		CardType:    CardTypeAgent,
		Level:       1,
		Attribute:   AttrWATER,
		AgentType:   "Splice",
		ATK:         300,
		DEF:         250,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// Datamancer — FLIP: Add 1 Program from your Scrapheap to your hand.
func Datamancer() *Card {
	eff := &CardEffect{
		Name:        "Datamancer",
		ExecSpeed:   ExecSpeed1,
		EffectType:  EffectFlip,
		IsTrigger:   true,
		IsMandatory: false,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeProgram {
					return true
				}
			}
			return false
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeProgram {
					candidates = append(candidates, c)
				}
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose 1 Program from Scrapheap to add to hand", candidates, 1, 1)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			for _, t := range targets {
				inScrapheap := false
				for _, c := range gs.Players[player].Scrapheap {
					if c.ID == t.ID {
						inScrapheap = true
						break
					}
				}
				if inScrapheap {
					d.removeFromScrapheap(player, t)
					t.Zone = ZoneHand
					gs.Players[player].Hand = append(gs.Players[player].Hand, t)
					d.log(log.NewAddToHandEvent(gs.Turn, gs.Phase.String(), player, t.Card.Name, "Datamancer"))
				}
			}
			return nil
		},
	}

	return &Card{
		Name:        "Datamancer",
		Description: "FLIP: Target 1 Program in your Scrapheap; add that target to your hand.",
		CardType:    CardTypeAgent,
		Level:       1,
		Attribute:   AttrLIGHT,
		AgentType:   "Hacker",
		ATK:         300,
		DEF:         400,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// MorphCanister — FLIP: Both players discard hand, draw 5.
func MorphCanister() *Card {
	eff := &CardEffect{
		Name:        "Morph Canister",
		ExecSpeed:   ExecSpeed1,
		EffectType:  EffectFlip,
		IsTrigger:   true,
		IsMandatory: true,
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			for p := 0; p < 2; p++ {
				for len(gs.Players[p].Hand) > 0 {
					c := gs.Players[p].Hand[0]
					gs.Players[p].RemoveFromHand(c)
					gs.Players[p].SendToScrapheap(c)
					d.log(log.NewDiscardEvent(gs.Turn, gs.Phase.String(), p, c.Card.Name))
				}
				for i := 0; i < 5; i++ {
					drawn := gs.Players[p].DrawCard()
					if drawn != nil {
						d.log(log.NewDrawEvent(gs.Turn, gs.Phase.String(), p, drawn.Card.Name))
					}
				}
			}
			return nil
		},
	}

	return &Card{
		Name:        "Morph Canister",
		Description: "FLIP: Both players discard their entire hand, then both players draw 5 cards.",
		CardType:    CardTypeAgent,
		Level:       2,
		Attribute:   AttrEARTH,
		AgentType:   "Monolith",
		ATK:         700,
		DEF:         600,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// AeroKnightParshath — Effect Agent. Piercing damage. When deals battle damage, draw 1.
func AeroKnightParshath() *Card {
	piercingEffect := &CardEffect{
		Name:        "Aero-Knight Piercing",
		EffectType:  EffectContinuous,
		HasPiercing: true,
		OnBattleDamage: func(d *Duel, card *CardInstance, player int) {
			gs := d.State
			p := gs.Players[player]
			drawn := p.DrawCard()
			if drawn != nil {
				d.log(log.NewDrawEvent(gs.Turn, gs.Phase.String(), player, drawn.Card.Name))
			}
		},
	}

	return &Card{
		Name:        "Aero-Knight Parshath",
		Description: "This card can inflict piercing battle damage. When this card inflicts battle damage to your opponent: Draw 1 card.",
		CardType:    CardTypeAgent,
		Level:       5,
		Attribute:   AttrLIGHT,
		AgentType:   "Specter",
		ATK:         1900,
		DEF:         1400,
		IsEffect:    true,
		Effects:     []*CardEffect{piercingEffect},
	}
}

// ChromePaladinEnvoy — Special summon: purge 1 LIGHT + 1 DARK from Scrapheap.
// Ignition: purge 1 agent (can't attack this turn) OR attack twice this turn.
func ChromePaladinEnvoy() *Card {
	specialSummonEff := &CardEffect{
		Name:       "Chrome Paladin Special Summon",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectNone,
		SpecialSummonCondition: func(d *Duel, card *CardInstance, player int) bool {
			hasLight := false
			hasDark := false
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent {
					if c.Card.Attribute == AttrLIGHT {
						hasLight = true
					}
					if c.Card.Attribute == AttrDARK {
						hasDark = true
					}
				}
			}
			return hasLight && hasDark && d.State.Players[player].FreeAgentZone() != -1
		},
		Cost: func(d *Duel, card *CardInstance, player int) (bool, error) {
			gs := d.State
			p := gs.Players[player]

			var lightCandidates []*CardInstance
			for _, c := range p.Scrapheap {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrLIGHT {
					lightCandidates = append(lightCandidates, c)
				}
			}
			lightChosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Purge 1 LIGHT from Scrapheap", lightCandidates, 1, 1)
			if err != nil {
				return false, err
			}
			d.purgeFromScrapheap(player, lightChosen[0], "Chrome Paladin cost")

			var darkCandidates []*CardInstance
			for _, c := range p.Scrapheap {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrDARK {
					darkCandidates = append(darkCandidates, c)
				}
			}
			darkChosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Purge 1 DARK from Scrapheap", darkCandidates, 1, 1)
			if err != nil {
				return false, err
			}
			d.purgeFromScrapheap(player, darkChosen[0], "Chrome Paladin cost")

			return true, nil
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			d.State.Players[player].RemoveFromHand(card)
			return d.executeSpecialSummon(card, player, PositionATK, FaceUp)
		},
	}

	purgeEffect := &CardEffect{
		Name:       "Chrome Paladin Purge",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectIgnition,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			if card.Counters["bls_used"] > 0 {
				return false
			}
			for p := 0; p < 2; p++ {
				for _, m := range d.State.Players[p].Agents() {
					if m.ID != card.ID {
						return true
					}
				}
			}
			return false
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for p := 0; p < 2; p++ {
				for _, m := range d.State.Players[p].Agents() {
					if m.ID != card.ID {
						candidates = append(candidates, m)
					}
				}
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose 1 agent to purge", candidates, 1, 1)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			card.Counters["bls_used"]++
			card.AttackedThisTurn = true
			for _, t := range targets {
				if d.isOnField(t) {
					d.purgeFromField(t, "Chrome Paladin purge")
				}
			}
			return nil
		},
	}

	return &Card{
		Name:        "Chrome Paladin - Envoy of Genesis",
		Description: "Cannot be Normal Summoned/Set. Must be Special Summoned by purging 1 LIGHT and 1 DARK agent from your Scrapheap. Once per turn, you can target 1 agent on the field; purge that target. If you activate this effect, this card cannot attack this turn.",
		CardType:    CardTypeAgent,
		Level:       8,
		Attribute:   AttrLIGHT,
		AgentType:   "Enforcer",
		ATK:         3000,
		DEF:         2500,
		IsEffect:    true,
		Effects:     []*CardEffect{specialSummonEff, purgeEffect},
	}
}

// --- Phase 3: Programs ---

// HostileTakeover — Equip Program: take control of opponent's agent. Opponent gains 1000 HP each Standby.
func HostileTakeover() *Card {
	eff := &CardEffect{
		Name:      "Hostile Takeover",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			opp := d.State.Opponent(player)
			return len(d.State.Players[opp].FaceUpAgents()) > 0 &&
				d.State.Players[player].FreeAgentZone() != -1
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			opp := d.State.Opponent(player)
			candidates := d.State.Players[opp].FaceUpAgents()
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose opponent's agent to steal", candidates, 1, 1)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			if len(targets) == 0 {
				return nil
			}
			target := targets[0]
			if !d.isOnField(target) {
				return nil
			}
			if err := d.changeControl(target, player); err != nil {
				return err
			}
			d.attachEquip(card, target, 0, 0)
			return nil
		},
		OnFieldEffect: func(d *Duel, card *CardInstance, player int) {
			gs := d.State
			if gs.Phase != PhaseStandby {
				return
			}
			opp := gs.Opponent(card.Controller)
			oldHP := gs.Players[opp].HP
			gs.Players[opp].HP += 1000
			d.log(log.NewHPChangeEvent(gs.Turn, gs.Phase.String(), opp, oldHP, gs.Players[opp].HP, "Hostile Takeover"))
		},
		OnLeaveField: func(d *Duel, card *CardInstance, player int) {
			if card.EquippedTo != nil {
				target := card.EquippedTo
				if d.isOnField(target) && target.Controller != target.Owner {
					_ = d.changeControl(target, target.Owner)
				}
			}
		},
	}

	return &Card{
		Name:        "Hostile Takeover",
		Description: "Target 1 face-up agent your opponent controls; take control of that target. During each of your opponent's Standby Phases, they gain 1000 HP. When this card leaves the field, return control of the equipped agent.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramEquip,
		Effects:     []*CardEffect{eff},
	}
}

// EmergencyReboot — Equip Program: pay 800 HP, special summon 1 agent from Scrapheap.
func EmergencyReboot() *Card {
	eff := &CardEffect{
		Name:      "Emergency Reboot",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			if d.State.Players[player].HP <= 800 {
				return false
			}
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent {
					return true
				}
			}
			return false
		},
		Cost: func(d *Duel, card *CardInstance, player int) (bool, error) {
			gs := d.State
			p := gs.Players[player]
			oldHP := p.HP
			p.HP -= 800
			d.log(log.NewHPChangeEvent(gs.Turn, gs.Phase.String(), player, oldHP, p.HP, "Emergency Reboot cost"))
			return true, nil
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent {
					candidates = append(candidates, c)
				}
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose 1 agent from Scrapheap to special summon", candidates, 1, 1)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			if len(targets) == 0 {
				return nil
			}
			target := targets[0]
			inScrapheap := false
			for _, c := range d.State.Players[player].Scrapheap {
				if c.ID == target.ID {
					inScrapheap = true
					break
				}
			}
			if !inScrapheap {
				return nil
			}
			d.removeFromScrapheap(player, target)
			if err := d.executeSpecialSummon(target, player, PositionATK, FaceUp); err != nil {
				return err
			}
			d.attachEquip(card, target, 0, 0)
			return nil
		},
		OnLeaveField: func(d *Duel, card *CardInstance, player int) {
			if card.EquippedTo != nil {
				target := card.EquippedTo
				if d.isOnField(target) {
					d.destroyByEffect(target, "Emergency Reboot destroyed")
				}
			}
		},
	}

	return &Card{
		Name:        "Emergency Reboot",
		Description: "Pay 800 HP, then target 1 agent in your Scrapheap; Special Summon that target and equip it with this card. When this card leaves the field, destroy the equipped agent.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramEquip,
		Effects:     []*CardEffect{eff},
	}
}

// NeuralSiphon — Normal Program: Draw 3, discard 2.
func NeuralSiphon() *Card {
	eff := &CardEffect{
		Name:      "Neural Siphon",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			return d.State.Players[player].DeckCount() >= 3
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			p := gs.Players[player]
			for i := 0; i < 3; i++ {
				drawn := p.DrawCard()
				if drawn != nil {
					d.log(log.NewDrawEvent(gs.Turn, gs.Phase.String(), player, drawn.Card.Name))
				}
			}
			if len(p.Hand) < 2 {
				for len(p.Hand) > 0 {
					c := p.Hand[0]
					p.RemoveFromHand(c)
					p.SendToScrapheap(c)
					d.log(log.NewDiscardEvent(gs.Turn, gs.Phase.String(), player, c.Card.Name))
				}
				return nil
			}
			toDiscard, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Choose 2 cards to discard", p.Hand, 2, 2)
			if err != nil {
				return err
			}
			for _, c := range toDiscard {
				p.RemoveFromHand(c)
				p.SendToScrapheap(c)
				d.log(log.NewDiscardEvent(gs.Turn, gs.Phase.String(), player, c.Card.Name))
			}
			return nil
		},
	}

	return &Card{
		Name:        "Neural Siphon",
		Description: "Draw 3 cards, then discard 2 cards.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// MemoryCorruption — Normal Program: Pay 1000 HP, opponent discards 1 random + 1 chosen.
func MemoryCorruption() *Card {
	eff := &CardEffect{
		Name:      "Memory Corruption",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			opp := d.State.Opponent(player)
			return d.State.Players[player].HP > 1000 && len(d.State.Players[opp].Hand) > 0
		},
		Cost: func(d *Duel, card *CardInstance, player int) (bool, error) {
			gs := d.State
			p := gs.Players[player]
			oldHP := p.HP
			p.HP -= 1000
			d.log(log.NewHPChangeEvent(gs.Turn, gs.Phase.String(), player, oldHP, p.HP, "Memory Corruption cost"))
			return true, nil
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			opp := gs.Opponent(player)
			oppP := gs.Players[opp]
			if len(oppP.Hand) == 0 {
				return nil
			}
			random := oppP.Hand[0]
			oppP.RemoveFromHand(random)
			oppP.SendToScrapheap(random)
			d.log(log.NewDiscardEvent(gs.Turn, gs.Phase.String(), opp, random.Card.Name))
			if len(oppP.Hand) > 0 {
				chosen, err := d.Controllers[opp].ChooseCards(d.ctx, gs, "Choose 1 card to discard", oppP.Hand, 1, 1)
				if err != nil {
					return err
				}
				if len(chosen) > 0 {
					oppP.RemoveFromHand(chosen[0])
					oppP.SendToScrapheap(chosen[0])
					d.log(log.NewDiscardEvent(gs.Turn, gs.Phase.String(), opp, chosen[0].Card.Name))
				}
			}
			return nil
		},
	}

	return &Card{
		Name:        "Memory Corruption",
		Description: "Pay 1000 HP; your opponent discards 1 random card, then discards 1 card of their choice.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// TraceAndTerminate — Normal Program: Destroy 1 face-down agent, purge it.
func TraceAndTerminate() *Card {
	eff := &CardEffect{
		Name:      "Trace and Terminate",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			for p := 0; p < 2; p++ {
				for _, m := range d.State.Players[p].AgentZones {
					if m != nil && m.Face == FaceDown {
						return true
					}
				}
			}
			return false
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for p := 0; p < 2; p++ {
				for _, m := range d.State.Players[p].AgentZones {
					if m != nil && m.Face == FaceDown {
						candidates = append(candidates, m)
					}
				}
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose 1 face-down agent to purge", candidates, 1, 1)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			for _, t := range targets {
				if d.isOnField(t) && t.Face == FaceDown {
					d.purgeFromField(t, "Trace and Terminate")
				}
			}
			return nil
		},
	}

	return &Card{
		Name:        "Trace and Terminate",
		Description: "Target 1 face-down agent; purge that target.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// --- Phase 3: Traps ---

// ResurrectionProtocol — Continuous Trap: special summon 1 agent from Scrapheap.
// If this card leaves the field, destroy the agent. If agent is destroyed, destroy this card.
func ResurrectionProtocol() *Card {
	eff := &CardEffect{
		Name:      "Resurrection Protocol",
		ExecSpeed: ExecSpeed2,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			if d.State.Players[player].FreeAgentZone() == -1 {
				return false
			}
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent {
					return true
				}
			}
			return false
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent {
					candidates = append(candidates, c)
				}
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose 1 agent from Scrapheap to special summon", candidates, 1, 1)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			if len(targets) == 0 {
				return nil
			}
			target := targets[0]
			inScrapheap := false
			for _, c := range d.State.Players[player].Scrapheap {
				if c.ID == target.ID {
					inScrapheap = true
					break
				}
			}
			if !inScrapheap {
				return nil
			}
			d.removeFromScrapheap(player, target)
			if err := d.executeSpecialSummon(target, player, PositionATK, FaceUp); err != nil {
				return err
			}
			d.attachEquip(card, target, 0, 0)
			return nil
		},
		OnLeaveField: func(d *Duel, card *CardInstance, player int) {
			if card.EquippedTo != nil {
				target := card.EquippedTo
				if d.isOnField(target) {
					d.destroyByEffect(target, "Resurrection Protocol destroyed")
				}
			}
		},
	}

	return &Card{
		Name:        "Resurrection Protocol",
		Description: "Target 1 agent in your Scrapheap; Special Summon it. When this card leaves the field, destroy that agent. When that agent is destroyed, destroy this card.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapContinuous,
		Effects:     []*CardEffect{eff},
	}
}

// StaticDischarge — SS2 Normal Trap: destroy 1 Tech. Then optionally set 1 Tech from hand.
func StaticDischarge() *Card {
	eff := &CardEffect{
		Name:      "Static Discharge",
		ExecSpeed: ExecSpeed2,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			for p := 0; p < 2; p++ {
				for _, st := range d.State.Players[p].TechCards() {
					if st.ID != card.ID {
						return true
					}
				}
			}
			return false
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for p := 0; p < 2; p++ {
				for _, st := range d.State.Players[p].TechCards() {
					if st.ID != card.ID {
						candidates = append(candidates, st)
					}
				}
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose 1 Tech to destroy", candidates, 1, 1)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			for _, t := range targets {
				if d.isOnField(t) {
					d.destroyByEffect(t, "Static Discharge")
				}
			}
			p := gs.Players[player]
			var settable []*CardInstance
			for _, c := range p.Hand {
				if c.Card.CardType == CardTypeProgram || c.Card.CardType == CardTypeTrap {
					settable = append(settable, c)
				}
			}
			if len(settable) > 0 && p.FreeTechZone() != -1 {
				yes, err := d.Controllers[player].ChooseYesNo(d.ctx, gs, "Set a Tech from hand?")
				if err != nil {
					return err
				}
				if yes {
					chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Choose 1 Tech to set", settable, 1, 1)
					if err != nil {
						return err
					}
					if len(chosen) > 0 {
						zone := p.FreeTechZone()
						p.RemoveFromHand(chosen[0])
						chosen[0].Face = FaceDown
						chosen[0].TurnPlaced = gs.Turn
						chosen[0].Controller = player
						p.PlaceTech(chosen[0], zone)
						d.log(log.NewSetTechEvent(gs.Turn, gs.Phase.String(), player, zone))
					}
				}
			}
			return nil
		},
	}

	return &Card{
		Name:        "Static Discharge",
		Description: "Target 1 Program/Trap on the field; destroy that target. Then you can Set 1 Program/Trap from your hand.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapNormal,
		Effects:     []*CardEffect{eff},
	}
}

// DecoyHolograms — Quick-Play Program: special summon 4 Sheep Tokens (0/0). Cannot summon this turn.
func DecoyHolograms() *Card {
	eff := &CardEffect{
		Name:      "Decoy Holograms",
		ExecSpeed: ExecSpeed2,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			return d.State.Players[player].FreeAgentZone() != -1
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			p := gs.Players[player]
			for i := 0; i < 4; i++ {
				zone := p.FreeAgentZone()
				if zone == -1 {
					break
				}
				token := gs.CreateCardInstance(&Card{
					Name:      "Holo-Decoy Token",
					CardType:  CardTypeAgent,
					Level:     1,
					Attribute: AttrEARTH,
					AgentType: "Bioweapon",
					ATK:       0,
					DEF:       0,
				}, player)
				token.Face = FaceUp
				token.Position = PositionDEF
				token.TurnPlaced = gs.Turn
				token.Controller = player
				p.PlaceAgent(token, zone)
				d.log(log.NewSpecialSummonEvent(gs.Turn, gs.Phase.String(), player, "Holo-Decoy Token", 0, zone))
			}
			gs.NormalSummonUsed = true
			return nil
		},
	}

	return &Card{
		Name:        "Decoy Holograms",
		Description: "Special Summon 4 Holo-Decoy Tokens (Bioweapon/EARTH/Level 1/ATK 0/DEF 0) in DEF Position. You cannot Summon other agents the turn you activate this card.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramQuickPlay,
		Effects:     []*CardEffect{eff},
	}
}

// ======== Phase 4: New card implementations ========

// --- Vanilla Agents ---

func PrismaticDatafish() *Card {
	return &Card{
		Name:        "Prismatic Datafish",
		Description: "A shimmering construct of liquid light, darting through datastreams. Its scales refract neon in hypnotic patterns, luring unwary hackers into the abyss.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrWATER,
		AgentType:   "Abyssal",
		ATK:         1800,
		DEF:         800,
	}
}

func BlazingAutomaton() *Card {
	return &Card{
		Name:        "Blazing Automaton",
		Description: "A walking inferno wrapped in molten chrome, built for one purpose: to burn through defenses. No armor, no mercy—just raw, incinerating fury.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrFIRE,
		AgentType:   "Burner",
		ATK:         1850,
		DEF:         0,
	}
}

func SteelJuggernaut() *Card {
	return &Card{
		Name:        "Steel Juggernaut",
		Description: "A hulking mech-beast forged from scorched steel and alloy plating. Once a failed corporate war machine, now a rogue juggernaut prowling the undercity ruins.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrFIRE,
		AgentType:   "Bioweapon",
		ATK:         1800,
		DEF:         600,
	}
}

func AbyssalNetrunner() *Card {
	return &Card{
		Name:        "Abyssal Netrunner",
		Description: "A shadow in the deep net, this leviathan slips through firewalls like oil through cracks. Those who meet its gaze in the void are never seen online again.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrWATER,
		AgentType:   "Leviathan",
		ATK:         1800,
		DEF:         1500,
	}
}

func VoidDrifter() *Card {
	return &Card{
		Name:        "Void Drifter",
		Description: "A nomadic hunter drifting through deleted sectors. It feeds on corrupted data, growing stronger in places where reality itself has decayed.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrWATER,
		AgentType:   "Abyssal",
		ATK:         1700,
		DEF:         1000,
	}
}

// --- Simple Programs ---

// HeadshotRoutine — Normal Program. Destroy the face-up ATK agent with the highest ATK.
func HeadshotRoutine() *Card {
	eff := &CardEffect{
		Name:      "Headshot Routine",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			for p := 0; p < 2; p++ {
				if len(d.State.Players[p].FaceUpATKAgents()) > 0 {
					return true
				}
			}
			return false
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			// Find highest ATK among face-up ATK position agents
			var highest []*CardInstance
			maxATK := -1
			for p := 0; p < 2; p++ {
				for _, m := range d.State.Players[p].FaceUpATKAgents() {
					atk := m.CurrentATK()
					if atk > maxATK {
						maxATK = atk
						highest = []*CardInstance{m}
					} else if atk == maxATK {
						highest = append(highest, m)
					}
				}
			}
			if len(highest) == 0 {
				return nil
			}
			var toDestroy *CardInstance
			if len(highest) == 1 {
				toDestroy = highest[0]
			} else {
				chosen, err := d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose which to destroy (tied ATK)", highest, 1, 1)
				if err != nil {
					return err
				}
				toDestroy = chosen[0]
			}
			if d.isOnField(toDestroy) {
				d.destroyByEffect(toDestroy, "Headshot Routine")
			}
			return nil
		},
	}
	return &Card{
		Name:        "Headshot Routine",
		Description: "Destroy the face-up ATK Position agent with the highest ATK on the field.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// OrbitalPayload — Normal Program. 1000 damage if opponent HP > 3000.
func OrbitalPayload() *Card {
	eff := &CardEffect{
		Name:      "Orbital Payload",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			opp := d.State.Opponent(player)
			return d.State.Players[opp].HP > 3000
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			opp := d.State.Opponent(player)
			d.applyEffectDamage(opp, 1000, "Orbital Payload")
			return nil
		},
	}
	return &Card{
		Name:        "Orbital Payload",
		Description: "If your opponent has more than 3000 HP: Inflict 1000 damage to your opponent.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// FlatlineCommand — Normal Program. Discard 1 card; destroy 1 agent.
func FlatlineCommand() *Card {
	eff := &CardEffect{
		Name:      "Flatline Command",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			if len(d.State.Players[player].Hand) <= 1 { // need 1 card to discard besides this program
				return false
			}
			for p := 0; p < 2; p++ {
				if d.State.Players[p].AgentCount() > 0 {
					return true
				}
			}
			return false
		},
		Cost: func(d *Duel, card *CardInstance, player int) (bool, error) {
			gs := d.State
			p := gs.Players[player]
			var discardable []*CardInstance
			for _, c := range p.Hand {
				if c.ID != card.ID {
					discardable = append(discardable, c)
				}
			}
			if len(discardable) == 0 {
				return false, nil
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Discard 1 card", discardable, 1, 1)
			if err != nil {
				return false, err
			}
			p.RemoveFromHand(chosen[0])
			p.SendToScrapheap(chosen[0])
			d.log(log.NewDiscardEvent(gs.Turn, gs.Phase.String(), player, chosen[0].Card.Name))
			return true, nil
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for p := 0; p < 2; p++ {
				candidates = append(candidates, d.State.Players[p].Agents()...)
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose 1 agent to destroy", candidates, 1, 1)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			for _, t := range targets {
				if d.isOnField(t) {
					d.destroyByEffect(t, "Flatline Command")
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Flatline Command",
		Description: "Discard 1 card, then target 1 agent on the field; destroy that target.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// ScrapheapRecovery — Normal Program. Add 2 WATER agents with 1500 or less ATK from Scrapheap to hand.
func ScrapheapRecovery() *Card {
	eff := &CardEffect{
		Name:      "Scrapheap Recovery",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			count := 0
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrWATER && c.Card.ATK <= 1500 {
					count++
				}
			}
			return count >= 2
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrWATER && c.Card.ATK <= 1500 {
					candidates = append(candidates, c)
				}
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose 2 WATER agents to add to hand", candidates, 2, 2)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			for _, t := range targets {
				inScrapheap := false
				for _, c := range gs.Players[player].Scrapheap {
					if c.ID == t.ID {
						inScrapheap = true
						break
					}
				}
				if inScrapheap {
					d.removeFromScrapheap(player, t)
					t.Zone = ZoneHand
					gs.Players[player].Hand = append(gs.Players[player].Hand, t)
					d.log(log.NewAddToHandEvent(gs.Turn, gs.Phase.String(), player, t.Card.Name, "Scrapheap Recovery"))
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Scrapheap Recovery",
		Description: "Target 2 WATER agents with 1500 or less ATK in your Scrapheap; add those targets to your hand.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// CoreDump — Quick-Play Program. Shuffle hand into deck, draw same number.
func CoreDump() *Card {
	eff := &CardEffect{
		Name:      "Core Dump",
		ExecSpeed: ExecSpeed2,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			count := 0
			for _, c := range d.State.Players[player].Hand {
				if c.ID != card.ID {
					count++
				}
			}
			return count > 0
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			p := gs.Players[player]
			count := len(p.Hand)
			// Return all hand cards to deck
			for len(p.Hand) > 0 {
				c := p.Hand[0]
				p.RemoveFromHand(c)
				c.Zone = ZoneDeck
				p.Deck = append(p.Deck, c)
			}
			p.ShuffleDeck()
			d.log(log.NewShuffleEvent(gs.Turn, gs.Phase.String(), player))
			// Draw same number
			for i := 0; i < count; i++ {
				drawn := p.DrawCard()
				if drawn != nil {
					d.log(log.NewDrawEvent(gs.Turn, gs.Phase.String(), player, drawn.Card.Name))
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Core Dump",
		Description: "Shuffle your hand into the Deck, then draw the same number of cards.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramQuickPlay,
		Effects:     []*CardEffect{eff},
	}
}

// SurgeOverride — Normal Program. Destroy all WATER you control, SS WATER from hand.
func SurgeOverride() *Card {
	eff := &CardEffect{
		Name:      "Surge Override",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			hasWATERAgent := false
			for _, m := range d.State.Players[player].FaceUpAgents() {
				if m.Card.Attribute == AttrWATER {
					hasWATERAgent = true
					break
				}
			}
			if !hasWATERAgent {
				return false
			}
			for _, c := range d.State.Players[player].Hand {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrWATER && c.ID != card.ID {
					return true
				}
			}
			return false
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			p := gs.Players[player]
			// Destroy all face-up WATER agents you control
			count := 0
			for _, m := range p.FaceUpAgents() {
				if m.Card.Attribute == AttrWATER {
					d.destroyByEffect(m, "Surge Override")
					count++
				}
			}
			// SS WATER from hand up to that number
			for i := 0; i < count; i++ {
				if p.FreeAgentZone() == -1 {
					break
				}
				var waterInHand []*CardInstance
				for _, c := range p.Hand {
					if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrWATER {
						waterInHand = append(waterInHand, c)
					}
				}
				if len(waterInHand) == 0 {
					break
				}
				chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Choose a WATER agent to Special Summon", waterInHand, 1, 1)
				if err != nil {
					return err
				}
				p.RemoveFromHand(chosen[0])
				if err := d.executeSpecialSummon(chosen[0], player, PositionATK, FaceUp); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Surge Override",
		Description: "Destroy all face-up WATER agents you control, then Special Summon WATER agents from your hand, up to the number of agents destroyed.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// IdentityHijack — Normal Program. Each player chooses 1 agent, switch control.
func IdentityHijack() *Card {
	eff := &CardEffect{
		Name:      "Identity Hijack",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			opp := d.State.Opponent(player)
			return d.State.Players[player].AgentCount() > 0 &&
				d.State.Players[opp].AgentCount() > 0 &&
				d.State.Players[player].FreeAgentZone() != -1 &&
				d.State.Players[opp].FreeAgentZone() != -1
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			opp := gs.Opponent(player)
			// Each player chooses 1 agent they control
			myAgents := gs.Players[player].Agents()
			if len(myAgents) == 0 {
				return nil
			}
			myChosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Choose 1 of your agents to give", myAgents, 1, 1)
			if err != nil {
				return err
			}
			oppAgents := gs.Players[opp].Agents()
			if len(oppAgents) == 0 {
				return nil
			}
			oppChosen, err := d.Controllers[opp].ChooseCards(d.ctx, gs, "Choose 1 of your agents to give", oppAgents, 1, 1)
			if err != nil {
				return err
			}
			// Switch control
			if err := d.changeControl(myChosen[0], opp); err != nil {
				return err
			}
			if err := d.changeControl(oppChosen[0], player); err != nil {
				return err
			}
			// Can't change position this turn
			myChosen[0].PositionChangedThisTurn = true
			oppChosen[0].PositionChangedThisTurn = true
			return nil
		},
	}
	return &Card{
		Name:        "Identity Hijack",
		Description: "Each player chooses 1 agent they control and switches control of those agents with each other. Those agents cannot change their battle position this turn.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramNormal,
		Effects:     []*CardEffect{eff},
	}
}

// --- Simple Trap ---

// CacheSiphon — Normal Trap. Draw 1 card.
func CacheSiphon() *Card {
	eff := &CardEffect{
		Name:      "Cache Siphon",
		ExecSpeed: ExecSpeed2,
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			drawn := gs.Players[player].DrawCard()
			if drawn != nil {
				d.log(log.NewDrawEvent(gs.Turn, gs.Phase.String(), player, drawn.Card.Name))
			}
			return nil
		},
	}
	return &Card{
		Name:        "Cache Siphon",
		Description: "Draw 1 card.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapNormal,
		Effects:     []*CardEffect{eff},
	}
}

// --- Operating Systems ---

// ReactorMeltdown — Operating System. FIRE +500 ATK, -400 DEF.
func ReactorMeltdown() *Card {
	eff := &CardEffect{
		Name:       "Reactor Meltdown",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectContinuous,
		ContinuousApply: func(d *Duel, card *CardInstance, player int) {
			gs := d.State
			for p := 0; p < 2; p++ {
				for _, m := range gs.Players[p].FaceUpAgents() {
					if m.Card.Attribute == AttrFIRE {
						m.AddModifier(StatModifier{
							Source:     card.ID,
							ATKMod:     500,
							DEFMod:     -400,
							Continuous: true,
						})
					}
				}
			}
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			d.recalculateContinuousEffects()
			return nil
		},
	}
	return &Card{
		Name:        "Reactor Meltdown",
		Description: "All FIRE agents on the field gain 500 ATK and lose 400 DEF.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramOS,
		Effects:     []*CardEffect{eff},
	}
}

// TheUndercityGrid — Operating System. Treated as "NetGrid". WATER +200 ATK/DEF. WATER level -1.
func TheUndercityGrid() *Card {
	eff := &CardEffect{
		Name:       "The Undercity Grid",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectContinuous,
		ContinuousApply: func(d *Duel, card *CardInstance, player int) {
			gs := d.State
			for p := 0; p < 2; p++ {
				for _, m := range gs.Players[p].FaceUpAgents() {
					if m.Card.Attribute == AttrWATER {
						m.AddModifier(StatModifier{
							Source:     card.ID,
							ATKMod:     200,
							DEFMod:     200,
							Continuous: true,
						})
					}
				}
			}
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			d.recalculateContinuousEffects()
			return nil
		},
	}
	return &Card{
		Name:        "The Undercity Grid",
		Description: "This card's name is also treated as \"NetGrid\". All WATER agents on the field gain 200 ATK and DEF. All WATER agents in your hand and on the field have their Level reduced by 1.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramOS,
		Effects:     []*CardEffect{eff},
	}
}

// --- Continuous Programs ---

// TortureSubnet — Continuous Program. When opponent takes effect damage, 300 more.
func TortureSubnet() *Card {
	eff := &CardEffect{
		Name:       "Torture Subnet",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectContinuous,
		OnFieldEffect: func(d *Duel, card *CardInstance, player int) {
			// This is called by applyEffectDamage when opponent takes effect damage
			opp := d.State.Opponent(player)
			d.applyDamage(opp, 300, "Torture Subnet")
		},
	}
	return &Card{
		Name:        "Torture Subnet",
		Description: "Each time your opponent takes effect damage, they take 300 additional damage.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramContinuous,
		Effects:     []*CardEffect{eff},
	}
}

// SectorLockdownZoneB — Continuous Program. All face-up L4+ agents to DEF.
func SectorLockdownZoneB() *Card {
	eff := &CardEffect{
		Name:       "Sector Lockdown - Zone B",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectContinuous,
		ContinuousApply: func(d *Duel, card *CardInstance, player int) {
			gs := d.State
			for p := 0; p < 2; p++ {
				for _, m := range gs.Players[p].FaceUpAgents() {
					if m.Card.Level >= 4 && m.Position == PositionATK {
						m.Position = PositionDEF
					}
				}
			}
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			d.recalculateContinuousEffects()
			return nil
		},
	}
	return &Card{
		Name:        "Sector Lockdown - Zone B",
		Description: "All face-up Level 4 or higher agents on the field are changed to DEF Position.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramContinuous,
		Effects:     []*CardEffect{eff},
	}
}

// --- Equip Program ---

// NeuralShackle — Equip. When equipped agent destroyed by battle, draw 1 or discard opponent's hand.
func NeuralShackle() *Card {
	eff := &CardEffect{
		Name:      "Neural Shackle",
		ExecSpeed: ExecSpeed1,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			for p := 0; p < 2; p++ {
				if len(d.State.Players[p].FaceUpAgents()) > 0 {
					return true
				}
			}
			return false
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for p := 0; p < 2; p++ {
				candidates = append(candidates, d.State.Players[p].FaceUpAgents()...)
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose agent to equip", candidates, 1, 1)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			if len(targets) == 0 || !d.isOnField(targets[0]) {
				return nil
			}
			d.attachEquip(card, targets[0], 0, 0)
			return nil
		},
		OnLeaveField: func(d *Duel, card *CardInstance, player int) {
			// Neural Shackle leaves field when equipped agent is destroyed
			// The draw/discard effect is handled via OnBattleDestruction on the equipped agent
			// Actually, Neural Shackle triggers when sent to Scrapheap due to equipped agent battle destruction
			// For simplicity, we handle this as a trigger on Neural Shackle leaving the field
			if card.EquippedTo != nil {
				gs := d.State
				opp := gs.Opponent(player)
				// Choose: draw 1 or discard random from opponent
				yes, _ := d.Controllers[player].ChooseYesNo(d.ctx, gs, "Neural Shackle: Draw 1? (No = discard from opponent)")
				if yes {
					drawn := gs.Players[player].DrawCard()
					if drawn != nil {
						d.log(log.NewDrawEvent(gs.Turn, gs.Phase.String(), player, drawn.Card.Name))
					}
				} else if len(gs.Players[opp].Hand) > 0 {
					// Random discard
					c := gs.Players[opp].Hand[0]
					gs.Players[opp].RemoveFromHand(c)
					gs.Players[opp].SendToScrapheap(c)
					d.log(log.NewDiscardEvent(gs.Turn, gs.Phase.String(), opp, c.Card.Name))
				}
			}
		},
	}
	return &Card{
		Name:        "Neural Shackle",
		Description: "When the equipped agent is destroyed by battle: You can either draw 1 card, or discard 1 random card from your opponent's hand.",
		CardType:    CardTypeProgram,
		ProgramSub:  ProgramEquip,
		Effects:     []*CardEffect{eff},
	}
}

// --- Counter Trap ---

// FirewallSentinel — Counter Trap. Negate a program activation.
func FirewallSentinel() *Card {
	eff := &CardEffect{
		Name:      "Firewall Sentinel",
		ExecSpeed: ExecSpeed3,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			gs := d.State
			if gs.Chain == nil || len(gs.Chain.Links) == 0 {
				return false
			}
			topLink := gs.Chain.Links[len(gs.Chain.Links)-1]
			return topLink.Card.Card.CardType == CardTypeProgram
		},
		Cost: func(d *Duel, card *CardInstance, player int) (bool, error) {
			// Option 1: targeting program negate (free)
			// Option 2: discard a program to negate any program
			// For simplicity, always use discard cost if needed
			gs := d.State
			topLink := gs.Chain.Links[len(gs.Chain.Links)-1]
			// If it targets exactly 1 agent, free negate
			if topLink.Targets != nil && len(topLink.Targets) == 1 {
				for _, t := range topLink.Targets {
					if t.Card.CardType == CardTypeAgent {
						return true, nil // free negate
					}
				}
			}
			// Otherwise need to discard a program
			p := gs.Players[player]
			var programs []*CardInstance
			for _, c := range p.Hand {
				if c.Card.CardType == CardTypeProgram && c.ID != card.ID {
					programs = append(programs, c)
				}
			}
			if len(programs) == 0 {
				return false, nil
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Discard 1 Program to negate", programs, 1, 1)
			if err != nil {
				return false, err
			}
			p.RemoveFromHand(chosen[0])
			p.SendToScrapheap(chosen[0])
			d.log(log.NewDiscardEvent(gs.Turn, gs.Phase.String(), player, chosen[0].Card.Name))
			return true, nil
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			if gs.Chain != nil && len(gs.Chain.Links) > 0 {
				myIndex := -1
				for i, link := range gs.Chain.Links {
					if link.Card.ID == card.ID {
						myIndex = i
						break
					}
				}
				if myIndex > 0 {
					negated := gs.Chain.Links[myIndex-1]
					if d.isOnField(negated.Card) {
						d.destroyByEffect(negated.Card, "negated by Firewall Sentinel")
					}
					gs.Chain.Links[myIndex-1].Effect = &CardEffect{
						Name: "Negated",
						Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
							return nil
						},
					}
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Firewall Sentinel",
		Description: "When a Program is activated: If it targets exactly 1 agent, negate the activation and destroy it. Otherwise, discard 1 Program to negate the activation and destroy it.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapCounter,
		Effects:     []*CardEffect{eff},
	}
}

// --- Continuous Traps ---

// CounterHack — Continuous Trap. When a FIRE you control is destroyed, 500 damage to opponent.
func CounterHack() *Card {
	eff := &CardEffect{
		Name:         "Counter-Hack",
		ExecSpeed:    ExecSpeed2,
		EffectType:   EffectTrigger,
		IsTrigger:    true,
		IsMandatory:  true,
		TriggerEvent: log.EventDestroy,
		TriggerFilter: func(d *Duel, card *CardInstance, event log.GameEvent) bool {
			return event.Type == log.EventDestroy || event.Type == log.EventBattleDestroy
		},
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			// Simplified: check recent events for FIRE agent destruction
			// This is handled via OnFieldEffect approach instead
			return false
		},
		OnFieldEffect: func(d *Duel, card *CardInstance, player int) {
			// Called after a FIRE agent owned by player is destroyed
			// Actual trigger handled inline by the engine
		},
	}
	return &Card{
		Name:        "Counter-Hack",
		Description: "Each time a FIRE agent you control is destroyed: Inflict 500 damage to your opponent.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapContinuous,
		Effects:     []*CardEffect{eff},
	}
}

// GravityClamp — Continuous Trap. Level 4+ agents cannot attack.
func GravityClamp() *Card {
	eff := &CardEffect{
		Name:       "Gravity Clamp",
		ExecSpeed:  ExecSpeed2,
		EffectType: EffectContinuous,
		AttackRestriction: func(d *Duel, attacker *CardInstance) bool {
			return attacker.Card.Level < 4
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			return nil // just stays face-up
		},
	}
	return &Card{
		Name:        "Gravity Clamp",
		Description: "Level 4 or higher agents cannot declare an attack.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapContinuous,
		Effects:     []*CardEffect{eff},
	}
}

// SurgeBarrier — Continuous Trap. While Umi on field, no battle damage.
func SurgeBarrier() *Card {
	eff := &CardEffect{
		Name:       "Surge Barrier",
		ExecSpeed:  ExecSpeed2,
		EffectType: EffectContinuous,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			return d.isNetGridOnField()
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			return nil // stays face-up
		},
		// Surge Barrier is destroyed when NetGrid leaves the field
		// We check this in recalculateContinuousEffects
		OnFieldEffect: func(d *Duel, card *CardInstance, player int) {
			if !d.isNetGridOnField() && card.Face == FaceUp {
				d.destroyByEffect(card, "NetGrid left field")
			}
		},
	}
	return &Card{
		Name:        "Surge Barrier",
		Description: "Activate only while \"NetGrid\" is on the field. Players take no battle damage. This card is destroyed when \"NetGrid\" leaves the field.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapContinuous,
		Effects:     []*CardEffect{eff},
	}
}

// DeadlockSeal — Continuous Trap. Select 2 set Tech; they can't be activated.
func DeadlockSeal() *Card {
	eff := &CardEffect{
		Name:      "Deadlock Seal",
		ExecSpeed: ExecSpeed2,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			count := 0
			for p := 0; p < 2; p++ {
				for _, st := range d.State.Players[p].FaceDownTech() {
					if st.ID != card.ID {
						count++
					}
				}
			}
			return count >= 2
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for p := 0; p < 2; p++ {
				for _, st := range d.State.Players[p].FaceDownTech() {
					if st.ID != card.ID {
						candidates = append(candidates, st)
					}
				}
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose 2 Set Tech to lock", candidates, 2, 2)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			// Mark targets as locked by storing their IDs on this card's counters
			for i, t := range targets {
				card.Counters[fmt.Sprintf("locked_%d", i)] = t.ID
			}
			return nil
		},
	}
	return &Card{
		Name:        "Deadlock Seal",
		Description: "Target 2 Set Programs/Traps on the field; as long as this card remains face-up on the field, the targeted cards cannot be activated.",
		CardType:    CardTypeTrap,
		TrapSub:     TrapContinuous,
		Effects:     []*CardEffect{eff},
	}
}

// --- Effect Agents: Continuous Stat Boosters ---

// SignalAmplifier — WATER +500 ATK, FIRE -400 ATK.
func SignalAmplifier() *Card {
	eff := &CardEffect{
		Name:       "Signal Amplifier Aura",
		EffectType: EffectContinuous,
		ContinuousApply: func(d *Duel, card *CardInstance, player int) {
			gs := d.State
			for p := 0; p < 2; p++ {
				for _, m := range gs.Players[p].FaceUpAgents() {
					if m.ID == card.ID {
						continue
					}
					if m.Card.Attribute == AttrWATER {
						m.AddModifier(StatModifier{Source: card.ID, ATKMod: 500, Continuous: true})
					}
					if m.Card.Attribute == AttrFIRE {
						m.AddModifier(StatModifier{Source: card.ID, ATKMod: -400, Continuous: true})
					}
				}
			}
		},
	}
	return &Card{
		Name:        "Signal Amplifier",
		Description: "All WATER agents on the field gain 500 ATK. All FIRE agents on the field lose 400 ATK.",
		CardType:    CardTypeAgent,
		Level:       2,
		Attribute:   AttrWATER,
		AgentType:   "Wetware",
		ATK:         550,
		DEF:         500,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// MicroChimera — FIRE +500 ATK, WATER -400 ATK.
func MicroChimera() *Card {
	eff := &CardEffect{
		Name:       "Micro Chimera Aura",
		EffectType: EffectContinuous,
		ContinuousApply: func(d *Duel, card *CardInstance, player int) {
			gs := d.State
			for p := 0; p < 2; p++ {
				for _, m := range gs.Players[p].FaceUpAgents() {
					if m.ID == card.ID {
						continue
					}
					if m.Card.Attribute == AttrFIRE {
						m.AddModifier(StatModifier{Source: card.ID, ATKMod: 500, Continuous: true})
					}
					if m.Card.Attribute == AttrWATER {
						m.AddModifier(StatModifier{Source: card.ID, ATKMod: -400, Continuous: true})
					}
				}
			}
		},
	}
	return &Card{
		Name:        "Micro Chimera",
		Description: "All FIRE agents on the field gain 500 ATK. All WATER agents on the field lose 400 ATK.",
		CardType:    CardTypeAgent,
		Level:       2,
		Attribute:   AttrFIRE,
		AgentType:   "Bioweapon",
		ATK:         600,
		DEF:         550,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// --- Effect Agents: Battle Destruction Recruiters ---

// DenMotherUnit — When destroyed by battle, SS a WATER agent with ≤1500 ATK from Deck.
func DenMotherUnit() *Card {
	eff := &CardEffect{
		Name:       "Den Mother Unit",
		EffectType: EffectTrigger,
		OnBattleDestruction: func(d *Duel, card *CardInstance, player int) {
			gs := d.State
			p := gs.Players[player]
			if p.FreeAgentZone() == -1 {
				return
			}
			var candidates []*CardInstance
			for _, c := range p.Deck {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrWATER && c.Card.ATK <= 1500 {
					candidates = append(candidates, c)
				}
			}
			if len(candidates) == 0 {
				return
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Choose a WATER agent (ATK≤1500) to Special Summon", candidates, 1, 1)
			if err != nil || len(chosen) == 0 {
				return
			}
			// Remove from deck
			for i, c := range p.Deck {
				if c.ID == chosen[0].ID {
					p.Deck = append(p.Deck[:i], p.Deck[i+1:]...)
					break
				}
			}
			_ = d.executeSpecialSummon(chosen[0], player, PositionATK, FaceUp)
			p.ShuffleDeck()
			d.log(log.NewShuffleEvent(gs.Turn, gs.Phase.String(), player))
		},
	}
	return &Card{
		Name:        "Den Mother Unit",
		Description: "When this card is destroyed by battle: You can Special Summon 1 WATER agent with 1500 or less ATK from your Deck, then shuffle your Deck.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrWATER,
		AgentType:   "Cyborg",
		ATK:         1400,
		DEF:         1000,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// DroneCarrier — When destroyed by battle, SS a FIRE agent with ≤1500 ATK from Deck.
func DroneCarrier() *Card {
	eff := &CardEffect{
		Name:       "Drone Carrier",
		EffectType: EffectTrigger,
		OnBattleDestruction: func(d *Duel, card *CardInstance, player int) {
			gs := d.State
			p := gs.Players[player]
			if p.FreeAgentZone() == -1 {
				return
			}
			var candidates []*CardInstance
			for _, c := range p.Deck {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrFIRE && c.Card.ATK <= 1500 {
					candidates = append(candidates, c)
				}
			}
			if len(candidates) == 0 {
				return
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Choose a FIRE agent (ATK≤1500) to Special Summon", candidates, 1, 1)
			if err != nil || len(chosen) == 0 {
				return
			}
			for i, c := range p.Deck {
				if c.ID == chosen[0].ID {
					p.Deck = append(p.Deck[:i], p.Deck[i+1:]...)
					break
				}
			}
			_ = d.executeSpecialSummon(chosen[0], player, PositionATK, FaceUp)
			p.ShuffleDeck()
			d.log(log.NewShuffleEvent(gs.Turn, gs.Phase.String(), player))
		},
	}
	return &Card{
		Name:        "Drone Carrier",
		Description: "When this card is destroyed by battle: You can Special Summon 1 FIRE agent with 1500 or less ATK from your Deck, then shuffle your Deck.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrFIRE,
		AgentType:   "Machine",
		ATK:         1400,
		DEF:         1200,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// --- Effect Agents: Sacrifice Summon Triggers ---

// FrostbiteTyrant — On sacrifice summon: destroy up to 2 Tech.
func FrostbiteTyrant() *Card {
	eff := &CardEffect{
		Name:         "Mobius",
		ExecSpeed:    ExecSpeed1,
		EffectType:   EffectTrigger,
		IsTrigger:    true,
		IsMandatory:  false,
		TriggerEvent: log.EventSacrificeSummon,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			if d.State.LastSummonEvent == nil || d.State.LastSummonEvent.Card.ID != card.ID {
				return false
			}
			for p := 0; p < 2; p++ {
				if len(d.State.Players[p].TechCards()) > 0 {
					return true
				}
			}
			return false
		},
		Target: func(d *Duel, card *CardInstance, player int) ([]*CardInstance, error) {
			var candidates []*CardInstance
			for p := 0; p < 2; p++ {
				candidates = append(candidates, d.State.Players[p].TechCards()...)
			}
			max := 2
			if len(candidates) < max {
				max = len(candidates)
			}
			return d.Controllers[player].ChooseCards(d.ctx, d.State, "Choose up to 2 Tech to destroy", candidates, 1, max)
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			for _, t := range targets {
				if d.isOnField(t) {
					d.destroyByEffect(t, "Frostbite Tyrant")
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Frostbite Tyrant",
		Description: "When this card is Sacrifice Summoned: You can target up to 2 Programs/Traps on the field; destroy those targets.",
		CardType:    CardTypeAgent,
		Level:       6,
		Attribute:   AttrWATER,
		AgentType:   "Wetware",
		ATK:         2400,
		DEF:         1000,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// PlasmaArcTyrant — On sacrifice summon: discard 1 random from opponent's hand.
func PlasmaArcTyrant() *Card {
	eff := &CardEffect{
		Name:         "Thestalos",
		ExecSpeed:    ExecSpeed1,
		EffectType:   EffectTrigger,
		IsTrigger:    true,
		IsMandatory:  true,
		TriggerEvent: log.EventSacrificeSummon,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			if d.State.LastSummonEvent == nil || d.State.LastSummonEvent.Card.ID != card.ID {
				return false
			}
			opp := d.State.Opponent(player)
			return len(d.State.Players[opp].Hand) > 0
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			opp := gs.Opponent(player)
			oppP := gs.Players[opp]
			if len(oppP.Hand) == 0 {
				return nil
			}
			// Random discard (first card for determinism in tests)
			c := oppP.Hand[0]
			oppP.RemoveFromHand(c)
			oppP.SendToScrapheap(c)
			d.log(log.NewDiscardEvent(gs.Turn, gs.Phase.String(), opp, c.Card.Name))
			// If agent, deal level*100 damage
			if c.Card.CardType == CardTypeAgent {
				dmg := c.Card.Level * 100
				d.applyEffectDamage(opp, dmg, fmt.Sprintf("Thestalos (%s Lv%d)", c.Card.Name, c.Card.Level))
			}
			return nil
		},
	}
	return &Card{
		Name:        "Plasma Arc Tyrant",
		Description: "When this card is Sacrifice Summoned: Discard 1 random card from your opponent's hand. If the discarded card was an agent, inflict damage to your opponent equal to its Level × 100.",
		CardType:    CardTypeAgent,
		Level:       6,
		Attribute:   AttrFIRE,
		AgentType:   "Burner",
		ATK:         2400,
		DEF:         1000,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// --- Effect Agents: Special Summon from Hand ---

// ThermalSpike — SS by purgeing 1 FIRE from Scrapheap. 1500 damage when destroys agent by battle.
func ThermalSpike() *Card {
	ssEff := &CardEffect{
		Name:       "ThermalSpike Special Summon",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectNone,
		SpecialSummonCondition: func(d *Duel, card *CardInstance, player int) bool {
			if d.State.Players[player].FreeAgentZone() == -1 {
				return false
			}
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrFIRE {
					return true
				}
			}
			return false
		},
		Cost: func(d *Duel, card *CardInstance, player int) (bool, error) {
			gs := d.State
			var candidates []*CardInstance
			for _, c := range gs.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrFIRE {
					candidates = append(candidates, c)
				}
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Purge 1 FIRE from Scrapheap", candidates, 1, 1)
			if err != nil {
				return false, err
			}
			d.purgeFromScrapheap(player, chosen[0], "ThermalSpike cost")
			return true, nil
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			d.State.Players[player].RemoveFromHand(card)
			return d.executeSpecialSummon(card, player, PositionATK, FaceUp)
		},
	}
	battleEff := &CardEffect{
		Name:       "ThermalSpike Burn",
		EffectType: EffectTrigger,
		OnDestroyByBattle: func(d *Duel, card *CardInstance, player int) {
			opp := d.State.Opponent(player)
			d.applyEffectDamage(opp, 1500, "Thermal Spike")
		},
	}
	return &Card{
		Name:        "Thermal Spike",
		Description: "You can Special Summon this card from your hand by purging 1 FIRE agent from your Scrapheap. When this card destroys an agent by battle: Inflict 1500 damage to your opponent.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrFIRE,
		AgentType:   "Burner",
		ATK:         1100,
		DEF:         1900,
		IsEffect:    true,
		Effects:     []*CardEffect{ssEff, battleEff},
	}
}

// FenrirMkII — SS by purgeing 2 WATER from Scrapheap.
func FenrirMkII() *Card {
	ssEff := &CardEffect{
		Name:       "FenrirMkII Special Summon",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectNone,
		SpecialSummonCondition: func(d *Duel, card *CardInstance, player int) bool {
			if d.State.Players[player].FreeAgentZone() == -1 {
				return false
			}
			count := 0
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrWATER {
					count++
				}
			}
			return count >= 2
		},
		Cost: func(d *Duel, card *CardInstance, player int) (bool, error) {
			gs := d.State
			var candidates []*CardInstance
			for _, c := range gs.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrWATER {
					candidates = append(candidates, c)
				}
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Purge 2 WATER from Scrapheap", candidates, 2, 2)
			if err != nil {
				return false, err
			}
			for _, c := range chosen {
				d.purgeFromScrapheap(player, c, "FenrirMkII cost")
			}
			return true, nil
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			d.State.Players[player].RemoveFromHand(card)
			return d.executeSpecialSummon(card, player, PositionATK, FaceUp)
		},
	}
	return &Card{
		Name:        "Fenrir Mk.II",
		Description: "You can Special Summon this card from your hand by purging 2 WATER agents from your Scrapheap.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrWATER,
		AgentType:   "Bioweapon",
		ATK:         1400,
		DEF:         1200,
		IsEffect:    true,
		Effects:     []*CardEffect{ssEff},
	}
}

// --- Effect Agents: Umi-dependent ---

// AmphibiousMechMK3 — Direct attack while Umi on field.
func AmphibiousMechMK3() *Card {
	eff := &CardEffect{
		Name:       "Amphibious Direct Attack",
		EffectType: EffectContinuous,
		CanDirectAttack: func(d *Duel, card *CardInstance, player int) bool {
			return d.isNetGridOnField()
		},
	}
	return &Card{
		Name:        "Amphibious Mech MK-3",
		Description: "While \"NetGrid\" is on the field, this card can attack your opponent directly.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrWATER,
		AgentType:   "Machine",
		ATK:         1500,
		DEF:         1300,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// SirenEnforcer — While Umi on field, can attack twice.
func SirenEnforcer() *Card {
	// We handle double attack by resetting AttackedThisTurn after first attack
	// via OnBattleDamage callback
	eff := &CardEffect{
		Name:       "Siren Enforcer Double Attack",
		EffectType: EffectContinuous,
		OnBattleDamage: func(d *Duel, card *CardInstance, player int) {
			if d.isNetGridOnField() && card.AttackedThisTurn {
				// Allow a second attack by resetting the flag
				// Only allow once per turn using a counter
				if card.Counters["double_attacked"] == 0 {
					card.AttackedThisTurn = false
					card.Counters["double_attacked"] = 1
				}
			}
		},
	}
	return &Card{
		Name:        "Siren Enforcer",
		Description: "While \"NetGrid\" is on the field, this card can attack twice during each Battle Phase.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrWATER,
		AgentType:   "Wetware",
		ATK:         1500,
		DEF:         700,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// AbyssalCircuitLeviathan — Send 1 face-up Umi to Scrapheap; destroy all other cards on field.
func AbyssalCircuitLeviathan() *Card {
	eff := &CardEffect{
		Name:       "Levia-Mech Nuke",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectIgnition,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			return d.isNetGridOnField()
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			// Send Umi to Scrapheap
			for p := 0; p < 2; p++ {
				if fs := gs.Players[p].OS; fs != nil && fs.Face == FaceUp {
					if fs.Card.Name == "NetGrid" || fs.Card.Name == "The Undercity Grid" {
						d.destroyOS(p)
						break
					}
				}
			}
			// Destroy all other cards on field
			for p := 0; p < 2; p++ {
				for _, m := range gs.Players[p].Agents() {
					if m.ID != card.ID {
						d.destroyByEffect(m, "Abyssal Circuit Leviathan")
					}
				}
				for _, st := range gs.Players[p].TechCards() {
					d.destroyByEffect(st, "Abyssal Circuit Leviathan")
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Abyssal Circuit Leviathan",
		Description: "You can send 1 face-up \"NetGrid\" you control to the Scrapheap; destroy all other cards on the field.",
		CardType:    CardTypeAgent,
		Level:       7,
		Attribute:   AttrWATER,
		AgentType:   "Leviathan",
		ATK:         2600,
		DEF:         1500,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// ChromeborneHydraNexus — Can't normal summon. SS by tributing Levia-Dragon.
// Send Umi to Scrapheap; send all other cards to Scrapheap.
func ChromeborneHydraNexus() *Card {
	ssEff := &CardEffect{
		Name:       "Neo-Daedalus Special Summon",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectNone,
		SpecialSummonCondition: func(d *Duel, card *CardInstance, player int) bool {
			if d.State.Players[player].FreeAgentZone() == -1 {
				return false
			}
			for _, m := range d.State.Players[player].FaceUpAgents() {
				if m.Card.Name == "Abyssal Circuit Leviathan" {
					return true
				}
			}
			return false
		},
		Cost: func(d *Duel, card *CardInstance, player int) (bool, error) {
			gs := d.State
			var candidates []*CardInstance
			for _, m := range gs.Players[player].FaceUpAgents() {
				if m.Card.Name == "Abyssal Circuit Leviathan" {
					candidates = append(candidates, m)
				}
			}
			if len(candidates) == 0 {
				return false, nil
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Sacrifice Abyssal Circuit Leviathan", candidates, 1, 1)
			if err != nil {
				return false, err
			}
			gs.Players[player].RemoveAgent(chosen[0])
			gs.Players[player].SendToScrapheap(chosen[0])
			d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), player, chosen[0].Card.Name, "sacrificed for Neo-Daedalus"))
			return true, nil
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			d.State.Players[player].RemoveFromHand(card)
			return d.executeSpecialSummon(card, player, PositionATK, FaceUp)
		},
	}
	nukeEff := &CardEffect{
		Name:       "Neo-Daedalus Nuke",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectIgnition,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			return d.isNetGridOnField()
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			// Send Umi to Scrapheap
			for p := 0; p < 2; p++ {
				if fs := gs.Players[p].OS; fs != nil && fs.Face == FaceUp {
					if fs.Card.Name == "NetGrid" || fs.Card.Name == "The Undercity Grid" {
						d.destroyOS(p)
						break
					}
				}
			}
			// Send ALL other cards (field + hand) to Scrapheap except this card
			for p := 0; p < 2; p++ {
				for _, m := range gs.Players[p].Agents() {
					if m.ID != card.ID {
						d.destroyByEffect(m, "Chromeborne Hydra Nexus")
					}
				}
				for _, st := range gs.Players[p].TechCards() {
					d.destroyByEffect(st, "Chromeborne Hydra Nexus")
				}
				// Send hand to Scrapheap
				for len(gs.Players[p].Hand) > 0 {
					c := gs.Players[p].Hand[0]
					gs.Players[p].RemoveFromHand(c)
					gs.Players[p].SendToScrapheap(c)
					d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), p, c.Card.Name, "Neo-Daedalus"))
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Chromeborne Hydra Nexus",
		Description: "Cannot be Normal Summoned/Set. Must be Special Summoned by Tributing 1 \"Abyssal Circuit Leviathan\". You can send 1 face-up \"NetGrid\" you control to the Scrapheap; send all other cards on the field and in both players' hands to the Scrapheap.",
		CardType:    CardTypeAgent,
		Level:       8,
		Attribute:   AttrWATER,
		AgentType:   "Leviathan",
		ATK:         2900,
		DEF:         1600,
		IsEffect:    true,
		Effects:     []*CardEffect{ssEff, nukeEff},
	}
}

// --- Effect Agents: Misc ---

// StealthGlider — When normal summoned, no traps can be activated in response.
func StealthGlider() *Card {
	// Implementation note: This prevents traps from being activated in the post-summon
	// response window. Simplified: it's a L3 1300/1200 beater. The trap suppression
	// would require engine changes to the effect serialization/response window system.
	// For now, just define the card stats.
	return &Card{
		Name:        "Stealth Glider",
		Description: "When this card is Normal Summoned: Your opponent cannot activate Trap cards in response to the Summon.",
		CardType:    CardTypeAgent,
		Level:       3,
		Attribute:   AttrWATER,
		AgentType:   "Abyssal",
		ATK:         1300,
		DEF:         1200,
		IsEffect:    true,
		Effects:     []*CardEffect{},
	}
}

// RagingPlasmaSprite — Direct attack. +1000 ATK when deals direct battle damage.
func RagingPlasmaSprite() *Card {
	directEff := &CardEffect{
		Name:       "Raging Plasma Sprite Direct Attack",
		EffectType: EffectContinuous,
		CanDirectAttack: func(d *Duel, card *CardInstance, player int) bool {
			return true
		},
		OnBattleDamage: func(d *Duel, card *CardInstance, player int) {
			// Gain 1000 ATK permanently
			card.AddModifier(StatModifier{Source: card.ID, ATKMod: 1000, Permanent: true})
		},
	}
	return &Card{
		Name:        "Raging Plasma Sprite",
		Description: "This card can attack your opponent directly. When this card inflicts battle damage to your opponent: This card gains 1000 ATK.",
		CardType:    CardTypeAgent,
		Level:       3,
		Attribute:   AttrFIRE,
		AgentType:   "Burner",
		ATK:         100,
		DEF:         200,
		IsEffect:    true,
		Effects:     []*CardEffect{directEff},
	}
}

// SolarFlareSerpent — Can't be attacked while you control another Pyro. End Phase: 500 damage.
func SolarFlareSerpent() *Card {
	cantBeAttacked := &CardEffect{
		Name:       "Solar Flare Serpent Protection",
		EffectType: EffectContinuous,
		TargetRestriction: func(d *Duel, card *CardInstance, player int) bool {
			// Can be attacked only if controller has no other Pyro
			for _, m := range d.State.Players[player].FaceUpAgents() {
				if m.ID != card.ID && m.Card.AgentType == "Burner" {
					return false // can't be attacked
				}
			}
			return true
		},
	}
	burnEff := &CardEffect{
		Name:          "Solar Flare Serpent Burn",
		ExecSpeed:     ExecSpeed1,
		EffectType:    EffectTrigger,
		IsTrigger:     true,
		IsMandatory:   true,
		TriggerEvent:  log.EventPhaseChange,
		OnFieldEffect: func(d *Duel, card *CardInstance, player int) {},
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			return d.State.Phase == PhaseEnd && d.State.TurnPlayer == player
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			opp := d.State.Opponent(player)
			d.applyEffectDamage(opp, 500, "Solar Flare Serpent")
			return nil
		},
	}
	return &Card{
		Name:        "Solar Flare Serpent",
		Description: "Cannot be attacked while you control another Burner agent. During the End Phase: Inflict 500 damage to your opponent.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrFIRE,
		AgentType:   "Burner",
		ATK:         1500,
		DEF:         1000,
		IsEffect:    true,
		Effects:     []*CardEffect{cantBeAttacked, burnEff},
	}
}

// GhostProcess — When destroyed by battle, SS from Scrapheap at End Phase.
func GhostProcess() *Card {
	eff := &CardEffect{
		Name:       "Ghost Process Revival",
		EffectType: EffectTrigger,
		OnBattleDestruction: func(d *Duel, card *CardInstance, player int) {
			// Mark for end phase revival
			card.Counters["revive_ep"] = 1
		},
	}
	reviveEff := &CardEffect{
		Name:         "Ghost Process End Phase",
		ExecSpeed:    ExecSpeed1,
		EffectType:   EffectTrigger,
		IsTrigger:    true,
		IsMandatory:  true,
		TriggerEvent: log.EventPhaseChange,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			return card.Zone == ZoneScrapheap && d.State.Phase == PhaseEnd && card.Counters["revive_ep"] > 0
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			card.Counters["revive_ep"] = 0
			if gs.Players[player].FreeAgentZone() == -1 {
				return nil
			}
			d.removeFromScrapheap(player, card)
			return d.executeSpecialSummon(card, player, PositionDEF, FaceUp)
		},
	}
	return &Card{
		Name:        "Ghost Process",
		Description: "If this card is destroyed by battle: During the End Phase, Special Summon this card from the Scrapheap in DEF Position.",
		CardType:    CardTypeAgent,
		Level:       2,
		Attribute:   AttrFIRE,
		AgentType:   "Burner",
		ATK:         300,
		DEF:         200,
		IsEffect:    true,
		Effects:     []*CardEffect{eff, reviveEff},
	}
}

// GaiaCoreTheVolatileSwarm — Sacrifice Pyros for +1000 ATK each. Piercing. Self-destruct at EP.
func GaiaCoreTheVolatileSwarm() *Card {
	sacrificeEff := &CardEffect{
		Name:       "Gaia Core Sacrifice",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectIgnition,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			if card.Counters["gaia_used"] > 0 {
				return false
			}
			for _, m := range d.State.Players[player].FaceUpAgents() {
				if m.ID != card.ID && m.Card.AgentType == "Burner" {
					return true
				}
			}
			return false
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			var candidates []*CardInstance
			for _, m := range gs.Players[player].FaceUpAgents() {
				if m.ID != card.ID && m.Card.AgentType == "Burner" {
					candidates = append(candidates, m)
				}
			}
			max := 2
			if len(candidates) < max {
				max = len(candidates)
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Sacrifice up to 2 Pyro agents", candidates, 1, max)
			if err != nil {
				return err
			}
			for _, c := range chosen {
				gs.Players[player].RemoveAgent(c)
				gs.Players[player].SendToScrapheap(c)
				d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), player, c.Card.Name, "sacrificed for Gaia Core"))
				card.AddModifier(StatModifier{Source: card.ID, ATKMod: 1000, Permanent: true})
			}
			card.Counters["gaia_used"] = 1
			return nil
		},
	}
	piercingEff := &CardEffect{
		Name:        "Gaia Core Piercing",
		EffectType:  EffectContinuous,
		HasPiercing: true,
	}
	selfDestructEff := &CardEffect{
		Name:          "Gaia Core Self Destruct",
		ExecSpeed:     ExecSpeed1,
		EffectType:    EffectTrigger,
		IsTrigger:     true,
		IsMandatory:   true,
		TriggerEvent:  log.EventPhaseChange,
		OnFieldEffect: func(d *Duel, card *CardInstance, player int) {},
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			return d.State.Phase == PhaseEnd && d.State.TurnPlayer == player
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			if d.isOnField(card) {
				d.destroyByEffect(card, "Gaia Core self-destruct")
			}
			return nil
		},
	}
	return &Card{
		Name:        "Gaia Core the Volatile Swarm",
		Description: "Once per turn: You can Tribute up to 2 Burner agents; this card gains 1000 ATK for each. This card can inflict piercing battle damage. During your End Phase: Destroy this card.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrFIRE,
		AgentType:   "Burner",
		ATK:         2000,
		DEF:         0,
		IsEffect:    true,
		Effects:     []*CardEffect{sacrificeEff, piercingEff, selfDestructEff},
	}
}

// MoltenCyborg — Draw 1 when Special Summoned from Scrapheap.
func MoltenCyborg() *Card {
	eff := &CardEffect{
		Name:         "Molten Cyborg Draw",
		ExecSpeed:    ExecSpeed1,
		EffectType:   EffectTrigger,
		IsTrigger:    true,
		IsMandatory:  true,
		TriggerEvent: log.EventSpecialSummon,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			// Trigger when this card is special summoned
			return d.State.LastSummonEvent != nil && d.State.LastSummonEvent.Card.ID == card.ID
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			drawn := gs.Players[player].DrawCard()
			if drawn != nil {
				d.log(log.NewDrawEvent(gs.Turn, gs.Phase.String(), player, drawn.Card.Name))
			}
			return nil
		},
	}
	return &Card{
		Name:        "Molten Cyborg",
		Description: "When this card is Special Summoned: Draw 1 card.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrFIRE,
		AgentType:   "Burner",
		ATK:         1600,
		DEF:         400,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}

// UltimateStreetPunk — +1000 ATK per face-up FIRE (except itself). Sacrifice FIRE for 500 damage.
func UltimateStreetPunk() *Card {
	contEff := &CardEffect{
		Name:       "Ultimate Street Punk ATK Boost",
		EffectType: EffectContinuous,
		ContinuousApply: func(d *Duel, card *CardInstance, player int) {
			gs := d.State
			count := 0
			for p := 0; p < 2; p++ {
				for _, m := range gs.Players[p].FaceUpAgents() {
					if m.ID != card.ID && m.Card.Attribute == AttrFIRE {
						count++
					}
				}
			}
			if count > 0 {
				card.AddModifier(StatModifier{Source: card.ID, ATKMod: count * 1000, Continuous: true})
			}
		},
	}
	ignEff := &CardEffect{
		Name:       "Ultimate Street Punk Burn",
		ExecSpeed:  ExecSpeed1,
		EffectType: EffectIgnition,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			for _, m := range d.State.Players[player].FaceUpAgents() {
				if m.ID != card.ID && m.Card.Attribute == AttrFIRE {
					return true
				}
			}
			return false
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			var candidates []*CardInstance
			for _, m := range gs.Players[player].FaceUpAgents() {
				if m.ID != card.ID && m.Card.Attribute == AttrFIRE {
					candidates = append(candidates, m)
				}
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Sacrifice 1 FIRE agent", candidates, 1, 1)
			if err != nil {
				return err
			}
			gs.Players[player].RemoveAgent(chosen[0])
			gs.Players[player].SendToScrapheap(chosen[0])
			d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), player, chosen[0].Card.Name, "sacrificed for Ultimate Street Punk"))
			opp := gs.Opponent(player)
			d.applyEffectDamage(opp, 500, "Ultimate Street Punk")
			return nil
		},
	}
	return &Card{
		Name:        "Ultimate Street Punk",
		Description: "This card gains 1000 ATK for each face-up FIRE agent on the field, except itself. Once per turn: You can Tribute 1 face-up FIRE agent you control; inflict 500 damage to your opponent.",
		CardType:    CardTypeAgent,
		Level:       3,
		Attribute:   AttrFIRE,
		AgentType:   "Enforcer",
		ATK:         500,
		DEF:         1000,
		IsEffect:    true,
		Effects:     []*CardEffect{contEff, ignEff},
	}
}

// JunkyardLurker — Counts as 2 sacrifices for a WATER agent.
func JunkyardLurker() *Card {
	// Implementation note: this would require engine changes to the sacrifice system
	// to allow a single agent to count as 2 sacrifices. For now, define the card.
	return &Card{
		Name:        "Junkyard Lurker",
		Description: "You can Tribute this card to Tribute Summon 1 WATER agent. This card counts as 2 Tributes for the Tribute Summon of a WATER agent.",
		CardType:    CardTypeAgent,
		Level:       4,
		Attribute:   AttrWATER,
		AgentType:   "Abyssal",
		ATK:         1500,
		DEF:         1600,
		IsEffect:    true,
		Effects:     []*CardEffect{},
	}
}

// ScorchedCircuitDespot — L9. On sacrifice summon, purge up to 5 FIRE from Scrapheap, destroy that many Tech.
func ScorchedCircuitDespot() *Card {
	eff := &CardEffect{
		Name:         "Scorched Circuit Despot",
		ExecSpeed:    ExecSpeed1,
		EffectType:   EffectTrigger,
		IsTrigger:    true,
		IsMandatory:  false,
		TriggerEvent: log.EventSacrificeSummon,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			if d.State.LastSummonEvent == nil || d.State.LastSummonEvent.Card.ID != card.ID {
				return false
			}
			// Need at least 1 FIRE in Scrapheap
			for _, c := range d.State.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrFIRE {
					return true
				}
			}
			return false
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			// Choose up to 5 FIRE agents from Scrapheap to purge
			var candidates []*CardInstance
			for _, c := range gs.Players[player].Scrapheap {
				if c.Card.CardType == CardTypeAgent && c.Card.Attribute == AttrFIRE {
					candidates = append(candidates, c)
				}
			}
			max := 5
			if len(candidates) < max {
				max = len(candidates)
			}
			chosen, err := d.Controllers[player].ChooseCards(d.ctx, gs, "Purge up to 5 FIRE from Scrapheap", candidates, 1, max)
			if err != nil {
				return err
			}
			for _, c := range chosen {
				d.purgeFromScrapheap(player, c, "Scorched Circuit Despot")
			}
			// Destroy that many Tech
			count := len(chosen)
			var stCandidates []*CardInstance
			for p := 0; p < 2; p++ {
				stCandidates = append(stCandidates, gs.Players[p].TechCards()...)
			}
			if len(stCandidates) == 0 || count == 0 {
				return nil
			}
			if count > len(stCandidates) {
				count = len(stCandidates)
			}
			toDestroy, err := d.Controllers[player].ChooseCards(d.ctx, gs, fmt.Sprintf("Choose %d Tech to destroy", count), stCandidates, count, count)
			if err != nil {
				return err
			}
			for _, t := range toDestroy {
				if d.isOnField(t) {
					d.destroyByEffect(t, "Scorched Circuit Despot")
				}
			}
			return nil
		},
	}
	return &Card{
		Name:        "Scorched Circuit Despot",
		Description: "When this card is Sacrifice Summoned: You can purge up to 5 FIRE agents from your Scrapheap; destroy Programs/Traps on the field, up to the number purged.",
		CardType:    CardTypeAgent,
		Level:       9,
		Attribute:   AttrFIRE,
		AgentType:   "Burner",
		ATK:         2700,
		DEF:         1600,
		IsEffect:    true,
		Effects:     []*CardEffect{eff},
	}
}
