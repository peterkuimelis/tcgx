package net

// Message types for the JSON protocol over TCP.

// --- Server → Client messages ---

// ServerMessage is the envelope for all server-to-client messages.
type ServerMessage struct {
	Type string `json:"type"`

	// For "notify"
	Event *EventView `json:"event,omitempty"`

	// For "choose_action"
	Actions []ActionView `json:"actions,omitempty"`
	State   *StateView   `json:"state,omitempty"`

	// For "choose_cards"
	Prompt     string     `json:"prompt,omitempty"`
	Candidates []CardView `json:"candidates,omitempty"`
	Min        int        `json:"min,omitempty"`
	Max        int        `json:"max,omitempty"`

	// For "game_over"
	Winner int    `json:"winner,omitempty"`
	Result string `json:"result,omitempty"`
}

// EventView is a simplified game event for the client.
type EventView struct {
	Turn    int    `json:"turn"`
	Phase   string `json:"phase"`
	Player  int    `json:"player"`
	Type    string `json:"type"`
	Card    string `json:"card,omitempty"`
	Details string `json:"details"`
}

// ActionView is a numbered action choice.
type ActionView struct {
	Index int    `json:"index"`
	Desc  string `json:"desc"`
}

// CardView describes a card candidate for selection.
type CardView struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	ATK   int    `json:"atk,omitempty"`
	DEF   int    `json:"def,omitempty"`
}

// StateView is the game state from one player's perspective.
type StateView struct {
	You        PlayerView `json:"you"`
	Opponent   PlayerView `json:"opponent"`
	Turn       int        `json:"turn"`
	Phase      string     `json:"phase"`
	IsYourTurn bool       `json:"is_your_turn"`
}

// PlayerView shows one side of the board.
type PlayerView struct {
	HP             int         `json:"hp"`
	HandCount      int         `json:"hand_count"`
	Hand           []string    `json:"hand,omitempty"` // card names (only for "you")
	Agents         [5]ZoneView `json:"agents"`
	TechZone       [5]ZoneView `json:"tech_zone"`
	OS             *ZoneView   `json:"os,omitempty"`
	ScrapheapCount int         `json:"scrapheap_count"`
	DeckCount      int         `json:"deck_count"`
}

// ZoneView describes a single zone on the field.
type ZoneView struct {
	Empty    bool   `json:"empty,omitempty"`
	FaceDown bool   `json:"face_down,omitempty"`
	Name     string `json:"name,omitempty"`
	ATK      int    `json:"atk,omitempty"`
	DEF      int    `json:"def,omitempty"`
	Position string `json:"position,omitempty"` // "ATK" or "DEF"
}

// --- Client → Server messages ---

// ClientMessage is the envelope for all client-to-server messages.
type ClientMessage struct {
	Type string `json:"type"`

	// For "action"
	Index int `json:"index,omitempty"`

	// For "cards"
	Indices []int `json:"indices,omitempty"`

	// For "yes_no"
	Answer bool `json:"answer,omitempty"`

	// For "join" (initial handshake)
	DeckNumber int `json:"deck_number,omitempty"`
}
