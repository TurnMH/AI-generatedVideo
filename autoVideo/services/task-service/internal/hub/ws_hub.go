package hub

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Client represents a single WebSocket connection.
type Client struct {
	conn      *websocket.Conn
	send      chan []byte
	taskID    uint64 // 0 means subscribe by project
	projectID uint64
	userID    uint64
	hub       *Hub
}

// Hub maintains active WebSocket clients and broadcasts messages.
type Hub struct {
	mu             sync.RWMutex
	clients        map[*Client]bool
	taskClients    map[uint64][]*Client
	projectClients map[uint64][]*Client
	logger         *zap.Logger
}

// WSMessage is the envelope sent to WebSocket clients.
type WSMessage struct {
	Type      string      `json:"type"`
	TaskID    uint64      `json:"task_id"`
	Progress  int         `json:"progress,omitempty"`
	Message   string      `json:"message,omitempty"`
	Status    string      `json:"status,omitempty"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// NewHub —— 创建 WebSocket Hub 实例，管理所有客户端连接
// NewHub creates a Hub with the given logger.
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:        make(map[*Client]bool),
		taskClients:    make(map[uint64][]*Client),
		projectClients: make(map[uint64][]*Client),
		logger:         logger,
	}
}

// Run —— Hub 主循环占位（当前直推实现无需循环），保留接口兼容
// Run is a no-op in this direct-push implementation; kept for API compatibility.
func (h *Hub) Run() {}

// Register —— 将客户端注册到 Hub 的任务/项目索引中
// Register adds a client to the hub's indices.
func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true
	if client.taskID != 0 {
		h.taskClients[client.taskID] = append(h.taskClients[client.taskID], client)
	}
	if client.projectID != 0 {
		h.projectClients[client.projectID] = append(h.projectClients[client.projectID], client)
	}
}

// Unregister —— 从 Hub 移除客户端并关闭其发送通道
// Unregister removes a client from the hub and closes its send channel.
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; !ok {
		return
	}
	delete(h.clients, client)
	close(client.send)

	if client.taskID != 0 {
		h.taskClients[client.taskID] = removeClient(h.taskClients[client.taskID], client)
		if len(h.taskClients[client.taskID]) == 0 {
			delete(h.taskClients, client.taskID)
		}
	}
	if client.projectID != 0 {
		h.projectClients[client.projectID] = removeClient(h.projectClients[client.projectID], client)
		if len(h.projectClients[client.projectID]) == 0 {
			delete(h.projectClients, client.projectID)
		}
	}
}

// BroadcastToTask —— 向订阅指定任务的所有 WebSocket 客户端推送消息
// BroadcastToTask pushes a message to all clients subscribed to the given task.
func (h *Hub) BroadcastToTask(taskID uint64, msg WSMessage) {
	msg.Timestamp = time.Now().UnixMilli()
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("ws marshal error", zap.Error(err))
		return
	}

	h.mu.RLock()
	clients := make([]*Client, len(h.taskClients[taskID]))
	copy(clients, h.taskClients[taskID])
	h.mu.RUnlock()

	for _, c := range clients {
		select {
		case c.send <- data:
		default:
			h.Unregister(c)
		}
	}
}

// BroadcastToProject —— 向订阅指定项目的所有 WebSocket 客户端推送消息
// BroadcastToProject pushes a message to all clients subscribed to the given project.
func (h *Hub) BroadcastToProject(projectID uint64, msg WSMessage) {
	msg.Timestamp = time.Now().UnixMilli()
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("ws marshal error", zap.Error(err))
		return
	}

	h.mu.RLock()
	clients := make([]*Client, len(h.projectClients[projectID]))
	copy(clients, h.projectClients[projectID])
	h.mu.RUnlock()

	for _, c := range clients {
		select {
		case c.send <- data:
		default:
			h.Unregister(c)
		}
	}
}

// ServeWS —— 注册客户端并启动其读写协程
// ServeWS registers the client and starts its read/write pumps.
func (h *Hub) ServeWS(taskID, projectID, userID uint64, conn *websocket.Conn) {
	client := &Client{
		conn:      conn,
		send:      make(chan []byte, 256),
		taskID:    taskID,
		projectID: projectID,
		userID:    userID,
		hub:       h,
	}
	h.Register(client)

	go client.writePump()
	go client.readPump()
}

// readPump —— 客户端读协程，维持连接心跳并丢弃入站数据
// readPump keeps the connection alive by handling pong frames and discarding inbound data.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Warn("ws read error", zap.Error(err))
			}
			break
		}
	}
}

// writePump —— 客户端写协程，将发送通道的消息写入 WebSocket 并定期发送 ping
// writePump drains the send channel and sends ping frames to keep the connection alive.
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
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			// flush any queued messages in the same write frame
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// removeClient —— 从客户端切片中移除指定客户端，返回新切片
// removeClient removes a single client from a slice.
func removeClient(clients []*Client, target *Client) []*Client {
	result := clients[:0]
	for _, c := range clients {
		if c != target {
			result = append(result, c)
		}
	}
	return result
}
