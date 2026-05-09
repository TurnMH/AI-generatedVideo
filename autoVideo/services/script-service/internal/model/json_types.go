package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONMap represents a JSONB map column
type JSONMap map[string]interface{}

// Value —— 将 JSONMap 序列化为 JSON 字符串，用于数据库写入
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

// Scan —— 从数据库值反序列化为 JSONMap，支持 []byte 和 string 类型
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = JSONMap{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("unsupported type for JSONMap")
	}
	return json.Unmarshal(bytes, j)
}

// JSONSlice represents a JSONB array column
type JSONSlice []interface{}

// Value —— 将 JSONSlice 序列化为 JSON 字符串，用于数据库写入
func (j JSONSlice) Value() (driver.Value, error) {
	if j == nil {
		return "[]", nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

// Scan —— 从数据库值反序列化为 JSONSlice，支持 []byte 和 string 类型
func (j *JSONSlice) Scan(value interface{}) error {
	if value == nil {
		*j = JSONSlice{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("unsupported type for JSONSlice")
	}
	return json.Unmarshal(bytes, j)
}
