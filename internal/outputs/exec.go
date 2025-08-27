package outputs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"nomad-events/internal/nomad"
)

type ExecOutput struct {
	command []string
	timeout time.Duration
	workdir string
	env     []string
}

func NewExecOutput(config map[string]interface{}) (*ExecOutput, error) {
	cmdInterface, ok := config["command"]
	if !ok {
		return nil, fmt.Errorf("command is required for exec output")
	}

	var command []string
	switch cmd := cmdInterface.(type) {
	case string:
		command = strings.Fields(cmd)
	case []interface{}:
		command = make([]string, len(cmd))
		for i, arg := range cmd {
			if s, ok := arg.(string); ok {
				command[i] = s
			} else {
				return nil, fmt.Errorf("command arguments must be strings")
			}
		}
	case []string:
		command = cmd
	default:
		return nil, fmt.Errorf("command must be a string or array of strings")
	}

	if len(command) == 0 {
		return nil, fmt.Errorf("command cannot be empty")
	}

	timeout := 30 * time.Second
	if t, ok := config["timeout"].(int); ok {
		timeout = time.Duration(t) * time.Second
	}

	workdir, _ := config["workdir"].(string)

	var env []string
	if envMap, ok := config["env"].(map[string]interface{}); ok {
		for k, v := range envMap {
			if s, ok := v.(string); ok {
				env = append(env, fmt.Sprintf("%s=%s", k, s))
			}
		}
	}

	return &ExecOutput{
		command: command,
		timeout: timeout,
		workdir: workdir,
		env:     env,
	}, nil
}

func (o *ExecOutput) Send(event nomad.Event) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event to JSON: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, o.command[0], o.command[1:]...)
	
	if o.workdir != "" {
		cmd.Dir = o.workdir
	}
	
	if len(o.env) > 0 {
		cmd.Env = append(cmd.Env, o.env...)
	}

	cmd.Stdin = bytes.NewReader(eventJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command execution failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}