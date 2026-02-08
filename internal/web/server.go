package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/coder/websocket"
	"github.com/peterkuimelis/tcgx/internal/game"
)

//go:embed static
var staticFiles embed.FS

// CardInfo is the JSON representation of a card for the /api/cards endpoint.
type CardInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CardType    string `json:"cardType"`
	Level       int    `json:"level,omitempty"`
	Attribute   string `json:"attribute,omitempty"`
	AgentType   string `json:"agentType,omitempty"`
	ATK         int    `json:"atk,omitempty"`
	DEF         int    `json:"def,omitempty"`
	IsEffect    bool   `json:"isEffect,omitempty"`
	Subtype     string `json:"subtype,omitempty"`
	ArtPath     string `json:"artPath,omitempty"`
}

// DeckInfo is the JSON representation of a deck for the /api/decks endpoint.
type DeckInfo struct {
	Number int      `json:"number"`
	Name   string   `json:"name"`
	Cards  []string `json:"cards"`
}

// Server is the tcgx web UI server.
type Server struct {
	artDir     string
	decksFile  string
	artMapping map[string]string // card name → art file path
	mux        *http.ServeMux
}

// NewServer creates a new web server.
func NewServer(artDir, decksFile, mappingFile string) (*Server, error) {
	// Load art mapping
	artMapping := make(map[string]string)
	data, err := os.ReadFile(mappingFile)
	if err != nil {
		log.Printf("Warning: could not load art mapping: %v", err)
	} else {
		if err := json.Unmarshal(data, &artMapping); err != nil {
			log.Printf("Warning: could not parse art mapping: %v", err)
		}
	}

	s := &Server{
		artDir:     artDir,
		decksFile:  decksFile,
		artMapping: artMapping,
		mux:        http.NewServeMux(),
	}
	s.setupRoutes()
	return s, nil
}

func (s *Server) setupRoutes() {
	// Embedded static files
	staticFS, _ := fs.Sub(staticFiles, "static")

	// Serve index.html at root
	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		f, err := staticFS.Open("index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		io.Copy(w, f.(io.Reader))
	})

	// Static CSS/JS
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Card art from filesystem
	s.mux.Handle("GET /art/", http.StripPrefix("/art/", http.FileServer(http.Dir(s.artDir))))

	// API endpoints
	s.mux.HandleFunc("GET /api/cards", s.handleCards)
	s.mux.HandleFunc("GET /api/decks", s.handleDecks)

	// WebSocket proxy
	s.mux.HandleFunc("GET /ws", s.handleWebSocket)
}

func (s *Server) handleCards(w http.ResponseWriter, r *http.Request) {
	var cards []CardInfo
	for name, ctor := range game.CardRegistry {
		c := ctor()
		ci := CardInfo{
			Name:        name,
			Description: c.Description,
			Level:       c.Level,
			Attribute:   c.Attribute.String(),
			AgentType:   c.AgentType,
			ATK:         c.ATK,
			DEF:         c.DEF,
			IsEffect:    c.IsEffect,
		}
		switch c.CardType {
		case game.CardTypeAgent:
			ci.CardType = "Agent"
		case game.CardTypeProgram:
			ci.CardType = "Program"
			ci.Subtype = programSubtypeString(c.ProgramSub)
		case game.CardTypeTrap:
			ci.CardType = "Trap"
			ci.Subtype = trapSubtypeString(c.TrapSub)
		}
		// Art path: strip "card_art/" prefix since we serve from /art/
		if artPath, ok := s.artMapping[name]; ok {
			ci.ArtPath = "/art/" + strings.TrimPrefix(artPath, "card_art/")
		}
		cards = append(cards, ci)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cards)
}

func (s *Server) handleDecks(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(s.decksFile)
	if err != nil {
		http.Error(w, "could not read decks file", http.StatusInternalServerError)
		return
	}

	df, err := parseDeckFileYAML(data)
	if err != nil {
		http.Error(w, "could not parse decks file", http.StatusInternalServerError)
		return
	}

	var decks []DeckInfo
	for i, d := range df.Decks {
		di := DeckInfo{
			Number: i + 1,
			Name:   d.Name,
		}
		// Unique card names for display
		seen := make(map[string]bool)
		for _, c := range d.Cards {
			if !seen[c.Name] {
				di.Cards = append(di.Cards, c.Name)
				seen[c.Name] = true
			}
		}
		decks = append(decks, di)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(decks)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	wsConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow connections from any origin
	})
	if err != nil {
		log.Printf("WebSocket accept error: %v", err)
		return
	}
	defer wsConn.CloseNow()

	ctx := r.Context()

	// Read initial connect message from browser
	_, connectData, err := wsConn.Read(ctx)
	if err != nil {
		log.Printf("WebSocket read connect: %v", err)
		return
	}

	var connectMsg struct {
		Type       string `json:"type"`
		Addr       string `json:"addr"`
		DeckNumber int    `json:"deck_number"`
	}
	if err := json.Unmarshal(connectData, &connectMsg); err != nil || connectMsg.Type != "connect" {
		wsConn.Close(websocket.StatusPolicyViolation, "expected connect message")
		return
	}

	// Open TCP connection to game server
	tcpConn, err := net.Dial("tcp", connectMsg.Addr)
	if err != nil {
		errMsg, _ := json.Marshal(map[string]string{
			"type":   "error",
			"result": fmt.Sprintf("Could not connect to game server at %s: %v", connectMsg.Addr, err),
		})
		wsConn.Write(ctx, websocket.MessageText, errMsg)
		wsConn.Close(websocket.StatusNormalClosure, "connection failed")
		return
	}
	defer tcpConn.Close()

	// Send join message over TCP
	joinMsg, _ := json.Marshal(map[string]interface{}{
		"type":        "join",
		"deck_number": connectMsg.DeckNumber,
	})
	joinMsg = append(joinMsg, '\n')
	if _, err := tcpConn.Write(joinMsg); err != nil {
		log.Printf("TCP write join: %v", err)
		return
	}

	done := make(chan struct{})

	// TCP → WebSocket (server messages to browser)
	go func() {
		defer close(done)
		dec := json.NewDecoder(tcpConn)
		for {
			var msg json.RawMessage
			if err := dec.Decode(&msg); err != nil {
				if err != io.EOF {
					log.Printf("TCP read error: %v", err)
				}
				return
			}
			if err := wsConn.Write(ctx, websocket.MessageText, msg); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		}
	}()

	// WebSocket → TCP (browser responses to server)
	go func() {
		for {
			_, data, err := wsConn.Read(ctx)
			if err != nil {
				return
			}
			data = append(data, '\n')
			if _, err := tcpConn.Write(data); err != nil {
				log.Printf("TCP write error: %v", err)
				return
			}
		}
	}()

	<-done
	wsConn.Close(websocket.StatusNormalClosure, "game ended")
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

func programSubtypeString(sub game.ProgramSubtype) string {
	switch sub {
	case game.ProgramNormal:
		return "Normal"
	case game.ProgramQuickPlay:
		return "Quick-Play"
	case game.ProgramContinuous:
		return "Continuous"
	case game.ProgramEquip:
		return "Equip"
	case game.ProgramOS:
		return "OS"
	default:
		return ""
	}
}

func trapSubtypeString(sub game.TrapSubtype) string {
	switch sub {
	case game.TrapNormal:
		return "Normal"
	case game.TrapContinuous:
		return "Continuous"
	case game.TrapCounter:
		return "Counter"
	default:
		return ""
	}
}
