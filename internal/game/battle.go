package game

import (
	"fmt"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// computeBattlePhaseActions returns legal actions during the Battle Phase.
func (d *Duel) computeBattlePhaseActions() []Action {
	gs := d.State
	tp := gs.TurnPlayer
	p := gs.Players[tp]
	opp := gs.Players[gs.Opponent(tp)]
	var actions []Action

	// Eligible attackers: face-up ATK position agents that haven't attacked
	for _, m := range p.AgentZones {
		if m == nil || m.Face != FaceUp || m.Position != PositionATK || m.AttackedThisTurn {
			continue
		}

		// Check if this agent is restricted from attacking (Gravity Bind, etc.)
		if !d.canAgentAttack(m) {
			continue
		}

		oppAgents := opp.Agents()
		if len(oppAgents) > 0 {
			// Filter out untargetable agents
			var targetable []*CardInstance
			for _, target := range oppAgents {
				if d.canAgentBeAttacked(target) {
					targetable = append(targetable, target)
				}
			}

			if len(targetable) > 0 {
				for _, target := range targetable {
					targetDesc := target.Card.Name
					if target.Face == FaceDown {
						targetDesc = fmt.Sprintf("face-down agent (Zone %d)", target.ZoneIndex+1)
					}
					actions = append(actions, Action{
						Type:    ActionAttack,
						Player:  tp,
						Card:    m,
						Targets: []*CardInstance{target},
						Desc:    fmt.Sprintf("Attack with %s → %s", m.Card.Name, targetDesc),
					})
				}
			}

			// Check for conditional direct attack (Raging Plasma Sprite, etc.)
			if d.canDirectAttackWithDefenders(m) {
				actions = append(actions, Action{
					Type:   ActionDirectAttack,
					Player: tp,
					Card:   m,
					Desc:   fmt.Sprintf("Direct attack with %s (ATK %d)", m.Card.Name, m.CurrentATK()),
				})
			}

			// If all opponents are untargetable but no direct attack, offer direct attack
			if len(targetable) == 0 && !d.canDirectAttackWithDefenders(m) {
				actions = append(actions, Action{
					Type:   ActionDirectAttack,
					Player: tp,
					Card:   m,
					Desc:   fmt.Sprintf("Direct attack with %s (ATK %d)", m.Card.Name, m.CurrentATK()),
				})
			}
		} else {
			// Direct attack (no opponent agents)
			actions = append(actions, Action{
				Type:   ActionDirectAttack,
				Player: tp,
				Card:   m,
				Desc:   fmt.Sprintf("Direct attack with %s (ATK %d)", m.Card.Name, m.CurrentATK()),
			})
		}
	}

	// End battle / go to MP2
	actions = append(actions, Action{
		Type: ActionEnterMainPhase2,
		Desc: "Enter Main Phase 2",
	})

	return actions
}

// executeAttack performs an attack against an opponent's agent.
func (d *Duel) executeAttack(action Action) error {
	gs := d.State
	tp := action.Player
	opp := gs.Opponent(tp)

	attacker := action.Card
	defender := action.Targets[0]

	attacker.AttackedThisTurn = true
	gs.CurrentAttacker = attacker
	gs.CurrentTarget = defender

	// Log attack declaration
	defenderName := defender.Card.Name
	if defender.Face == FaceDown {
		defenderName = fmt.Sprintf("face-down agent (Zone %d)", defender.ZoneIndex+1)
	}
	d.log(log.NewAttackDeclareEvent(gs.Turn, tp, attacker.Card.Name, defenderName))

	// Response window after attack declaration (e.g. Reflector Array, Reactive Plating)
	if err := d.openResponseWindow(opp); err != nil {
		return err
	}
	if gs.Chain != nil {
		if err := d.resolveChain(); err != nil {
			return err
		}
	}
	if gs.Over {
		return nil
	}

	// Check if attacker was removed during response
	if !d.isOnField(attacker) {
		return nil // attack stops
	}
	// Re-check attack restrictions (e.g. Gravity Clamp activated during response)
	if !d.canAgentAttack(attacker) {
		d.log(log.NewAttackStoppedEvent(gs.Turn, tp, attacker.Card.Name, "restriction"))
		attacker.AttackedThisTurn = false
		gs.CurrentAttacker = nil
		gs.CurrentTarget = nil
		return nil
	}
	// Check if defender was removed during response — battle replay
	if !d.isOnField(defender) {
		d.log(log.NewReplayEvent(gs.Turn, tp, attacker.Card.Name))

		oppAgents := gs.Players[opp].Agents()
		if len(oppAgents) == 0 {
			// No targets: attacker can do a direct attack or cancel
			replayActions := []Action{
				{Type: ActionDirectAttack, Player: tp, Card: attacker, Desc: fmt.Sprintf("Direct attack with %s", attacker.Card.Name)},
				{Type: ActionPass, Player: tp, Desc: "Cancel attack"},
			}
			chosen, err := d.Controllers[tp].ChooseAction(d.ctx, gs, replayActions)
			if err != nil {
				return err
			}
			if chosen.Type == ActionPass {
				gs.CurrentAttacker = nil
				gs.CurrentTarget = nil
				return nil
			}
			// Direct attack
			gs.CurrentTarget = nil
			atkVal := attacker.CurrentATK()
			d.log(log.NewDamageCalcEvent(gs.Turn, tp,
				fmt.Sprintf("Direct attack: %s (ATK %d) → P%d", attacker.Card.Name, atkVal, opp+1)))
			d.applyDamage(opp, atkVal, fmt.Sprintf("direct attack by %s (replay)", attacker.Card.Name))
			gs.CurrentAttacker = nil
			gs.CurrentTarget = nil
			return nil
		}

		// Has targets: choose a new one or cancel
		var replayActions []Action
		for _, target := range oppAgents {
			targetDesc := target.Card.Name
			if target.Face == FaceDown {
				targetDesc = fmt.Sprintf("face-down agent (Zone %d)", target.ZoneIndex+1)
			}
			replayActions = append(replayActions, Action{
				Type:    ActionAttack,
				Player:  tp,
				Card:    attacker,
				Targets: []*CardInstance{target},
				Desc:    fmt.Sprintf("Attack %s (replay)", targetDesc),
			})
		}
		replayActions = append(replayActions, Action{Type: ActionPass, Player: tp, Desc: "Cancel attack"})

		chosen, err := d.Controllers[tp].ChooseAction(d.ctx, gs, replayActions)
		if err != nil {
			return err
		}
		if chosen.Type == ActionPass {
			gs.CurrentAttacker = nil
			gs.CurrentTarget = nil
			return nil
		}
		// Replace defender and continue with attack
		defender = chosen.Targets[0]
		gs.CurrentTarget = defender
	}

	// If defender is face-down, flip it face-up (no flip summon, just reveal)
	wasFlipped := false
	if defender.Face == FaceDown {
		defender.Face = FaceUp
		wasFlipped = true
		d.log(log.NewFlipEvent(gs.Turn, gs.Phase.String(), opp, defender.Card.Name))
	}

	// Damage calculation
	atkVal := attacker.CurrentATK()

	var destroyedAgents []*CardInstance
	battleDamageDealt := false
	if defender.Position == PositionATK {
		// ATK vs ATK
		defATK := defender.CurrentATK()
		d.log(log.NewDamageCalcEvent(gs.Turn, tp,
			fmt.Sprintf("Damage calc: %s (ATK %d) vs %s (ATK %d)", attacker.Card.Name, atkVal, defender.Card.Name, defATK)))

		if atkVal > defATK {
			// Attacker wins: defender destroyed, opponent takes damage
			damage := atkVal - defATK
			d.destroyByBattle(defender, opp)
			destroyedAgents = append(destroyedAgents, defender)
			d.applyDamage(opp, damage, fmt.Sprintf("battle: %s vs %s", attacker.Card.Name, defender.Card.Name))
			battleDamageDealt = true
		} else if defATK > atkVal {
			// Defender wins: attacker destroyed, turn player takes damage
			damage := defATK - atkVal
			d.destroyByBattle(attacker, tp)
			destroyedAgents = append(destroyedAgents, attacker)
			d.applyDamage(tp, damage, fmt.Sprintf("battle: %s vs %s", attacker.Card.Name, defender.Card.Name))
		} else {
			// Tie: both destroyed, no damage
			d.destroyByBattle(attacker, tp)
			d.destroyByBattle(defender, opp)
			destroyedAgents = append(destroyedAgents, attacker, defender)
		}
		if battleDamageDealt && d.isOnField(attacker) && attacker.Card.IsEffect {
			d.checkBattleDamageTrigger(attacker, tp)
		}
		// "Destroys by battle" triggers (separate from dealing damage)
		if atkVal > defATK && attacker.Card.IsEffect {
			d.checkDestroyByBattleTrigger(attacker, tp)
		} else if defATK > atkVal && defender.Card.IsEffect {
			d.checkDestroyByBattleTrigger(defender, opp)
		}
	} else {
		// ATK vs DEF
		defDEF := defender.CurrentDEF()
		d.log(log.NewDamageCalcEvent(gs.Turn, tp,
			fmt.Sprintf("Damage calc: %s (ATK %d) vs %s (DEF %d)", attacker.Card.Name, atkVal, defender.Card.Name, defDEF)))

		if atkVal > defDEF {
			d.destroyByBattle(defender, opp)
			destroyedAgents = append(destroyedAgents, defender)
			// "Destroys by battle" trigger
			if attacker.Card.IsEffect {
				d.checkDestroyByBattleTrigger(attacker, tp)
			}
			// Piercing damage check
			if d.hasPiercing(attacker) {
				pierceDmg := atkVal - defDEF
				d.applyDamage(opp, pierceDmg, fmt.Sprintf("piercing: %s vs %s", attacker.Card.Name, defender.Card.Name))
				if d.isOnField(attacker) && attacker.Card.IsEffect {
					d.checkBattleDamageTrigger(attacker, tp)
				}
			}
		} else if defDEF > atkVal {
			// Defender wins: no destruction, attacker takes damage
			damage := defDEF - atkVal
			d.applyDamage(tp, damage, fmt.Sprintf("battle: %s vs %s", attacker.Card.Name, defender.Card.Name))
		}
		// Tie: nothing happens
	}

	// Process flip effects after damage (if defender was face-down and survived)
	if wasFlipped && d.isOnField(defender) && defender.Card.IsEffect {
		d.queueFlipEffects(defender, opp)
		if len(d.State.PendingTriggers) > 0 {
			if err := d.processEffectSerialization(log.EventFlipNoSummon); err != nil {
				return err
			}
		}
	}

	// Check for battle destruction triggers (Mother Grizzly, UFO Turtle, etc.)
	if len(destroyedAgents) > 0 && !gs.Over {
		d.checkBattleDestructionTriggers(destroyedAgents)
	}

	gs.CurrentAttacker = nil
	gs.CurrentTarget = nil
	d.recalculateContinuousEffects()

	return nil
}

// executeDirectAttack performs a direct attack on the opponent.
func (d *Duel) executeDirectAttack(action Action) error {
	gs := d.State
	tp := action.Player
	opp := gs.Opponent(tp)
	attacker := action.Card

	attacker.AttackedThisTurn = true
	gs.CurrentAttacker = attacker
	gs.CurrentTarget = nil

	d.log(log.NewDirectAttackDeclareEvent(gs.Turn, tp, attacker.Card.Name))

	// Response window after direct attack declaration
	if err := d.openResponseWindow(opp); err != nil {
		return err
	}
	if gs.Chain != nil {
		if err := d.resolveChain(); err != nil {
			return err
		}
	}
	if gs.Over {
		return nil
	}
	// Check if attacker was removed
	if !d.isOnField(attacker) {
		return nil
	}
	// Re-check attack restrictions (e.g. Gravity Clamp activated during response)
	if !d.canAgentAttack(attacker) {
		d.log(log.NewAttackStoppedEvent(gs.Turn, tp, attacker.Card.Name, "restriction"))
		attacker.AttackedThisTurn = false
		gs.CurrentAttacker = nil
		gs.CurrentTarget = nil
		return nil
	}

	atkVal := attacker.CurrentATK()
	d.log(log.NewDamageCalcEvent(gs.Turn, tp,
		fmt.Sprintf("Direct attack: %s (ATK %d) → P%d", attacker.Card.Name, atkVal, opp+1)))

	d.applyDamage(opp, atkVal, fmt.Sprintf("direct attack by %s", attacker.Card.Name))

	// Check for battle damage triggers (e.g. Aero-Knight Parshath draw)
	if d.isOnField(attacker) && attacker.Card.IsEffect {
		d.checkBattleDamageTrigger(attacker, tp)
	}

	gs.CurrentAttacker = nil
	gs.CurrentTarget = nil

	return nil
}

// destroyByBattle sends a agent to its owner's scrapheap as a result of battle destruction.
func (d *Duel) destroyByBattle(card *CardInstance, controller int) {
	gs := d.State
	p := gs.Players[controller]

	d.log(log.NewBattleDestroyEvent(gs.Turn, controller, card.Card.Name))

	// Destroy equips attached to this agent
	d.destroyEquips(card)

	p.RemoveAgent(card)

	// Cards go to owner's scrapheap, not controller's
	owner := gs.Players[card.Owner]
	owner.SendToScrapheap(card)

	d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), card.Owner, card.Card.Name, "destroyed by battle"))
}

// isOnField checks if a card instance is still on the field (agent, tech, or OS zone).
func (d *Duel) isOnField(card *CardInstance) bool {
	gs := d.State
	for p := 0; p < 2; p++ {
		for _, m := range gs.Players[p].AgentZones {
			if m != nil && m.ID == card.ID {
				return true
			}
		}
		for _, st := range gs.Players[p].TechZones {
			if st != nil && st.ID == card.ID {
				return true
			}
		}
		if gs.Players[p].OS != nil && gs.Players[p].OS.ID == card.ID {
			return true
		}
	}
	return false
}

// hasPiercing checks if an attacker has a piercing damage effect.
func (d *Duel) hasPiercing(attacker *CardInstance) bool {
	if !attacker.Card.IsEffect {
		return false
	}
	for _, eff := range attacker.Card.Effects {
		if eff.HasPiercing {
			return true
		}
	}
	return false
}

// checkBattleDamageTrigger fires any "when this card deals battle damage" triggers.
func (d *Duel) checkBattleDamageTrigger(attacker *CardInstance, controller int) {
	for _, eff := range attacker.Card.Effects {
		if eff.OnBattleDamage != nil {
			eff.OnBattleDamage(d, attacker, controller)
		}
	}
}

// checkDestroyByBattleTrigger fires any "when this card destroys a agent by battle" triggers.
func (d *Duel) checkDestroyByBattleTrigger(victor *CardInstance, controller int) {
	for _, eff := range victor.Card.Effects {
		if eff.OnDestroyByBattle != nil {
			eff.OnDestroyByBattle(d, victor, controller)
		}
	}
}

// checkBattleDestructionTriggers checks if agents destroyed by battle have triggers.
func (d *Duel) checkBattleDestructionTriggers(destroyed []*CardInstance) {
	gs := d.State
	for _, card := range destroyed {
		for _, eff := range card.Card.Effects {
			if eff.OnBattleDestruction != nil {
				gs.PendingTriggers = append(gs.PendingTriggers, PendingTrigger{
					Card: card,
					Effect: &CardEffect{
						Name:        eff.Name + " (battle destruction)",
						ExecSpeed:   ExecSpeed1,
						IsTrigger:   true,
						IsMandatory: !eff.IsMandatory, // optional triggers need confirmation
						Resolve: func(d *Duel, c *CardInstance, p int, t []*CardInstance) error {
							eff.OnBattleDestruction(d, card, card.Owner)
							return nil
						},
					},
					Controller: card.Owner,
				})
			}
		}
	}
	if len(gs.PendingTriggers) > 0 {
		_ = d.processEffectSerialization(log.EventBattleDestroy)
	}
}

// canAgentAttack checks if a agent is allowed to attack (level restrictions, etc.).
func (d *Duel) canAgentAttack(agent *CardInstance) bool {
	gs := d.State
	for p := 0; p < 2; p++ {
		// Check face-up tech
		for _, st := range gs.Players[p].TechCards() {
			if st.Face != FaceUp {
				continue
			}
			for _, eff := range st.Card.Effects {
				if eff.AttackRestriction != nil && !eff.AttackRestriction(d, agent) {
					return false
				}
			}
		}
		// Check OS
		if fs := gs.Players[p].OS; fs != nil && fs.Face == FaceUp {
			for _, eff := range fs.Card.Effects {
				if eff.AttackRestriction != nil && !eff.AttackRestriction(d, agent) {
					return false
				}
			}
		}
	}
	return true
}

// canAgentBeAttacked checks if a agent can be targeted for an attack.
func (d *Duel) canAgentBeAttacked(agent *CardInstance) bool {
	for _, eff := range agent.Card.Effects {
		if eff.TargetRestriction != nil && !eff.TargetRestriction(d, agent, agent.Controller) {
			return false
		}
	}
	return true
}

// canDirectAttackWithDefenders checks if a agent can attack directly even when opponent has agents.
func (d *Duel) canDirectAttackWithDefenders(agent *CardInstance) bool {
	for _, eff := range agent.Card.Effects {
		if eff.CanDirectAttack != nil && eff.CanDirectAttack(d, agent, agent.Controller) {
			return true
		}
	}
	return false
}

// applyDamage reduces a player's HP and checks win conditions.
func (d *Duel) applyDamage(player int, amount int, reason string) {
	gs := d.State
	p := gs.Players[player]

	oldHP := p.HP
	p.HP -= amount
	if p.HP < 0 {
		p.HP = 0
	}

	d.log(log.NewHPChangeEvent(gs.Turn, gs.Phase.String(), player, oldHP, p.HP, reason))

	if gs.CheckWinCondition() {
		d.log(log.NewWinEvent(gs.Turn, gs.Phase.String(), gs.Winner, gs.Result))
	}
}

// applyEffectDamage reduces HP and also triggers Dark Room of Nightmare type effects.
func (d *Duel) applyEffectDamage(player int, amount int, reason string) {
	d.applyDamage(player, amount, reason)
	if d.State.Over {
		return
	}
	// Check for "when opponent takes effect damage" triggers (Dark Room of Nightmare)
	gs := d.State
	for p := 0; p < 2; p++ {
		for _, st := range gs.Players[p].TechCards() {
			if st.Face != FaceUp {
				continue
			}
			for _, eff := range st.Card.Effects {
				if eff.OnFieldEffect != nil && eff.Name == "Torture Subnet" {
					eff.OnFieldEffect(d, st, p)
				}
			}
		}
	}
}

// destroyOS destroys a player's OS and sends it to scrapheap.
func (d *Duel) destroyOS(player int) {
	gs := d.State
	fs := gs.Players[player].OS
	if fs == nil {
		return
	}

	d.log(log.NewDestroyEvent(gs.Turn, gs.Phase.String(), player, fs.Card.Name, "OS replaced"))
	d.triggerOnLeaveField(fs)
	gs.Players[player].OS = nil
	gs.Players[fs.Owner].SendToScrapheap(fs)
	d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), fs.Owner, fs.Card.Name, "OS replaced"))
}

// isNetGridOnField checks if "NetGrid" (or a card treated as "NetGrid") is face-up on the field.
func (d *Duel) isNetGridOnField() bool {
	gs := d.State
	for p := 0; p < 2; p++ {
		if fs := gs.Players[p].OS; fs != nil && fs.Face == FaceUp {
			if fs.Card.Name == "NetGrid" || fs.Card.Name == "The Undercity Grid" {
				return true
			}
		}
	}
	return false
}
