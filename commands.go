package git

import (
	"bytes"
	"os"
	"os/exec"
	"sync"
)

type gitCmd struct {
	command string
	args    []string
	dir     string

	sync.RWMutex
}

// Exec executes the command initiated in gitCmd.
func (g *gitCmd) Exec(dir string) error {
	g.Lock()
	g.dir = dir
	g.Unlock()
	return runCmd(g.command, g.args, dir)
}

// runCmd is a helper function to run commands.
// It runs command with args from directory at dir.
// The executed process outputs to os.Stderr
func runCmd(command string, args []string, dir string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

// runCmdOutput is a helper function to run commands and return output.
// It runs command with args from directory at dir.
// If successful, returns output and nil error
func runCmdOutput(command string, args []string, dir string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	var err error
	if output, err := cmd.Output(); err == nil {
		return string(bytes.TrimSpace(output)), nil
	}
	return "", err
}
