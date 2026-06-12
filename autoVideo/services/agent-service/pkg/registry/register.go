package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func Start(ctx context.Context, gatewayAddr, name, selfAddr string) {
	go func() {
		heartbeat(gatewayAddr, name, selfAddr)
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				heartbeat(gatewayAddr, name, selfAddr)
			}
		}
	}()
}

func heartbeat(gatewayAddr, name, selfAddr string) {
	body, _ := json.Marshal(map[string]string{"name": name, "addr": selfAddr})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/_internal/register", gatewayAddr),
		bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}
