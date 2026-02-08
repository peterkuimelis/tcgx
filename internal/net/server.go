package net

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/peterkuimelis/tcgx/internal/game"
	"github.com/peterkuimelis/tcgx/internal/log"
)

// Server hosts a duel between two TCP clients.
type Server struct {
	DeckFile string
	Port     string
	HostDeck int // host's deck number (1-indexed)
}

// Run starts the server, waits for a client to join, then runs the duel.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", ":"+s.Port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	fmt.Printf("Waiting for opponent on port %s...\n", s.Port)

	// Accept exactly one connection (the joiner)
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	defer conn.Close()

	fmt.Printf("Opponent connected from %s\n", conn.RemoteAddr())

	// Read the joiner's deck choice
	dec := json.NewDecoder(conn)
	var joinMsg ClientMessage
	if err := dec.Decode(&joinMsg); err != nil {
		return fmt.Errorf("read join message: %w", err)
	}
	joinerDeck := joinMsg.DeckNumber
	if joinerDeck == 0 {
		joinerDeck = 2
	}

	fmt.Printf("Opponent chose deck %d\n", joinerDeck)

	// Load decks
	hostDeckName, hostCards, err := game.DeckByNumber(s.DeckFile, s.HostDeck)
	if err != nil {
		return fmt.Errorf("load host deck: %w", err)
	}
	joinerDeckName, joinerCards, err := game.DeckByNumber(s.DeckFile, joinerDeck)
	if err != nil {
		return fmt.Errorf("load joiner deck: %w", err)
	}

	fmt.Printf("Host: %s (%d cards)\n", hostDeckName, len(hostCards))
	fmt.Printf("Joiner: %s (%d cards)\n", joinerDeckName, len(joinerCards))

	// Create a pipe for the host's local connection
	hostConn, hostServerConn := net.Pipe()

	// Create controllers
	// Player 0 = host, Player 1 = joiner
	hostCtrl := NewNetworkController(hostServerConn, 0)
	joinerCtrl := NewNetworkController(conn, 1)

	// Create duel
	logger := log.NewTextLogger(os.Stdout)
	duel := game.NewDuel(game.DuelConfig{
		Deck0:  hostCards,
		Deck1:  joinerCards,
		Logger: logger,
	}, hostCtrl, joinerCtrl)

	// Run the host's local REPL in a goroutine
	errCh := make(chan error, 2)
	go func() {
		client := &Client{conn: hostConn, playerName: "P1"}
		errCh <- client.RunREPL(ctx)
	}()

	// Run the duel
	go func() {
		winner, err := duel.Run(ctx)
		if err != nil {
			errCh <- fmt.Errorf("duel error: %w", err)
			return
		}

		// Send game_over to both players
		gameOverMsg := ServerMessage{
			Type:   "game_over",
			Winner: winner,
			Result: duel.State.Result,
		}

		// Send to joiner
		joinerCtrl.mu.Lock()
		_ = joinerCtrl.send(gameOverMsg)
		joinerCtrl.mu.Unlock()

		// Send to host
		hostCtrl.mu.Lock()
		_ = hostCtrl.send(gameOverMsg)
		hostCtrl.mu.Unlock()

		errCh <- nil
	}()

	// Wait for either the duel or the REPL to finish
	err = <-errCh
	return err
}
