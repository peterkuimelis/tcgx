package mcp

import (
	"context"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	tcgxnet "github.com/peterkuimelis/tcgx/internal/net"
)

// activeSession is the singleton game session (one per stdio process).
var activeSession *GameSession

// decksFile is the path to the decks YAML file, set by main.
var decksFile string

// port is the TCP port for the human player connection, set by main.
var port string

// SetDecksFile sets the path to the decks YAML file.
func SetDecksFile(path string) {
	decksFile = path
}

// SetPort sets the TCP port for the human player connection.
func SetPort(p string) {
	port = p
}

// RegisterTools adds all game tools to the MCP server.
func RegisterTools(s *server.MCPServer) {
	s.AddTool(startGameTool(), handleStartGame)
	s.AddTool(takeActionTool(), handleTakeAction)
	s.AddTool(selectCardsTool(), handleSelectCards)
	s.AddTool(answerYesNoTool(), handleAnswerYesNo)
	s.AddTool(getGameStateTool(), handleGetGameState)
}

// --- Tool definitions ---

func startGameTool() mcp.Tool {
	return mcp.NewTool("start_game",
		mcp.WithDescription("Start a new GOAT TCG duel. Returns the initial game state and first pending decision. "+
			"The human player connects via `tcgx join --addr localhost:<port> --deck N` in a separate terminal. "+
			"This call blocks until the human connects."),
		mcp.WithNumber("claude_deck", mcp.Required(), mcp.Description("Deck number for Claude (1-indexed from decks.yaml)")),
		mcp.WithNumber("claude_player", mcp.Required(), mcp.Description("Which player Claude is: 0 = goes first, 1 = goes second")),
	)
}

func takeActionTool() mcp.Tool {
	return mcp.NewTool("take_action",
		mcp.WithDescription("Choose an action from the pending action list. Use this when the pending decision type is 'choose_action'."),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("0-based index of the action to take from the actions list")),
	)
}

func selectCardsTool() mcp.Tool {
	return mcp.NewTool("select_cards",
		mcp.WithDescription("Select cards from the pending candidates list. Use this when the pending decision type is 'choose_cards'."),
		mcp.WithString("indices", mcp.Required(), mcp.Description("Space-separated 0-based indices of cards to select (e.g. '0 2 3'), or empty string for no selection")),
	)
}

func answerYesNoTool() mcp.Tool {
	return mcp.NewTool("answer_yes_no",
		mcp.WithDescription("Answer a yes/no question. Use this when the pending decision type is 'choose_yes_no'."),
		mcp.WithBoolean("answer", mcp.Required(), mcp.Description("true for yes, false for no")),
	)
}

func getGameStateTool() mcp.Tool {
	return mcp.NewTool("get_game_state",
		mcp.WithDescription("Get the current game state, accumulated events, and pending decision without submitting a response. Read-only."),
	)
}

// --- Tool handlers ---

func handleStartGame(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if activeSession != nil {
		return mcp.NewToolResultError("A game is already running. Only one game at a time is supported."), nil
	}

	claudeDeck := request.GetInt("claude_deck", 0)
	claudePlayer := request.GetInt("claude_player", 0)

	if claudeDeck < 1 {
		return mcp.NewToolResultError("claude_deck must be >= 1"), nil
	}
	if claudePlayer != 0 && claudePlayer != 1 {
		return mcp.NewToolResultError("claude_player must be 0 or 1"), nil
	}

	sess, err := NewGameSession(decksFile, claudeDeck, claudePlayer, port)
	if err != nil {
		return mcp.NewToolResultErrorf("Failed to start game: %v", err), nil
	}

	activeSession = sess

	resp, err := sess.waitForPending()
	if err != nil {
		return mcp.NewToolResultErrorf("Error waiting for first decision: %v", err), nil
	}

	resp.Port = port

	return mcp.NewToolResultText(respondJSON(resp)), nil
}

func handleTakeAction(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if activeSession == nil {
		return mcp.NewToolResultError("No game is running. Use start_game first."), nil
	}

	sess := activeSession
	pending := sess.currentPending
	if pending == nil {
		return mcp.NewToolResultError("No pending decision."), nil
	}
	if pending.Player != sess.claudePlayer {
		return mcp.NewToolResultError("Waiting for human player to respond via their terminal."), nil
	}
	if pending.Type != DecisionChooseAction {
		return mcp.NewToolResultErrorf("Wrong tool: pending decision is '%s', not 'choose_action'. Use the correct tool.", pending.Type), nil
	}

	index := request.GetInt("index", -1)
	if index < 0 || index >= len(pending.Actions) {
		return mcp.NewToolResultErrorf("Invalid index %d. Must be 0-%d.", index, len(pending.Actions)-1), nil
	}

	sess.claudeCtrl.responseCh <- ActionResponse{Index: index}

	resp, err := sess.waitForPending()
	if err != nil {
		return mcp.NewToolResultErrorf("Error waiting for next decision: %v", err), nil
	}

	if resp.GameOver {
		activeSession = nil
	}

	return mcp.NewToolResultText(respondJSON(resp)), nil
}

func handleSelectCards(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if activeSession == nil {
		return mcp.NewToolResultError("No game is running. Use start_game first."), nil
	}

	sess := activeSession
	pending := sess.currentPending
	if pending == nil {
		return mcp.NewToolResultError("No pending decision."), nil
	}
	if pending.Player != sess.claudePlayer {
		return mcp.NewToolResultError("Waiting for human player to respond via their terminal."), nil
	}
	if pending.Type != DecisionChooseCards {
		return mcp.NewToolResultErrorf("Wrong tool: pending decision is '%s', not 'choose_cards'. Use the correct tool.", pending.Type), nil
	}

	indicesStr := request.GetString("indices", "")
	var indices []int
	if strings.TrimSpace(indicesStr) != "" {
		parts := strings.Fields(indicesStr)
		for _, p := range parts {
			idx, err := strconv.Atoi(p)
			if err != nil {
				return mcp.NewToolResultErrorf("Invalid index '%s': must be an integer.", p), nil
			}
			if idx < 0 || idx >= len(pending.Candidates) {
				return mcp.NewToolResultErrorf("Index %d out of range. Must be 0-%d.", idx, len(pending.Candidates)-1), nil
			}
			indices = append(indices, idx)
		}
	}

	if len(indices) < pending.Min {
		return mcp.NewToolResultErrorf("Must select at least %d card(s), got %d.", pending.Min, len(indices)), nil
	}
	if len(indices) > pending.Max {
		return mcp.NewToolResultErrorf("Must select at most %d card(s), got %d.", pending.Max, len(indices)), nil
	}

	sess.claudeCtrl.responseCh <- CardsResponse{Indices: indices}

	resp, err := sess.waitForPending()
	if err != nil {
		return mcp.NewToolResultErrorf("Error waiting for next decision: %v", err), nil
	}

	if resp.GameOver {
		activeSession = nil
	}

	return mcp.NewToolResultText(respondJSON(resp)), nil
}

func handleAnswerYesNo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if activeSession == nil {
		return mcp.NewToolResultError("No game is running. Use start_game first."), nil
	}

	sess := activeSession
	pending := sess.currentPending
	if pending == nil {
		return mcp.NewToolResultError("No pending decision."), nil
	}
	if pending.Player != sess.claudePlayer {
		return mcp.NewToolResultError("Waiting for human player to respond via their terminal."), nil
	}
	if pending.Type != DecisionChooseYesNo {
		return mcp.NewToolResultErrorf("Wrong tool: pending decision is '%s', not 'choose_yes_no'. Use the correct tool.", pending.Type), nil
	}

	answer := request.GetBool("answer", false)

	sess.claudeCtrl.responseCh <- YesNoResponse{Answer: answer}

	resp, err := sess.waitForPending()
	if err != nil {
		return mcp.NewToolResultErrorf("Error waiting for next decision: %v", err), nil
	}

	if resp.GameOver {
		activeSession = nil
	}

	return mcp.NewToolResultText(respondJSON(resp)), nil
}

func handleGetGameState(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if activeSession == nil {
		return mcp.NewToolResultError("No game is running. Use start_game first."), nil
	}

	sess := activeSession
	events := sess.drainEvents()

	sess.mu.Lock()
	gameOver := sess.gameOver
	winner := sess.winner
	result := sess.result
	sess.mu.Unlock()

	resp := &ToolResponse{
		Events:   events,
		GameOver: gameOver,
		Winner:   winner,
		Result:   result,
	}

	if gameOver {
		if sess.currentPending != nil {
			resp.State = sess.currentPending.State
		}
	} else if sess.duel != nil {
		// Build a fresh state view from Claude's perspective
		resp.State = tcgxnet.BuildStateView(sess.duel.State, sess.claudePlayer)
		if sess.currentPending != nil {
			if sess.currentPending.Player != sess.claudePlayer {
				resp.Pending = &PendingView{
					Type:      DecisionChooseAction,
					ForPlayer: "human",
				}
			} else {
				resp.Pending = &PendingView{
					Type:       sess.currentPending.Type,
					ForPlayer:  "claude",
					Actions:    sess.currentPending.Actions,
					Prompt:     sess.currentPending.Prompt,
					Candidates: sess.currentPending.Candidates,
					Min:        sess.currentPending.Min,
					Max:        sess.currentPending.Max,
				}
			}
		}
	}

	// Ensure events is never null in JSON
	if resp.Events == nil {
		resp.Events = []tcgxnet.EventView{}
	}

	return mcp.NewToolResultText(respondJSON(resp)), nil
}
