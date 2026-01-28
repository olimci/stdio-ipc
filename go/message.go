package ipc

import "encoding/json"

type wireError struct {
	Message string `json:"message"`
}

type wireMessage struct {
	Type    string          `json:"type"`
	ID      uint64          `json:"id"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *wireError      `json:"error,omitempty"`
}

type response struct {
	payload json.RawMessage
	err     error
}
