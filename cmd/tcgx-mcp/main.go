package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	tcgxmcp "github.com/peterkuimelis/tcgx/internal/mcp"
)

func main() {
	decks := flag.String("decks", "decks.yaml", "path to decks YAML file")
	port := flag.String("port", "9999", "TCP port for human player connection")
	flag.Parse()

	tcgxmcp.SetDecksFile(*decks)
	tcgxmcp.SetPort(*port)

	s := server.NewMCPServer("tcgx", "1.0.0")
	tcgxmcp.RegisterTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
