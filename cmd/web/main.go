package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/peterkuimelis/tcgx/internal/web"
)

func main() {
	port := flag.Int("port", 8080, "HTTP port to listen on")
	artDir := flag.String("art", "./card_art", "path to card art directory")
	decksFile := flag.String("decks", "decks.yaml", "path to decks YAML file")
	mappingFile := flag.String("mapping", "card_art_mapping.json", "path to card art mapping JSON")
	flag.Parse()

	srv, err := web.NewServer(*artDir, *decksFile, *mappingFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("tcgx web UI listening on http://localhost:%d", *port)
	if err := srv.ListenAndServe(addr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
