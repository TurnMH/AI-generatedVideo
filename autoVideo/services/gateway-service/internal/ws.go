package internal

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // CORS handled by gateway
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// proxyWebSocket upgrades the client connection to WebSocket and bidirectionally
// pipes frames between the client and the upstream WebSocket server.
func proxyWebSocket(w http.ResponseWriter, r *http.Request, target *url.URL, timeout time.Duration, logger *zap.Logger) {
	// Build upstream WebSocket URL (ws:// or wss://)
	upURL := *target
	switch upURL.Scheme {
	case "https":
		upURL.Scheme = "wss"
	default:
		upURL.Scheme = "ws"
	}
	upURL.Path = r.URL.Path
	upURL.RawQuery = r.URL.RawQuery

	// Forward request headers to upstream (except Upgrade/Connection which gorilla sets)
	reqHeader := http.Header{}
	for k, vv := range r.Header {
		switch k {
		case "Upgrade", "Connection", "Sec-Websocket-Key",
			"Sec-Websocket-Version", "Sec-Websocket-Extensions":
			// gorilla handles these
		default:
			reqHeader[k] = vv
		}
	}

	dialTimeout := timeout
	if dialTimeout <= 0 {
		dialTimeout = 10 * time.Second
	}
	dialer := websocket.Dialer{HandshakeTimeout: dialTimeout}

	upstream, resp, err := dialer.Dial(upURL.String(), reqHeader)
	if err != nil {
		status := http.StatusBadGateway
		if resp != nil {
			status = resp.StatusCode
		}
		logger.Warn("ws upstream dial failed", zap.String("url", upURL.String()), zap.Error(err))
		http.Error(w, fmt.Sprintf(`{"code":%d,"message":"ws upstream unavailable"}`, status), status)
		return
	}
	defer upstream.Close()

	client, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Warn("ws client upgrade failed", zap.Error(err))
		return
	}
	defer client.Close()

	errc := make(chan error, 2)

	// client → upstream
	go func() {
		for {
			mt, msg, err := client.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := upstream.WriteMessage(mt, msg); err != nil {
				errc <- err
				return
			}
		}
	}()

	// upstream → client
	go func() {
		for {
			mt, msg, err := upstream.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := client.WriteMessage(mt, msg); err != nil {
				errc <- err
				return
			}
		}
	}()

	if err := <-errc; err != nil && !isWSCloseErr(err) {
		logger.Debug("ws session ended", zap.Error(err))
	}
	// drain the second goroutine's error silently
	go func() { <-errc }()
	_ = io.Discard
}

func isWSCloseErr(err error) bool {
	return websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
	)
}

// isWebSocketUpgrade returns true when the request is an HTTP upgrade to WebSocket.
func isWebSocketUpgrade(r *http.Request) bool {
	return websocket.IsWebSocketUpgrade(r)
}
