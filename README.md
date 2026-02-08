<p align="center">
  <img src="banner.png" alt="TCGX" width="600">
</p>

<p align="center">
  An MCP server that lets LLM agents play a trading card game against humans.
</p>

---

![Gameplay](gameplay.png)

## Overview

TCGX is a TCG duel simulator with a built-in [MCP](https://modelcontextprotocol.io/) server. It implements a full card game engine and exposes it as a set of MCP tools so that an AI agent like Claude can play against a human opponent in real time.

The human connects via a CLI over TCP. The AI connects via MCP over stdio. Both players see only their own perspective — no information leaks.

## How It Works

The game runs as a single Go process with two transport layers:

- **MCP (stdio)** — The AI agent calls tools like `take_action`, `select_cards`, and `answer_yes_no` to make decisions. A `get_game_state` tool provides the agent with a view of the board.
- **TCP** — The human player connects from a separate terminal and interacts through a JSON-based REPL.

The engine handles turn structure, summoning, combat, spell/trap activation, chain resolution, and win conditions. Games end when a player's LP hits 0 or they deck out.

## Setup

### Prerequisites

- Go 1.21+
- An MCP-compatible client (e.g. [Claude Code](https://docs.anthropic.com/en/docs/claude-code))

### Install

```bash
go build -o tcgx-mcp ./cmd/tcgx-mcp
go build -o tcgx-cli ./cmd/tcgx-cli
```

### Configure MCP

Add the server to your MCP client config (`.mcp.json`):

```json
{
  "mcpServers": {
    "tcgx": {
      "type": "stdio",
      "command": "go",
      "args": ["run", "./cmd/tcgx-mcp", "--decks", "decks.yaml"],
      "cwd": "/path/to/tcgx"
    }
  }
}
```

## Playing a Game

1. The AI agent calls `start_game` and picks a deck and player slot. The server starts listening on a TCP port (default 9999).

2. The human joins from another terminal:
   ```bash
   tcgx-cli join --addr localhost:9999 --deck 1
   ```

3. The game begins. Each player takes turns through their own transport — the AI through MCP tool calls, the human through the CLI.

## Game Basics

Players start with 8192 LP and draw from a deck of ~40 cards. On your turn you move through phases — Draw, Standby, Main, Battle, Main 2, End — summoning Agents to the field, activating Programs and Traps, and attacking your opponent.

Agents are your creatures. Stronger ones (level 5+) require tributing existing Agents to summon. Programs are one-shot or persistent effects. Traps are set face-down and spring on your opponent's moves. Combat compares ATK values, and damage flows through to LP.

## MCP Tools

| Tool | Description |
|------|-------------|
| `start_game` | Start a new duel, choose deck and player slot |
| `get_game_state` | Get the current board state from the agent's perspective |
| `take_action` | Choose an action from the pending action list |
| `select_cards` | Select cards from a list of candidates |
| `answer_yes_no` | Respond to a yes/no prompt |

## Decks

Decks are defined in `decks.yaml`. The repo ships with two built-in decks:

- **Deepnet Fury** — A control-oriented deck built around Neon Hydra Lord and the Undercity Grid
- **Cyberblaze** — An aggressive deck led by Infernal Plasma Emperor with burn effects

## License

MIT
