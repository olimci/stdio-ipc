package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// Request is the envelope used by Router to dispatch payloads by type.
type Request struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// NewRequest creates a routed request envelope with a JSON-encoded payload.
func NewRequest(kind string, payload any) (Request, error) {
	if kind == "" {
		return Request{}, errors.New("ipc: request type is empty")
	}
	raw, err := marshalPayload(payload)
	if err != nil {
		return Request{}, err
	}
	return Request{Type: kind, Payload: raw}, nil
}

type route struct {
	handle func(ctx context.Context, payload json.RawMessage) (any, error)
}

// Router dispatches requests by type and returns a combined Handler.
type Router struct {
	mu       sync.RWMutex
	routes   map[string]route
	fallback func(ctx context.Context, kind string, payload json.RawMessage) (any, error)
}

// NewRouter creates an empty router.
func NewRouter() *Router {
	return &Router{
		routes: make(map[string]route),
	}
}

// Handle registers a handler that receives the raw JSON payload for the kind.
func (r *Router) Handle(kind string, fn func(context.Context, json.RawMessage) (any, error)) error {
	if kind == "" {
		return errors.New("ipc: route kind is empty")
	}
	if fn == nil {
		return errors.New("ipc: nil handler")
	}

	r.mu.Lock()
	r.routes[kind] = route{handle: fn}
	r.mu.Unlock()
	return nil
}

// HandleTyped registers a typed handler for the given kind.
func HandleTyped[T any](r *Router, kind string, fn func(context.Context, T) (any, error)) error {
	if r == nil {
		return errors.New("ipc: nil router")
	}
	if fn == nil {
		return errors.New("ipc: nil handler")
	}
	return r.Handle(kind, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var req T
		if len(payload) == 0 {
			payload = json.RawMessage("null")
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		return fn(ctx, req)
	})
}

// SetFallback configures a handler to run when no route matches.
func (r *Router) SetFallback(fn func(context.Context, string, json.RawMessage) (any, error)) {
	r.mu.Lock()
	r.fallback = fn
	r.mu.Unlock()
}

// Handler returns a single Handler that dispatches to registered routes.
func (r *Router) Handler() Handler {
	return func(ctx context.Context, req json.RawMessage) (any, error) {
		var msg Request
		if err := json.Unmarshal(req, &msg); err != nil {
			return nil, err
		}
		if msg.Type == "" {
			return nil, errors.New("ipc: request type is empty")
		}

		payload := msg.Payload
		if len(payload) == 0 {
			payload = json.RawMessage("null")
		}

		r.mu.RLock()
		route, ok := r.routes[msg.Type]
		fallback := r.fallback
		r.mu.RUnlock()

		if ok {
			return route.handle(ctx, payload)
		}
		if fallback != nil {
			return fallback(ctx, msg.Type, payload)
		}
		return nil, fmt.Errorf("ipc: unknown route %q", msg.Type)
	}
}
