package main

import (
	"context"
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
	router := ipc.NewRouter()
	_ = ipc.HandleTyped(router, "ping", func(ctx context.Context, req Ping) (any, error) {
		return Pong{Text: "child got: " + req.Text}, nil
	})

	ep := ipc.NewStdioEndpoint(router.Handler())
	ep.Start()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var reply ParentHello
		req, err := ipc.NewRequest("child-hello", ChildHello{From: "child"})
		if err != nil {
			fmt.Fprintln(os.Stderr, "child make request:", err)
			return
		}
		if err := ep.Call(ctx, req, &reply); err != nil {
			fmt.Fprintln(os.Stderr, "child -> parent call failed:", err)
			return
		}
		fmt.Fprintln(os.Stderr, "child received:", reply.Note)
	}()

	<-ep.Done()
}
