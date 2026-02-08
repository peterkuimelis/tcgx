package web

import (
	"github.com/peterkuimelis/tcgx/internal/game"
	"gopkg.in/yaml.v3"
)

func parseDeckFileYAML(data []byte) (game.DeckFile, error) {
	var df game.DeckFile
	err := yaml.Unmarshal(data, &df)
	return df, err
}
