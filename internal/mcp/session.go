package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	tcgxnet "github.com/peterkuimelis/tcgx/internal/net"

	"github.com/peterkuimelis/tcgx/internal/game"
	"github.com/peterkuimelis/tcgx/internal/log"

	stdnet "net"
)

// DecisionType identifies what kind of decision the game engine is waiting for.
type DecisionType string

const (
	DecisionChooseAction DecisionType = "choose_action"
	DecisionChooseCards  DecisionType = "choose_cards"
	DecisionChooseYesNo  DecisionType = "choose_yes_no"
	DecisionGameOver     DecisionType = "game_over"
)

// PendingDecision represents a decision the game engine is waiting for.
type PendingDecision struct {
	Type       DecisionType         `json:"type"`
	Player     int                  `json:"player"`
	State      *tcgxnet.StateView   `json:"state"`
	Actions    []tcgxnet.ActionView `json:"actions,omitempty"`
	Prompt     string               `json:"prompt,omitempty"`
	Candidates []tcgxnet.CardView   `json:"candidates,omitempty"`
	Min        int                  `json:"min,omitempty"`
	Max        int                  `json:"max,omitempty"`
}

// Response types sent back from MCP tools to controllers.

type ActionResponse struct {
	Index int
}

type CardsResponse struct {
	Indices []int
}

type YesNoResponse struct {
	Answer bool
}

// ToolResponse is the JSON envelope returned by all MCP tools.
type ToolResponse struct {
	Events   []tcgxnet.EventView `json:"events"`
	State    *tcgxnet.StateView  `json:"state,omitempty"`
	Pending  *PendingView        `json:"pending,omitempty"`
	GameOver bool                `json:"game_over"`
	Winner   int                 `json:"winner,omitempty"`
	Result   string              `json:"result,omitempty"`
	Port     string              `json:"port,omitempty"`
}

// PendingView is the pending decision as presented in the tool response JSON.
type PendingView struct {
	Type       DecisionType         `json:"type"`
	ForPlayer  string               `json:"for_player"`
	Actions    []tcgxnet.ActionView `json:"actions,omitempty"`
	Prompt     string               `json:"prompt,omitempty"`
	Candidates []tcgxnet.CardView   `json:"candidates,omitempty"`
	Min        int                  `json:"min,omitempty"`
	Max        int                  `json:"max,omitempty"`
}

// GameSession holds the state of a single MCP game session.
type GameSession struct {
	duel         *game.Duel
	claudeCtrl   *MCPController
	humanCtrl    *tcgxnet.NetworkController
	claudePlayer int

	listener  stdnet.Listener
	humanConn stdnet.Conn

	pendingCh      chan *PendingDecision
	currentPending *PendingDecision

	mu       sync.Mutex
	events   []tcgxnet.EventView
	gameOver bool
	winner   int
	result   string
}

// NewGameSession creates a new game session. It starts a TCP listener,
// waits for the human player to connect via `tcgx join`, then starts the duel.
func NewGameSession(decksFile string, claudeDeck, claudePlayer int, port string) (*GameSession, error) {
	claudeDeckName, claudeCards, err := game.DeckByNumber(decksFile, claudeDeck)
	if err != nil {
		return nil, fmt.Errorf("load claude deck: %w", err)
	}
	_ = claudeDeckName

	// Start TCP listener for human player
	ln, err := stdnet.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("listen on port %s: %w", port, err)
	}

	// Accept one connection (blocks until human runs `tcgx join`)
	conn, err := ln.Accept()
	if err != nil {
		ln.Close()
		return nil, fmt.Errorf("accept: %w", err)
	}

	// Read join message to get human's deck choice
	dec := json.NewDecoder(conn)
	var joinMsg tcgxnet.ClientMessage
	if err := dec.Decode(&joinMsg); err != nil {
		conn.Close()
		ln.Close()
		return nil, fmt.Errorf("read join message: %w", err)
	}
	humanDeck := joinMsg.DeckNumber
	if humanDeck == 0 {
		humanDeck = 2
	}

	humanDeckName, humanCards, err := game.DeckByNumber(decksFile, humanDeck)
	if err != nil {
		conn.Close()
		ln.Close()
		return nil, fmt.Errorf("load human deck: %w", err)
	}
	_ = humanDeckName

	sess := &GameSession{
		claudePlayer: claudePlayer,
		pendingCh:    make(chan *PendingDecision, 1),
		winner:       -1,
		listener:     ln,
		humanConn:    conn,
	}

	humanPlayer := 1 - claudePlayer
	sess.claudeCtrl = NewMCPController(claudePlayer, sess)
	sess.humanCtrl = tcgxnet.NewNetworkController(conn, humanPlayer)

	// Assign decks to player indices
	var deck0, deck1 []*game.Card
	var ctrl0, ctrl1 game.PlayerController
	if claudePlayer == 0 {
		deck0 = claudeCards
		deck1 = humanCards
		ctrl0 = sess.claudeCtrl
		ctrl1 = sess.humanCtrl
	} else {
		deck0 = humanCards
		deck1 = claudeCards
		ctrl0 = sess.humanCtrl
		ctrl1 = sess.claudeCtrl
	}

	cfg := game.DuelConfig{
		Deck0:  deck0,
		Deck1:  deck1,
		Logger: log.NewMemoryLogger(),
	}

	sess.duel = game.NewDuel(cfg, ctrl0, ctrl1)

	// Start the duel in a goroutine
	go func() {
		winner, err := sess.duel.Run(context.Background())
		if err != nil {
			sess.mu.Lock()
			sess.gameOver = true
			sess.result = fmt.Sprintf("error: %v", err)
			sess.mu.Unlock()
		}

		result := sess.duel.State.Result
		if result == "" {
			result = fmt.Sprintf("Game over. Winner: player %d", winner)
		}

		// Notify human over TCP
		_ = sess.humanCtrl.SendGameOver(winner, result)

		// Clean up TCP resources
		sess.humanConn.Close()
		sess.listener.Close()

		// Notify Claude via pending channel
		sess.pendingCh <- &PendingDecision{
			Type:   DecisionGameOver,
			Player: winner,
			State:  tcgxnet.BuildStateView(sess.duel.State, sess.claudePlayer),
		}

		sess.mu.Lock()
		sess.gameOver = true
		sess.winner = winner
		sess.result = result
		sess.mu.Unlock()
	}()

	return sess, nil
}

// appendEvent adds an event to the session's event log. Thread-safe.
func (s *GameSession) appendEvent(ev tcgxnet.EventView) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
}

// drainEvents returns all accumulated events and clears the buffer.
func (s *GameSession) drainEvents() []tcgxnet.EventView {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := s.events
	s.events = nil
	return events
}

// waitForPending blocks until the next decision arrives from the game engine,
// then builds a ToolResponse with accumulated events + the pending decision.
func (s *GameSession) waitForPending() (*ToolResponse, error) {
	pending := <-s.pendingCh
	s.currentPending = pending

	events := s.drainEvents()

	resp := &ToolResponse{
		Events: events,
	}

	if pending.Type == DecisionGameOver {
		s.mu.Lock()
		resp.GameOver = true
		resp.Winner = s.winner
		resp.Result = s.result
		s.mu.Unlock()
		resp.State = pending.State
		resp.Pending = nil
		return resp, nil
	}

	resp.State = pending.State
	resp.Pending = &PendingView{
		Type:       pending.Type,
		ForPlayer:  s.playerLabel(pending.Player),
		Actions:    pending.Actions,
		Prompt:     pending.Prompt,
		Candidates: pending.Candidates,
		Min:        pending.Min,
		Max:        pending.Max,
	}

	return resp, nil
}

// playerLabel returns "claude" or "human" for the given player index.
func (s *GameSession) playerLabel(player int) string {
	if player == s.claudePlayer {
		return "claude"
	}
	return "human"
}

// respondJSON marshals a ToolResponse to a JSON string.
func respondJSON(resp *ToolResponse) string {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Sprintf(`{"error": "marshal error: %v"}`, err)
	}
	return string(data)
}
