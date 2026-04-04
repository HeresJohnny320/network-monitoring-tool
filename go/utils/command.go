package utils

import (
	"os/exec"
	"runtime"
)

func Command(command string, args ...string) string {

	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		PrintColor("red", "Error:", err.Error())
		return "false"
	}
	PrintColor("cyan", "Running command on "+runtime.GOOS)
	return string(output)
}
