// Package codec provides a JSON-based gRPC codec that replaces the default
// protobuf codec. This allows hand-crafted Go structs (without protoc generation)
// to be used as gRPC message types, using JSON tags for serialization.
//
// Import this package with _ in cmd/main.go to register the codec via init().
package codec

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

// init —— 注册 JSON 编解码器以替代默认的 protobuf 编解码器
func init() {
	// Override the default "proto" codec with JSON so hand-crafted message
	// structs can be used without protoc-generated ProtoReflect() methods.
	encoding.RegisterCodec(JSONCodec{})
}

// JSONCodec implements grpc/encoding.Codec using encoding/json.
type JSONCodec struct{}

// Name —— 返回编解码器名称，用于覆盖默认 gRPC 编解码器
// Name returns "proto" to override the default gRPC codec.
func (JSONCodec) Name() string { return "proto" }

// Marshal —— 将对象序列化为 JSON 字节数组
func (JSONCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal —— 将 JSON 字节数组反序列化为对象
func (JSONCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
