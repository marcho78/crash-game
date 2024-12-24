package server

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, implement proper origin checks
	},
}

type Client struct {
	conn *websocket.Conn
	send chan WSMessage
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func (s *GameServer) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan WSMessage, 256),
	}

	s.clients.Store(client, true)

	go client.writePump()
	go client.readPump(s)
}

func (client *Client) writePump() {
	defer func() {
		client.conn.Close()
	}()

	for message := range client.send {
		err := client.conn.WriteJSON(message)
		if err != nil {
			return
		}
	}
}

func (client *Client) readPump(s *GameServer) {
	defer func() {
		s.clients.Delete(client)
		client.conn.Close()
	}()

	for {
		var message WSMessage
		err := client.conn.ReadJSON(&message)
		if err != nil {
			break
		}

		// Handle incoming messages if needed
	}
}

func (s *GameServer) broadcastMessage(message WSMessage) {
	s.clients.Range(func(key, _ interface{}) bool {
		if client, ok := key.(*Client); ok {
			select {
			case client.send <- message:
			default:
				s.clients.Delete(client)
				close(client.send)
			}
		}
		return true
	})
}
