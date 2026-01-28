package ipc

import (
	"os"
	"os/exec"
)

func FromCmd(cmd *exec.Cmd, handler Handler) (*IPC, error) {
	i := New(cmd.Stdin, cmd.Stdout, handler)

	i.startFunc = cmd.Start

	return i, nil
}

func Default(handler Handler) *IPC {
	return New(os.Stdout, os.Stdin, handler)
}
