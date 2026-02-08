package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tcgxnet "github.com/peterkuimelis/tcgx/internal/net"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "host":
		runHost(os.Args[2:])
	case "join":
		runJoin(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  tcgx host [--deck N] [--port P] [--decks FILE]")
	fmt.Println("  tcgx join [--deck N] [--addr ADDR] ")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  host    Start a game server and play as Player 1")
	fmt.Println("  join    Connect to a game server and play as Player 2")
}

func runHost(args []string) {
	fs := flag.NewFlagSet("host", flag.ExitOnError)
	deck := fs.Int("deck", 1, "deck number to use (from decks.yaml)")
	port := fs.String("port", "9000", "TCP port to listen on")
	decksFile := fs.String("decks", "decks.yaml", "path to decks file")
	fs.Parse(args)

	srv := &tcgxnet.Server{
		DeckFile: *decksFile,
		Port:     *port,
		HostDeck: *deck,
	}

	if err := srv.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runJoin(args []string) {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	deck := fs.Int("deck", 2, "deck number to use (from decks.yaml)")
	addr := fs.String("addr", "localhost:9000", "server address to connect to")
	fs.Parse(args)

	if err := tcgxnet.Connect(context.Background(), *addr, *deck); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
