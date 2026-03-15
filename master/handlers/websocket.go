package handlers

import (
	"net/http"
	"sync"
	"time"

	"gpu-optimizer/master/models"
	"gpu-optimizer/master/worker"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WebSocketHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan models.ProgressUpdate
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	manager    *worker.Manager
	ticker     *time.Ticker
	mu         sync.RWMutex
}

func NewWebSocketHub(manager *worker.Manager) *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan models.ProgressUpdate),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		manager:    manager,
		ticker:     nil,
	}
}

func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if err := client.WriteJSON(message); err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *WebSocketHub) StartBroadcasting() {
	h.ticker = time.NewTicker(200 * time.Millisecond)
	go func() {
		for range h.ticker.C {
			h.broadcast <- h.manager.GetProgress()
		}
	}()
}

func (h *WebSocketHub) StopBroadcasting() {
	if h.ticker != nil {
		h.ticker.Stop()
		h.ticker = nil
	}
}

func (h *WebSocketHub) HandleWebSocket(c *gin.Context) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	h.register <- conn

	go func() {
		defer func() { h.unregister <- conn }()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}
