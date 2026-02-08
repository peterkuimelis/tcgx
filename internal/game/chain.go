package game

import (
	"fmt"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// ChainLink represents a single link in a chain.
type ChainLink struct {
	Index      int
	Card       *CardInstance
	Effect     *CardEffect
	Controller int
	Targets    []*CardInstance
}

// Chain represents an active chain of effects waiting to resolve.
type Chain struct {
	Links []ChainLink
}

// PendingTrigger represents a trigger effect waiting to be placed on a chain.
type PendingTrigger struct {
	Card       *CardInstance
	Effect     *CardEffect
	Controller int
}

// startChain creates a new chain with the given card/effect as CL1.
func (d *Duel) startChain(card *CardInstance, effect *CardEffect, player int, targets []*CardInstance) error {
	d.State.Chain = &Chain{}
	return d.addToChain(card, effect, player, targets)
}

// addToChain appends a new link to the existing chain.
func (d *Duel) addToChain(card *CardInstance, effect *CardEffect, player int, targets []*CardInstance) error {
	gs := d.State
	if gs.Chain == nil {
		return fmt.Errorf("no active chain to add to")
	}

	index := len(gs.Chain.Links) + 1
	link := ChainLink{
		Index:      index,
		Card:       card,
		Effect:     effect,
		Controller: player,
		Targets:    targets,
	}
	gs.Chain.Links = append(gs.Chain.Links, link)

	d.log(log.NewChainLinkEvent(gs.Turn, gs.Phase.String(), player, card.Card.Name, index))

	return nil
}

// resolveChain resolves the chain in LIFO order (last link resolves first).
func (d *Duel) resolveChain() error {
	gs := d.State
	if gs.Chain == nil || len(gs.Chain.Links) == 0 {
		return nil
	}

	// Resolve in reverse order (LIFO)
	for i := len(gs.Chain.Links) - 1; i >= 0; i-- {
		if gs.Over {
			break
		}
		link := gs.Chain.Links[i]
		d.log(log.NewChainResolveEvent(gs.Turn, gs.Phase.String(), link.Controller, link.Card.Card.Name, link.Index))

		if link.Effect.Resolve != nil {
			if err := link.Effect.Resolve(d, link.Card, link.Controller, link.Targets); err != nil {
				return err
			}
		}

		// Post-resolution: send normal programs/traps to scrapheap (not continuous)
		d.handlePostResolution(link)

		if gs.Over {
			break
		}
	}

	gs.Chain = nil
	d.recalculateContinuousEffects()
	return nil
}

// handlePostResolution handles cleanup after a chain link resolves.
// Normal programs and non-continuous traps go to the scrapheap.
func (d *Duel) handlePostResolution(link ChainLink) {
	card := link.Card
	gs := d.State

	// Only move to scrapheap if card is still on the field (wasn't already destroyed during resolution)
	if card.Zone != ZoneTech {
		return // already moved (destroyed, etc.)
	}

	switch card.Card.CardType {
	case CardTypeProgram:
		if card.Card.ProgramSub == ProgramContinuous || card.Card.ProgramSub == ProgramEquip || card.Card.ProgramSub == ProgramOS {
			return // stays on field
		}
		// Normal and quick-play programs go to scrapheap after resolving
		gs.Players[card.Controller].RemoveFromTech(card)
		gs.Players[card.Owner].SendToScrapheap(card)
		d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), card.Owner, card.Card.Name, "resolved"))
	case CardTypeTrap:
		if card.Card.TrapSub == TrapContinuous {
			return // stays on field
		}
		// Normal and counter traps go to scrapheap after resolving
		gs.Players[card.Controller].RemoveFromTech(card)
		gs.Players[card.Owner].SendToScrapheap(card)
		d.log(log.NewSendToScrapheapEvent(gs.Turn, gs.Phase.String(), card.Owner, card.Card.Name, "resolved"))
	}
}

// canChainWith checks if a new execution speed can chain to the top of the current chain.
func canChainWith(topSS, newSS ExecSpeed) bool {
	// Must be >= the top link's execution speed
	if newSS < topSS {
		return false
	}
	// ES3 can only be responded to by ES3
	if topSS == ExecSpeed3 && newSS < ExecSpeed3 {
		return false
	}
	return true
}

// topChainExecSpeed returns the execution speed of the top chain link, or 0 if no chain.
func (d *Duel) topChainExecSpeed() ExecSpeed {
	gs := d.State
	if gs.Chain == nil || len(gs.Chain.Links) == 0 {
		return 0
	}
	topLink := gs.Chain.Links[len(gs.Chain.Links)-1]
	return topLink.Effect.ExecSpeed
}
