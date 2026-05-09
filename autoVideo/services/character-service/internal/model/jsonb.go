package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONB provides PostgreSQL JSONB support via map.
type JSONB map[string]interface{}

// Value —— 将 JSONB 序列化为数据库驱动可存储的 JSON 字符串
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

// Scan —— 从数据库读取值并反序列化为 JSONB map
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = JSONB{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("unsupported JSONB scan type")
	}
	return json.Unmarshal(bytes, j)
}
