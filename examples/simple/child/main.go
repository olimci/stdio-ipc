package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/olimci/ipc/ipc"
)

type Ping struct {
	Text string `json:"text"`
}

type Pong struct {
	Text string `json:"text"`
}

type ChildHello struct {
	From string `json:"from"`
}

type ParentHello struct {
	From string `json:"from"`
	Note string `json:"note"`
}

func main() {
	handler := func(ctx context.Context, req json.RawMessage) (any, error) {
		var ping Ping
		if err := json.Unmarshal(req, &ping); err != nil {
			return nil, err
		}
		return Pong{Text: "child got: " + ping.Text}, nil
	}

	ep := ipc.NewStdioEndpoint(handler)
	ep.Start()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var reply ParentHello
		if err := ep.Call(ctx, ChildHello{From: "child"}, &reply); err != nil {
			fmt.Fprintln(os.Stderr, "child -> parent call failed:", err)
			return
		}
		fmt.Fprintln(os.Stderr, "child received:", reply.Note)
	}()

	<-ep.Done()
}
