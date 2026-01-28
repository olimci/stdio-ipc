package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	childPath, err := buildChild()
	if err != nil {
		fmt.Fprintln(os.Stderr, "build child:", err)
		os.Exit(1)
	}

	cmd := exec.Command(childPath)

	childCalled := make(chan struct{})
	handler := func(ctx context.Context, req json.RawMessage) (any, error) {
		var hello ChildHello
		if err := json.Unmarshal(req, &hello); err != nil {
			return nil, err
		}
		select {
		case <-childCalled:
		default:
			close(childCalled)
		}
		return ParentHello{From: "parent", Note: "hi " + hello.From + ", got your call"}, nil
	}

	ep, stdin, _, err := ipc.NewCmdEndpoint(cmd, handler)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wire child:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var pong Pong
	if err := ep.Call(ctx, Ping{Text: "hello from parent"}, &pong); err != nil {
		fmt.Fprintln(os.Stderr, "parent -> child call failed:", err)
		_ = stdin.Close()
		_ = cmd.Wait()
		os.Exit(1)
	}
	fmt.Println("parent received:", pong.Text)

	select {
	case <-childCalled:
	case <-time.After(2 * time.Second):
		fmt.Fprintln(os.Stderr, "did not receive child -> parent call")
	}

	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		if !errors.Is(err, os.ErrProcessDone) {
			fmt.Fprintln(os.Stderr, "child exit:", err)
		}
	}
}

func buildChild() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	binDir := filepath.Join(cwd, "examples", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}

	childPath := filepath.Join(binDir, "ipc-simple-child")
	cmd := exec.Command("go", "build", "-o", childPath, "./examples/simple/child")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return childPath, cmd.Run()
}
