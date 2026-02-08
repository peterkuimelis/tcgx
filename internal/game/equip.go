package game

import "github.com/peterkuimelis/tcgx/internal/log"

// attachEquip attaches an equip program to a target agent and applies its modifier.
func (d *Duel) attachEquip(equip *CardInstance, target *CardInstance, atkMod, defMod int) {
	equip.EquippedTo = target
	target.Equips = append(target.Equips, equip)
	target.AddModifier(StatModifier{
		Source: equip.ID,
		ATKMod: atkMod,
		DEFMod: defMod,
	})
}

// detachEquip removes an equip from its target agent and removes its modifier.
func (d *Duel) detachEquip(equip *CardInstance) {
	if equip.EquippedTo == nil {
		return
	}
	target := equip.EquippedTo
	target.RemoveModifiersBySource(equip.ID)

	// Remove from target's Equips list
	for i, e := range target.Equips {
		if e.ID == equip.ID {
			target.Equips = append(target.Equips[:i], target.Equips[i+1:]...)
			break
		}
	}
	equip.EquippedTo = nil
}

// triggerOnLeaveField calls OnLeaveField handlers for a card about to leave the field.
// Must be called before detachEquip/RemoveAgent/RemoveFromTech so that EquippedTo is still set.
func (d *Duel) triggerOnLeaveField(card *CardInstance) {
	for _, eff := range card.Card.Effects {
		if eff.OnLeaveField != nil {
			eff.OnLeaveField(d, card, card.Controller)
		}
	}
}

// destroyEquips destroys all equip cards attached to a agent.
func (d *Duel) destroyEquips(agent *CardInstance) {
	// Copy the slice since destroying modifies it
	equips := make([]*CardInstance, len(agent.Equips))
	copy(equips, agent.Equips)

	for _, equip := range equips {
		d.detachEquip(equip)
		if equip.Zone == ZoneTech {
			gs := d.State
			gs.Players[equip.Controller].RemoveFromTech(equip)
			gs.Players[equip.Owner].SendToScrapheap(equip)
			d.log(log.NewDestroyEvent(gs.Turn, gs.Phase.String(), equip.Controller, equip.Card.Name, "equipped agent left field"))
			d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), equip.Owner, equip.Card.Name, "equipped agent left field"))
		}
	}
}
