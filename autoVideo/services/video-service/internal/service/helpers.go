package service

import "encoding/json"

// parseJSON —— 将 JSON 字节数组反序列化到目标结构体
func parseJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
