// system.go implements the "System" wrapper class to exec.Cmd

package main

import (
	"io"
	"os/exec"
)

const (
	shellpath = "/bin/sh"
	shellopts = "-c"
)

// System is a wrapper around exec.Cmd to run things in the Ed way
type System struct {
	Cmd    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	cmdSane string
	exe     *exec.Cmd
}

// Run a command (using the shell for arg processing)
func (s *System) Run() (e error) {
	s.cmdSane = rxSanitize.ReplaceAllString(s.Cmd, "..")
	idx := rxCmdSub.FindAllStringIndex(s.cmdSane, -1)
	fCmd := ""
	oCmd := 0
	for _, m := range idx {
		fCmd += s.Cmd[oCmd:m[0]]
		fCmd += state.fileName
		oCmd = m[1]
	}
	fCmd += s.Cmd[oCmd:]

	cmd := exec.Command(shellpath, shellopts, fCmd)
	cmd.Stdin = s.Stdin
	cmd.Stdout = s.Stdout
	cmd.Stderr = s.Stderr
	return cmd.Run()
}
