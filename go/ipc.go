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

type IPC struct {
	enc     *json.Encoder
	dec     *json.Decoder
	handler Handler

	writeMu   sync.Mutex
	pendingMu sync.Mutex
	pending   map[uint64]chan response

	nextID atomic.Uint64

	startFunc func() error

	startOnce sync.Once
	closeOnce sync.Once
	closed    chan struct{}
}

func New(r io.Reader, w io.Writer, handler Handler) *IPC {
	return &IPC{
		enc:     json.NewEncoder(w),
		dec:     json.NewDecoder(r),
		handler: handler,
		pending: make(map[uint64]chan response),
		closed:  make(chan struct{}),
	}
}

func (i *IPC) Start() error {
	var err error
	i.startOnce.Do(func() {
		go i.readLoop()
		err = i.startFunc()
	})

	return err
}

func (i *IPC) Done() <-chan struct{} {
	return i.closed
}

func (i *IPC) Close() {
	i.closeOnce.Do(func() {
		close(i.closed)

		i.pendingMu.Lock()
		for id, ch := range i.pending {
			delete(i.pending, id)
			ch <- response{err: errors.New("ipc: endpoint closed")}
			close(ch)
		}
		i.pendingMu.Unlock()
	})
}

func Call[T1, T2 any](i *IPC, ctx context.Context, req T1) (resp T2, err error) {
	err = i.Call(ctx, req, &resp)
	return resp, err
}

func (i *IPC) Call(ctx context.Context, req any, resp any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	payload, err := marshalPayload(req)
	if err != nil {
		return err
	}

	id := i.nextID.Add(1)
	ch := make(chan response, 1)

	i.pendingMu.Lock()
	i.pending[id] = ch
	i.pendingMu.Unlock()

	if err := i.send(wireMessage{Type: "request", ID: id, Payload: payload}); err != nil {
		i.pendingMu.Lock()
		delete(i.pending, id)
		i.pendingMu.Unlock()
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
		i.pendingMu.Lock()
		delete(i.pending, id)
		i.pendingMu.Unlock()
		return ctx.Err()
	case <-i.closed:
		return errors.New("ipc: endpoint closed")
	}
}

func (i *IPC) readLoop() {
	defer i.Close()
	for {
		var msg wireMessage
		if err := i.dec.Decode(&msg); err != nil {
			return
		}

		switch msg.Type {
		case "request":
			go i.handleRequest(msg)
		case "response":
			i.handleResponse(msg)
		}
	}
}

func (i *IPC) handleRequest(msg wireMessage) {
	if i.handler == nil {
		_ = i.send(wireMessage{Type: "response", ID: msg.ID, Error: &wireError{Message: "ipc: no handler"}})
		return
	}

	payload, err := i.handler(context.Background(), msg.Payload)
	if err != nil {
		_ = i.send(wireMessage{Type: "response", ID: msg.ID, Error: &wireError{Message: err.Error()}})
		return
	}

	encoded, err := marshalPayload(payload)
	if err != nil {
		_ = i.send(wireMessage{Type: "response", ID: msg.ID, Error: &wireError{Message: err.Error()}})
		return
	}

	_ = i.send(wireMessage{Type: "response", ID: msg.ID, Payload: encoded})
}

func (i *IPC) handleResponse(msg wireMessage) {
	i.pendingMu.Lock()
	ch, ok := i.pending[msg.ID]
	if ok {
		delete(i.pending, msg.ID)
	}
	i.pendingMu.Unlock()

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

func (i *IPC) send(msg wireMessage) error {
	i.writeMu.Lock()
	defer i.writeMu.Unlock()
	return i.enc.Encode(msg)
}
