package ipc

import (
	"os"
	"os/exec"
)

func FromCmd(cmd exec.Cmd, handler Handler) (*IPC, error) {
	i := New(cmd.Stdin, cmd.Stdout, handler)

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return i, nil
}

func Default(handler Handler) *IPC {
	return New(os.Stdout, os.Stdin, handler)
}
