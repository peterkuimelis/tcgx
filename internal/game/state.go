package game

import (
	"fmt"
	"math/rand"
)

const (
	StartingHP      = 8192
	InitialHandSize = 5
	MaxHandSize     = 6
	AgentZoneCount  = 5
	TechZoneCount   = 5
)

// Player represents one player's entire state.
type Player struct {
	HP        int
	Deck      []*CardInstance // top of deck is last element (pop from end)
	Hand      []*CardInstance
	Scrapheap []*CardInstance
	Purged    []*CardInstance

	AgentZones [AgentZoneCount]*CardInstance
	TechZones  [TechZoneCount]*CardInstance
	OS         *CardInstance
}

// DeckCount returns the number of cards remaining in the deck.
func (p *Player) DeckCount() int {
	return len(p.Deck)
}

// HandCount returns the number of cards in hand.
func (p *Player) HandCount() int {
	return len(p.Hand)
}

// DrawCard removes the top card from the deck and adds it to the hand.
// Returns the drawn card, or nil if the deck is empty.
func (p *Player) DrawCard() *CardInstance {
	if len(p.Deck) == 0 {
		return nil
	}
	card := p.Deck[len(p.Deck)-1]
	p.Deck = p.Deck[:len(p.Deck)-1]
	card.Zone = ZoneHand
	card.ZoneIndex = len(p.Hand)
	p.Hand = append(p.Hand, card)
	return card
}

// RemoveFromHand removes a card from the hand by instance ID.
func (p *Player) RemoveFromHand(card *CardInstance) {
	for i, c := range p.Hand {
		if c.ID == card.ID {
			p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
			return
		}
	}
}

// SendToScrapheap moves a card to the scrapheap.
func (p *Player) SendToScrapheap(card *CardInstance) {
	card.Zone = ZoneScrapheap
	card.ZoneIndex = len(p.Scrapheap)
	card.Face = FaceUp
	card.EquippedTo = nil
	card.Equips = nil
	p.Scrapheap = append(p.Scrapheap, card)
}

// FreeAgentZone returns the index of the first empty agent zone, or -1.
func (p *Player) FreeAgentZone() int {
	for i, z := range p.AgentZones {
		if z == nil {
			return i
		}
	}
	return -1
}

// FreeAgentZones returns all empty agent zone indices.
func (p *Player) FreeAgentZones() []int {
	var zones []int
	for i, z := range p.AgentZones {
		if z == nil {
			zones = append(zones, i)
		}
	}
	return zones
}

// AgentCount returns the number of agents on the field.
func (p *Player) AgentCount() int {
	count := 0
	for _, z := range p.AgentZones {
		if z != nil {
			count++
		}
	}
	return count
}

// Agents returns all non-nil agents on the field.
func (p *Player) Agents() []*CardInstance {
	var result []*CardInstance
	for _, z := range p.AgentZones {
		if z != nil {
			result = append(result, z)
		}
	}
	return result
}

// FaceUpATKAgents returns all face-up ATK position agents.
func (p *Player) FaceUpATKAgents() []*CardInstance {
	var result []*CardInstance
	for _, z := range p.AgentZones {
		if z != nil && z.Face == FaceUp && z.Position == PositionATK {
			result = append(result, z)
		}
	}
	return result
}

// RemoveAgent removes a agent from its zone (sets zone to nil).
func (p *Player) RemoveAgent(card *CardInstance) {
	for i, z := range p.AgentZones {
		if z != nil && z.ID == card.ID {
			p.AgentZones[i] = nil
			return
		}
	}
}

// PlaceAgent places a card in the specified agent zone.
func (p *Player) PlaceAgent(card *CardInstance, zone int) {
	p.AgentZones[zone] = card
	card.Zone = ZoneAgent
	card.ZoneIndex = zone
}

// FreeTechZone returns the index of the first empty tech zone, or -1.
func (p *Player) FreeTechZone() int {
	for i, z := range p.TechZones {
		if z == nil {
			return i
		}
	}
	return -1
}

// FreeTechZones returns all empty tech zone indices.
func (p *Player) FreeTechZones() []int {
	var zones []int
	for i, z := range p.TechZones {
		if z == nil {
			zones = append(zones, i)
		}
	}
	return zones
}

// PlaceTech places a card in the specified tech zone.
func (p *Player) PlaceTech(card *CardInstance, zone int) {
	p.TechZones[zone] = card
	card.Zone = ZoneTech
	card.ZoneIndex = zone
}

// RemoveFromTech removes a card from its tech zone.
func (p *Player) RemoveFromTech(card *CardInstance) {
	for i, z := range p.TechZones {
		if z != nil && z.ID == card.ID {
			p.TechZones[i] = nil
			return
		}
	}
}

// TechCards returns all non-nil cards in the tech zone.
func (p *Player) TechCards() []*CardInstance {
	var result []*CardInstance
	for _, z := range p.TechZones {
		if z != nil {
			result = append(result, z)
		}
	}
	return result
}

// FaceDownTech returns face-down set cards in the tech zone.
func (p *Player) FaceDownTech() []*CardInstance {
	var result []*CardInstance
	for _, z := range p.TechZones {
		if z != nil && z.Face == FaceDown {
			result = append(result, z)
		}
	}
	return result
}

// FaceUpAgents returns all face-up agents on the field.
func (p *Player) FaceUpAgents() []*CardInstance {
	var result []*CardInstance
	for _, z := range p.AgentZones {
		if z != nil && z.Face == FaceUp {
			result = append(result, z)
		}
	}
	return result
}

// ShuffleDeck randomizes the deck order.
func (p *Player) ShuffleDeck() {
	rand.Shuffle(len(p.Deck), func(i, j int) {
		p.Deck[i], p.Deck[j] = p.Deck[j], p.Deck[i]
	})
}

// SummonEventInfo holds information about a summon that just occurred, for trigger matching.
type SummonEventInfo struct {
	Card   *CardInstance
	Player int
}

// --- GameState ---

// GameState holds the complete state of a duel.
type GameState struct {
	Players    [2]*Player
	Turn       int // 1-based turn counter
	TurnPlayer int // 0 or 1: whose turn it is
	Phase      Phase
	BattleStep BattleStep

	// Per-turn flags
	NormalSummonUsed bool

	// Battle tracking
	CurrentAttacker *CardInstance
	CurrentTarget   *CardInstance // nil for direct attack

	// Chain system
	Chain            *Chain
	PendingTriggers  []PendingTrigger
	LastSummonEvent  *SummonEventInfo // info about most recent summon for trigger matching
	InResponseWindow bool             // true when inside openResponseWindow

	// ID counter for card instances
	nextID int

	// Game result
	Winner int // 0, 1, or -1 (no winner yet)
	Over   bool
	Result string
}

// NewGameState creates a fresh duel state.
func NewGameState() *GameState {
	gs := &GameState{
		Players: [2]*Player{
			{HP: StartingHP},
			{HP: StartingHP},
		},
		Turn:       0,
		TurnPlayer: 0,
		Phase:      PhaseNone,
		Winner:     -1,
	}
	return gs
}

// NextID generates a unique card instance ID.
func (gs *GameState) NextID() int {
	gs.nextID++
	return gs.nextID
}

// Opponent returns the index of the other player.
func (gs *GameState) Opponent(player int) int {
	return 1 - player
}

// CurrentPlayer returns the Player struct for the turn player.
func (gs *GameState) CurrentPlayer() *Player {
	return gs.Players[gs.TurnPlayer]
}

// OpponentPlayer returns the Player struct for the non-turn player.
func (gs *GameState) OpponentPlayer() *Player {
	return gs.Players[gs.Opponent(gs.TurnPlayer)]
}

// CheckWinCondition checks if either player's HP has hit 0.
// Returns true if the game is over.
func (gs *GameState) CheckWinCondition() bool {
	p0Dead := gs.Players[0].HP <= 0
	p1Dead := gs.Players[1].HP <= 0

	if p0Dead && p1Dead {
		gs.Over = true
		gs.Winner = -1
		gs.Result = "Draw — both players' HP reached 0"
		return true
	}
	if p0Dead {
		gs.Over = true
		gs.Winner = 1
		gs.Result = fmt.Sprintf("P2 wins — P1's HP reached 0")
		return true
	}
	if p1Dead {
		gs.Over = true
		gs.Winner = 0
		gs.Result = fmt.Sprintf("P1 wins — P2's HP reached 0")
		return true
	}
	return false
}

// ResetTurnFlags resets per-turn tracking for a new turn.
func (gs *GameState) ResetTurnFlags() {
	gs.NormalSummonUsed = false
	gs.CurrentAttacker = nil
	gs.CurrentTarget = nil

	// Reset per-turn flags on all agents for both players
	for p := 0; p < 2; p++ {
		for _, m := range gs.Players[p].AgentZones {
			if m != nil {
				m.AttackedThisTurn = false
				m.PositionChangedThisTurn = false
			}
		}
	}
}

// CreateCardInstance creates a CardInstance from a Card definition, assigned to a player.
func (gs *GameState) CreateCardInstance(card *Card, owner int) *CardInstance {
	return &CardInstance{
		Card:       card,
		ID:         gs.NextID(),
		Owner:      owner,
		Controller: owner,
		Face:       FaceDown,
		Zone:       ZoneDeck,
		Counters:   make(map[string]int),
	}
}
