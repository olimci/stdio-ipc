package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

type Handler func(ctx context.Context, req json.RawMessage) (any, error)

// Endpoint implements a request/response protocol over JSON-encoded stdio streams.
type Endpoint struct {
	enc     *json.Encoder
	dec     *json.Decoder
	handler Handler

	writeMu   sync.Mutex
	pendingMu sync.Mutex
	pending   map[uint64]chan response

	nextID atomic.Uint64

	startOnce sync.Once
	closeOnce sync.Once
	closed    chan struct{}
}

func NewEndpoint(r io.Reader, w io.Writer, handler Handler) *Endpoint {
	return &Endpoint{
		enc:     json.NewEncoder(w),
		dec:     json.NewDecoder(r),
		handler: handler,
		pending: make(map[uint64]chan response),
		closed:  make(chan struct{}),
	}
}

func (e *Endpoint) Start() {
	e.startOnce.Do(func() {
		go e.readLoop()
	})
}

func (e *Endpoint) Done() <-chan struct{} {
	return e.closed
}

func (e *Endpoint) Close() {
	e.closeOnce.Do(func() {
		close(e.closed)

		e.pendingMu.Lock()
		for id, ch := range e.pending {
			delete(e.pending, id)
			ch <- response{err: errors.New("ipc: endpoint closed")}
			close(ch)
		}
		e.pendingMu.Unlock()
	})
}

func (e *Endpoint) Call(ctx context.Context, req any, resp any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	payload, err := marshalPayload(req)
	if err != nil {
		return err
	}

	id := e.nextID.Add(1)
	ch := make(chan response, 1)

	e.pendingMu.Lock()
	e.pending[id] = ch
	e.pendingMu.Unlock()

	if err := e.send(wireMessage{Type: "request", ID: id, Payload: payload}); err != nil {
		e.pendingMu.Lock()
		delete(e.pending, id)
		e.pendingMu.Unlock()
		return err
	}

	select {
	case res := <-ch:
		if res.err != nil {
			return res.err
		}
		if resp == nil {
			return nil
		}
		if raw, ok := resp.(*json.RawMessage); ok {
			*raw = append((*raw)[:0], res.payload...)
			return nil
		}
		return json.Unmarshal(res.payload, resp)
	case <-ctx.Done():
		e.pendingMu.Lock()
		delete(e.pending, id)
		e.pendingMu.Unlock()
		return ctx.Err()
	case <-e.closed:
		return errors.New("ipc: endpoint closed")
	}
}

func (e *Endpoint) readLoop() {
	defer e.Close()
	for {
		var msg wireMessage
		if err := e.dec.Decode(&msg); err != nil {
			return
		}

		switch msg.Type {
		case "request":
			go e.handleRequest(msg)
		case "response":
			e.handleResponse(msg)
		}
	}
}

func (e *Endpoint) handleRequest(msg wireMessage) {
	if e.handler == nil {
		_ = e.send(wireMessage{Type: "response", ID: msg.ID, Error: &wireError{Message: "ipc: no handler"}})
		return
	}

	payload, err := e.handler(context.Background(), msg.Payload)
	if err != nil {
		_ = e.send(wireMessage{Type: "response", ID: msg.ID, Error: &wireError{Message: err.Error()}})
		return
	}

	encoded, err := marshalPayload(payload)
	if err != nil {
		_ = e.send(wireMessage{Type: "response", ID: msg.ID, Error: &wireError{Message: err.Error()}})
		return
	}

	_ = e.send(wireMessage{Type: "response", ID: msg.ID, Payload: encoded})
}

func (e *Endpoint) handleResponse(msg wireMessage) {
	e.pendingMu.Lock()
	ch, ok := e.pending[msg.ID]
	if ok {
		delete(e.pending, msg.ID)
	}
	e.pendingMu.Unlock()

	if !ok {
		return
	}

	if msg.Error != nil {
		ch <- response{err: errors.New(msg.Error.Message)}
		close(ch)
		return
	}

	ch <- response{payload: msg.Payload}
	close(ch)
}

func (e *Endpoint) send(msg wireMessage) error {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	return e.enc.Encode(msg)
}
