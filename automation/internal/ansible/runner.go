package ansible

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Runner handles the execution of Ansible playbooks and commands
type Runner struct{}

// NewRunner creates a new Ansible Runner
func NewRunner() *Runner {
	return &Runner{}
}

// ExecuteAdHoc runs an ad-hoc ansible command
// For example: ansible all -i "localhost," -m ping
func (r *Runner) ExecuteAdHoc(target string, module string, args string) (string, error) {
	// Construct the command
	// We use "localhost," (with comma) to treat it as a list, avoiding need for inventory file for simple tests
	// In production this will likely need a real inventory source
	cmdArgs := []string{
		target,
		"-i", fmt.Sprintf("%s,", target),
		"-m", module,
	}

	if args != "" {
		cmdArgs = append(cmdArgs, "-a", args)
	}

	// Use "local" connection for localhost to avoid SSH requirement for self-ping
	if target == "localhost" || target == "127.0.0.1" {
		cmdArgs = append(cmdArgs, "-c", "local")
	}

	cmd := exec.Command("ansible", cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("ansible execution failed: %s, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// ExecutePlaybook runs an Ansible playbook
func (r *Runner) ExecutePlaybook(target string, inventory string, playbookPath string, extraVars map[string]string) (string, error) {
	// Construct the command
	// ansible-playbook -i target, playbookPath --extra-vars "key=value"
	cmdArgs := []string{}

	// Handle inventory: either explicit file or target list
	if inventory != "" {
		cmdArgs = append(cmdArgs, "-i", inventory)
	} else {
		cmdArgs = append(cmdArgs, "-i", fmt.Sprintf("%s,", target))
	}

	cmdArgs = append(cmdArgs, playbookPath)

	// Add extra vars if provided
	if len(extraVars) > 0 {
		var varsList []string
		for k, v := range extraVars {
			varsList = append(varsList, fmt.Sprintf("%s=%s", k, v))
		}
		cmdArgs = append(cmdArgs, "--extra-vars", strings.Join(varsList, " "))
	}

	// Connection type local for localhost
	if target == "localhost" || target == "127.0.0.1" {
		cmdArgs = append(cmdArgs, "-c", "local")
	}

	cmd := exec.Command("ansible-playbook", cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Include stdout in the error because Ansible often prints important failure info there
		return stdout.String(), fmt.Errorf("playbook execution failed: %s\nSTDOUT: %s\nSTDERR: %s", err, stdout.String(), stderr.String())
	}

	return stdout.String(), nil
}
