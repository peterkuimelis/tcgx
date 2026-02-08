package mcp

import (
	"context"

	"github.com/peterkuimelis/tcgx/internal/game"
	"github.com/peterkuimelis/tcgx/internal/log"
	"github.com/peterkuimelis/tcgx/internal/net"
)

// MCPController implements game.PlayerController by sending decisions
// to the MCP session's pending channel and blocking on a response channel.
type MCPController struct {
	player     int
	session    *GameSession
	responseCh chan any
}

// NewMCPController creates a controller for the given player.
func NewMCPController(player int, session *GameSession) *MCPController {
	return &MCPController{
		player:     player,
		session:    session,
		responseCh: make(chan any),
	}
}

// ChooseAction implements game.PlayerController.
func (c *MCPController) ChooseAction(ctx context.Context, state *game.GameState, actions []game.Action) (game.Action, error) {
	var views []net.ActionView
	for i, a := range actions {
		views = append(views, net.ActionView{Index: i, Desc: a.String()})
	}

	c.session.pendingCh <- &PendingDecision{
		Type:    DecisionChooseAction,
		Player:  c.player,
		State:   net.BuildStateView(state, c.player),
		Actions: views,
	}

	resp := <-c.responseCh
	ar := resp.(ActionResponse)

	if ar.Index < 0 || ar.Index >= len(actions) {
		return actions[0], nil
	}
	return actions[ar.Index], nil
}

// ChooseCards implements game.PlayerController.
func (c *MCPController) ChooseCards(ctx context.Context, state *game.GameState, prompt string, candidates []*game.CardInstance, min, max int) ([]*game.CardInstance, error) {
	var views []net.CardView
	for i, card := range candidates {
		cv := net.CardView{Index: i, Name: card.Card.Name}
		if card.Card.CardType == game.CardTypeAgent {
			cv.ATK = card.CurrentATK()
			cv.DEF = card.CurrentDEF()
		}
		views = append(views, cv)
	}

	c.session.pendingCh <- &PendingDecision{
		Type:       DecisionChooseCards,
		Player:     c.player,
		State:      net.BuildStateView(state, c.player),
		Prompt:     prompt,
		Candidates: views,
		Min:        min,
		Max:        max,
	}

	resp := <-c.responseCh
	cr := resp.(CardsResponse)

	var result []*game.CardInstance
	for _, idx := range cr.Indices {
		if idx >= 0 && idx < len(candidates) {
			result = append(result, candidates[idx])
		}
	}
	return result, nil
}

// ChooseYesNo implements game.PlayerController.
func (c *MCPController) ChooseYesNo(ctx context.Context, state *game.GameState, prompt string) (bool, error) {
	c.session.pendingCh <- &PendingDecision{
		Type:   DecisionChooseYesNo,
		Player: c.player,
		State:  net.BuildStateView(state, c.player),
		Prompt: prompt,
	}

	resp := <-c.responseCh
	yr := resp.(YesNoResponse)
	return yr.Answer, nil
}

// Notify implements game.PlayerController.
// Only the Claude controller appends events to avoid duplicates.
func (c *MCPController) Notify(ctx context.Context, event log.GameEvent) error {
	if c.player == c.session.claudePlayer {
		c.session.appendEvent(net.EventView{
			Turn:    event.Turn,
			Phase:   event.Phase,
			Player:  event.Player,
			Type:    event.Type.String(),
			Card:    event.Card,
			Details: event.Details,
		})
	}
	return nil
}
