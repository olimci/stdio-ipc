package ipc

import (
	"errors"
	"io"
	"os"
	"os/exec"
)

// NewCmdEndpoint wires an exec.Cmd's stdin/stdout pipes to an Endpoint and starts the command.
// It sets cmd.Stderr to os.Stderr if unset.
func NewCmdEndpoint(cmd *exec.Cmd, handler Handler) (*Endpoint, io.WriteCloser, io.ReadCloser, error) {
	if cmd == nil {
		return nil, nil, nil, errors.New("ipc: nil cmd")
	}
	if cmd.Stdin != nil {
		return nil, nil, nil, errors.New("ipc: cmd stdin already set")
	}
	if cmd.Stdout != nil {
		return nil, nil, nil, errors.New("ipc: cmd stdout already set")
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, nil, nil, err
	}
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, nil, nil, err
	}

	ep := NewEndpoint(stdout, stdin, handler)
	ep.Start()
	return ep, stdin, stdout, nil
}

// NewStdioEndpoint builds an endpoint over the current process stdio.
func NewStdioEndpoint(handler Handler) *Endpoint {
	return NewEndpoint(os.Stdin, os.Stdout, handler)
}
