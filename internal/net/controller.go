package net

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/peterkuimelis/tcgx/internal/game"
	"github.com/peterkuimelis/tcgx/internal/log"
)

// NetworkController implements game.PlayerController over a TCP connection.
type NetworkController struct {
	conn   net.Conn
	enc    *json.Encoder
	dec    *json.Decoder
	player int // which player this controller is (0 or 1)
	mu     sync.Mutex
}

// NewNetworkController creates a new controller for the given connection.
func NewNetworkController(conn net.Conn, player int) *NetworkController {
	return &NetworkController{
		conn:   conn,
		enc:    json.NewEncoder(conn),
		dec:    json.NewDecoder(conn),
		player: player,
	}
}

// BuildStateView creates a StateView from the perspective of the given player.
func BuildStateView(state *game.GameState, player int) *StateView {
	me := player
	opp := 1 - me

	myPlayer := state.Players[me]
	oppPlayer := state.Players[opp]

	sv := &StateView{
		Turn:       state.Turn,
		Phase:      state.Phase.String(),
		IsYourTurn: state.TurnPlayer == me,
	}

	// My view
	sv.You = PlayerView{
		HP:             myPlayer.HP,
		HandCount:      len(myPlayer.Hand),
		ScrapheapCount: len(myPlayer.Scrapheap),
		DeckCount:      myPlayer.DeckCount(),
	}
	// Hand names (visible to you)
	for _, c := range myPlayer.Hand {
		sv.You.Hand = append(sv.You.Hand, c.Card.Name)
	}
	// My agents
	for i := 0; i < 5; i++ {
		sv.You.Agents[i] = AgentZoneView(myPlayer.AgentZones[i], true)
	}
	// My Tech
	for i := 0; i < 5; i++ {
		sv.You.TechZone[i] = TechZoneView(myPlayer.TechZones[i], true)
	}
	if myPlayer.OS != nil {
		fv := TechZoneView(myPlayer.OS, true)
		sv.You.OS = &fv
	}

	// Opponent view
	sv.Opponent = PlayerView{
		HP:             oppPlayer.HP,
		HandCount:      len(oppPlayer.Hand),
		ScrapheapCount: len(oppPlayer.Scrapheap),
		DeckCount:      oppPlayer.DeckCount(),
	}
	// Opponent agents (face-down info hidden)
	for i := 0; i < 5; i++ {
		sv.Opponent.Agents[i] = AgentZoneView(oppPlayer.AgentZones[i], false)
	}
	// Opponent Tech
	for i := 0; i < 5; i++ {
		sv.Opponent.TechZone[i] = TechZoneView(oppPlayer.TechZones[i], false)
	}
	if oppPlayer.OS != nil {
		fv := TechZoneView(oppPlayer.OS, false)
		sv.Opponent.OS = &fv
	}

	return sv
}

// buildStateView creates a StateView from the perspective of this controller's player.
func (nc *NetworkController) buildStateView(state *game.GameState) *StateView {
	return BuildStateView(state, nc.player)
}

// AgentZoneView creates a ZoneView for an agent zone.
func AgentZoneView(ci *game.CardInstance, isOwner bool) ZoneView {
	if ci == nil {
		return ZoneView{Empty: true}
	}
	if ci.Face == game.FaceDown {
		if isOwner {
			return ZoneView{
				FaceDown: true,
				Name:     ci.Card.Name,
				ATK:      ci.CurrentATK(),
				DEF:      ci.CurrentDEF(),
				Position: ci.Position.String(),
			}
		}
		return ZoneView{FaceDown: true, Position: ci.Position.String()}
	}
	return ZoneView{
		Name:     ci.Card.Name,
		ATK:      ci.CurrentATK(),
		DEF:      ci.CurrentDEF(),
		Position: ci.Position.String(),
	}
}

// TechZoneView creates a ZoneView for a tech zone.
func TechZoneView(ci *game.CardInstance, isOwner bool) ZoneView {
	if ci == nil {
		return ZoneView{Empty: true}
	}
	if ci.Face == game.FaceDown {
		if isOwner {
			return ZoneView{FaceDown: true, Name: ci.Card.Name}
		}
		return ZoneView{FaceDown: true}
	}
	return ZoneView{Name: ci.Card.Name}
}

// send sends a server message to the client. Must be called with mu held.
func (nc *NetworkController) send(msg ServerMessage) error {
	return nc.enc.Encode(msg)
}

// recv reads a client message. Must be called with mu held.
func (nc *NetworkController) recv() (ClientMessage, error) {
	var msg ClientMessage
	err := nc.dec.Decode(&msg)
	return msg, err
}

// ChooseAction implements game.PlayerController.
func (nc *NetworkController) ChooseAction(ctx context.Context, state *game.GameState, actions []game.Action) (game.Action, error) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	var views []ActionView
	for i, a := range actions {
		views = append(views, ActionView{Index: i, Desc: a.String()})
	}

	msg := ServerMessage{
		Type:    "choose_action",
		Actions: views,
		State:   nc.buildStateView(state),
	}
	if err := nc.send(msg); err != nil {
		return game.Action{}, fmt.Errorf("send choose_action: %w", err)
	}

	resp, err := nc.recv()
	if err != nil {
		return game.Action{}, fmt.Errorf("recv action: %w", err)
	}

	if resp.Index < 0 || resp.Index >= len(actions) {
		return actions[0], nil // fallback to first action
	}
	return actions[resp.Index], nil
}

// ChooseCards implements game.PlayerController.
func (nc *NetworkController) ChooseCards(ctx context.Context, state *game.GameState, prompt string, candidates []*game.CardInstance, min, max int) ([]*game.CardInstance, error) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	var views []CardView
	for i, c := range candidates {
		cv := CardView{Index: i, Name: c.Card.Name}
		if c.Card.CardType == game.CardTypeAgent {
			cv.ATK = c.CurrentATK()
			cv.DEF = c.CurrentDEF()
		}
		views = append(views, cv)
	}

	msg := ServerMessage{
		Type:       "choose_cards",
		Prompt:     prompt,
		Candidates: views,
		Min:        min,
		Max:        max,
		State:      nc.buildStateView(state),
	}
	if err := nc.send(msg); err != nil {
		return nil, fmt.Errorf("send choose_cards: %w", err)
	}

	resp, err := nc.recv()
	if err != nil {
		return nil, fmt.Errorf("recv cards: %w", err)
	}

	var result []*game.CardInstance
	for _, idx := range resp.Indices {
		if idx >= 0 && idx < len(candidates) {
			result = append(result, candidates[idx])
		}
	}
	return result, nil
}

// ChooseYesNo implements game.PlayerController.
func (nc *NetworkController) ChooseYesNo(ctx context.Context, state *game.GameState, prompt string) (bool, error) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	msg := ServerMessage{
		Type:   "choose_yes_no",
		Prompt: prompt,
		State:  nc.buildStateView(state),
	}
	if err := nc.send(msg); err != nil {
		return false, fmt.Errorf("send choose_yes_no: %w", err)
	}

	resp, err := nc.recv()
	if err != nil {
		return false, fmt.Errorf("recv yes_no: %w", err)
	}

	return resp.Answer, nil
}

// SendGameOver sends a game_over message to the client.
func (nc *NetworkController) SendGameOver(winner int, result string) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.send(ServerMessage{Type: "game_over", Winner: winner, Result: result})
}

// Notify implements game.PlayerController.
func (nc *NetworkController) Notify(ctx context.Context, event log.GameEvent) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	msg := ServerMessage{
		Type: "notify",
		Event: &EventView{
			Turn:    event.Turn,
			Phase:   event.Phase,
			Player:  event.Player,
			Type:    event.Type.String(),
			Card:    event.Card,
			Details: event.Details,
		},
	}
	return nc.send(msg)
}
