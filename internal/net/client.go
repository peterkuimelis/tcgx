package net

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// Client connects to a game server and provides a terminal REPL.
type Client struct {
	conn       net.Conn
	playerName string // "P1" or "P2"
}

// Connect connects to a server, sends the deck choice, and runs the REPL.
func Connect(ctx context.Context, addr string, deckNumber int) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	// Send join message with deck choice
	enc := json.NewEncoder(conn)
	if err := enc.Encode(ClientMessage{Type: "join", DeckNumber: deckNumber}); err != nil {
		return fmt.Errorf("send join: %w", err)
	}

	fmt.Println("Connected! Waiting for game to start...")

	client := &Client{conn: conn, playerName: "P2"}
	return client.RunREPL(ctx)
}

// RunREPL reads server messages and handles them interactively.
func (c *Client) RunREPL(ctx context.Context) error {
	dec := json.NewDecoder(c.conn)
	enc := json.NewEncoder(c.conn)
	reader := bufio.NewReader(os.Stdin)

	for {
		var msg ServerMessage
		if err := dec.Decode(&msg); err != nil {
			return fmt.Errorf("read message: %w", err)
		}

		switch msg.Type {
		case "notify":
			c.renderEvent(msg.Event)

		case "choose_action":
			c.renderState(msg.State)
			c.renderActions(msg.Actions)
			idx := c.readChoice(reader, len(msg.Actions))
			if err := enc.Encode(ClientMessage{Type: "action", Index: idx}); err != nil {
				return fmt.Errorf("send action: %w", err)
			}

		case "choose_cards":
			if msg.State != nil {
				c.renderState(msg.State)
			}
			c.renderCardChoice(msg.Prompt, msg.Candidates, msg.Min, msg.Max)
			indices := c.readCardIndices(reader, len(msg.Candidates), msg.Min, msg.Max)
			if err := enc.Encode(ClientMessage{Type: "cards", Indices: indices}); err != nil {
				return fmt.Errorf("send cards: %w", err)
			}

		case "choose_yes_no":
			fmt.Printf("\n%s (y/n): ", msg.Prompt)
			answer := c.readYesNo(reader)
			if err := enc.Encode(ClientMessage{Type: "yes_no", Answer: answer}); err != nil {
				return fmt.Errorf("send yes_no: %w", err)
			}

		case "game_over":
			fmt.Println()
			fmt.Println("═══════════════════════════════════")
			fmt.Println("          GAME OVER")
			fmt.Println("═══════════════════════════════════")
			fmt.Println(msg.Result)
			fmt.Println("═══════════════════════════════════")
			return nil
		}
	}
}

func (c *Client) renderEvent(ev *EventView) {
	if ev == nil {
		return
	}
	// Format like the TextLogger
	phase := ev.Phase
	if phase == "" {
		phase = "          "
	}
	for len(phase) < 16 {
		phase += " "
	}
	fmt.Printf("T%-2d %s| %s\n", ev.Turn, phase, ev.Details)
}

func (c *Client) renderState(sv *StateView) {
	if sv == nil {
		return
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════╗")

	// Opponent info
	opp := sv.Opponent
	fmt.Printf("║  OPPONENT (HP: %d)  Hand: %d  Deck: %d  Scrapheap: %d\n",
		opp.HP, opp.HandCount, opp.DeckCount, opp.ScrapheapCount)

	// Opponent agents
	fmt.Printf("║  Agent:   ")
	for i := 0; i < 5; i++ {
		fmt.Printf("%s ", formatAgentZone(opp.Agents[i], false))
	}
	fmt.Println()

	// Opponent Tech
	fmt.Printf("║  Tech:     ")
	for i := 0; i < 5; i++ {
		fmt.Printf("%s ", formatTechZone(opp.TechZone[i]))
	}
	fmt.Println()

	if opp.OS != nil && !opp.OS.Empty {
		fmt.Printf("║  OS:   %s\n", formatTechZone(*opp.OS))
	}

	fmt.Println("║──────────────────────────────────────────────────────")

	// My Tech
	you := sv.You
	if you.OS != nil && !you.OS.Empty {
		fmt.Printf("║  OS:   %s\n", formatTechZone(*you.OS))
	}

	fmt.Printf("║  Tech:     ")
	for i := 0; i < 5; i++ {
		fmt.Printf("%s ", formatTechZone(you.TechZone[i]))
	}
	fmt.Println()

	// My agents
	fmt.Printf("║  Agent:   ")
	for i := 0; i < 5; i++ {
		fmt.Printf("%s ", formatAgentZone(you.Agents[i], true))
	}
	fmt.Println()

	fmt.Printf("║  YOU (HP: %d)  Hand: %d  Deck: %d  Scrapheap: %d\n",
		you.HP, you.HandCount, you.DeckCount, you.ScrapheapCount)
	fmt.Println("╚══════════════════════════════════════════════════════╝")

	turnInfo := fmt.Sprintf("Turn %d | %s", sv.Turn, sv.Phase)
	if sv.IsYourTurn {
		turnInfo += " | Your turn"
	} else {
		turnInfo += " | Opponent's turn"
	}
	fmt.Println(turnInfo)

	// Show hand
	if len(you.Hand) > 0 {
		fmt.Printf("\nHand: ")
		for i, name := range you.Hand {
			fmt.Printf("[%d] %s  ", i+1, name)
		}
		fmt.Println()
	}
}

func formatAgentZone(zv ZoneView, isOwner bool) string {
	if zv.Empty {
		return "[ ]"
	}
	if zv.FaceDown {
		if isOwner {
			return fmt.Sprintf("[SET:%s]", zv.Name)
		}
		return "[SET]"
	}
	if zv.Position == "ATK" {
		return fmt.Sprintf("[%s ATK/%d]", zv.Name, zv.ATK)
	}
	return fmt.Sprintf("[%s DEF/%d]", zv.Name, zv.DEF)
}

func formatTechZone(zv ZoneView) string {
	if zv.Empty {
		return "[ ]"
	}
	if zv.FaceDown {
		return "[SET]"
	}
	return fmt.Sprintf("[%s]", zv.Name)
}

func (c *Client) renderActions(actions []ActionView) {
	fmt.Println("\nActions:")
	for _, a := range actions {
		fmt.Printf("  %d) %s\n", a.Index+1, a.Desc)
	}
}

func (c *Client) readChoice(reader *bufio.Reader, count int) int {
	for {
		fmt.Print("> ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > count {
			fmt.Printf("Enter a number between 1 and %d\n", count)
			continue
		}
		return n - 1 // convert to 0-indexed
	}
}

func (c *Client) renderCardChoice(prompt string, candidates []CardView, min, max int) {
	fmt.Printf("\n%s (select %d", prompt, min)
	if max != min {
		fmt.Printf("-%d", max)
	}
	fmt.Println(")")
	for _, cv := range candidates {
		if cv.ATK > 0 || cv.DEF > 0 {
			fmt.Printf("  %d) %s (ATK %d / DEF %d)\n", cv.Index+1, cv.Name, cv.ATK, cv.DEF)
		} else {
			fmt.Printf("  %d) %s\n", cv.Index+1, cv.Name)
		}
	}
}

func (c *Client) readCardIndices(reader *bufio.Reader, count, min, max int) []int {
	for {
		fmt.Print("> ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		parts := strings.Fields(line)

		if len(parts) < min || len(parts) > max {
			fmt.Printf("Enter %d-%d numbers separated by spaces\n", min, max)
			continue
		}

		var indices []int
		valid := true
		for _, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil || n < 1 || n > count {
				fmt.Printf("Each number must be between 1 and %d\n", count)
				valid = false
				break
			}
			indices = append(indices, n-1) // convert to 0-indexed
		}
		if valid {
			return indices
		}
	}
}

func (c *Client) readYesNo(reader *bufio.Reader) bool {
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		switch line {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			fmt.Print("Enter y or n: ")
		}
	}
}
