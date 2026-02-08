package game

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DeckFile represents the top-level YAML structure.
type DeckFile struct {
	Decks []DeckEntry `yaml:"decks"`
}

// DeckEntry represents a single deck in the YAML file.
type DeckEntry struct {
	Name  string      `yaml:"name"`
	Cards []CardEntry `yaml:"cards"`
}

// CardEntry represents a card and its count in a deck.
type CardEntry struct {
	Name  string `yaml:"name"`
	Count int    `yaml:"count"`
}

// ParseDeckFile parses a YAML deck file and returns a map of deck name → card slice.
func ParseDeckFile(path string) (map[string][]*Card, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var df DeckFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		return nil, fmt.Errorf("parse deck YAML: %w", err)
	}

	decks := make(map[string][]*Card)
	for _, deck := range df.Decks {
		var cards []*Card
		for _, entry := range deck.Cards {
			for i := 0; i < entry.Count; i++ {
				cards = append(cards, LookupCard(entry.Name))
			}
		}
		decks[deck.Name] = cards
	}

	return decks, nil
}

// DeckByNumber returns the Nth deck (1-indexed) from the deck file.
func DeckByNumber(path string, n int) (string, []*Card, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}

	var df DeckFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		return "", nil, fmt.Errorf("parse deck YAML: %w", err)
	}

	if n < 1 || n > len(df.Decks) {
		return "", nil, fmt.Errorf("deck %d not found (have %d decks)", n, len(df.Decks))
	}

	deck := df.Decks[n-1]
	var cards []*Card
	for _, entry := range deck.Cards {
		for i := 0; i < entry.Count; i++ {
			cards = append(cards, LookupCard(entry.Name))
		}
	}

	return deck.Name, cards, nil
}
