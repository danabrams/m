package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/anthropics/m/internal/store"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// WSMessage represents a message sent over WebSocket.
type WSMessage struct {
	Type  string      `json:"type"`
	Event *EventDTO   `json:"event,omitempty"`
	State string      `json:"state,omitempty"`
}

// EventDTO is the JSON representation of an event.
type EventDTO struct {
	ID        string          `json:"id"`
	Seq       int64           `json:"seq"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	CreatedAt int64           `json:"created_at"`
}

// ClientMessage represents a message from client to server.
type ClientMessage struct {
	Type string `json:"type"`
}

// Client represents a WebSocket client connection.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	runID  string
}

// Hub maintains the set of active clients per run.
type Hub struct {
	mu         sync.RWMutex
	clients    map[string]map[*Client]bool // runID -> clients
	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMessage
}

// BroadcastMessage carries an event to broadcast to a run's clients.
type BroadcastMessage struct {
	RunID   string
	Message []byte
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage, 256),
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.runID] == nil {
				h.clients[client.runID] = make(map[*Client]bool)
			}
			h.clients[client.runID][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.runID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.runID)
					}
				}
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			clients := h.clients[msg.RunID]
			for client := range clients {
				select {
				case client.send <- msg.Message:
				default:
					// Buffer full, drop client
					go func(c *Client) {
						h.unregister <- c
					}(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastEvent sends an event to all clients watching a run.
func (h *Hub) BroadcastEvent(event *store.Event) {
	dto := eventToDTO(event)
	msg := WSMessage{
		Type:  "event",
		Event: dto,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("websocket: marshal event: %v", err)
		return
	}

	h.broadcast <- &BroadcastMessage{
		RunID:   event.RunID,
		Message: data,
	}
}

// BroadcastState sends a state change to all clients watching a run.
func (h *Hub) BroadcastState(runID string, state store.RunState) {
	msg := WSMessage{
		Type:  "state",
		State: string(state),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("websocket: marshal state: %v", err)
		return
	}

	h.broadcast <- &BroadcastMessage{
		RunID:   runID,
		Message: data,
	}
}

// ClientCount returns the number of clients connected to a run.
func (h *Hub) ClientCount(runID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[runID])
}

func eventToDTO(e *store.Event) *EventDTO {
	dto := &EventDTO{
		ID:        e.ID,
		Seq:       e.Seq,
		Type:      e.Type,
		CreatedAt: e.CreatedAt.Unix(),
	}
	if e.Data != nil {
		dto.Data = json.RawMessage(*e.Data)
	} else {
		dto.Data = json.RawMessage("{}")
	}
	return dto
}

// handleEventsWS handles WebSocket connections for event streaming.
func (s *Server) handleEventsWS(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	if runID == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "missing run id")
		return
	}

	// Validate run exists
	run, err := s.store.GetRun(runID)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "not_found", "run not found")
		return
	}
	if err != nil {
		log.Printf("websocket: get run: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	// Parse from_seq query parameter
	var fromSeq int64
	if fromSeqStr := r.URL.Query().Get("from_seq"); fromSeqStr != "" {
		fromSeq, err = strconv.ParseInt(fromSeqStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "invalid from_seq parameter")
			return
		}
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket: upgrade: %v", err)
		return
	}

	client := &Client{
		hub:   s.hub,
		conn:  conn,
		send:  make(chan []byte, 256),
		runID: runID,
	}

	s.hub.register <- client

	// Send replay events
	events, err := s.store.ListEventsByRunSince(runID, fromSeq)
	if err != nil {
		log.Printf("websocket: list events: %v", err)
		conn.Close()
		return
	}

	for _, event := range events {
		dto := eventToDTO(event)
		msg := WSMessage{
			Type:  "event",
			Event: dto,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("websocket: marshal replay event: %v", err)
			continue
		}
		client.send <- data
	}

	// Send current state if run is active
	if run.IsActive() {
		stateMsg := WSMessage{
			Type:  "state",
			State: string(run.State),
		}
		data, err := json.Marshal(stateMsg)
		if err == nil {
			client.send <- data
		}
	}

	// Start read and write pumps
	go client.writePump()
	go client.readPump()
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket: read error: %v", err)
			}
			break
		}

		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		// Handle pong messages from client
		if msg.Type == "pong" {
			c.conn.SetReadDeadline(time.Now().Add(pongWait))
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			// Send ping as JSON message per API spec
			pingMsg, _ := json.Marshal(WSMessage{Type: "ping"})
			if err := c.conn.WriteMessage(websocket.TextMessage, pingMsg); err != nil {
				return
			}
		}
	}
}
