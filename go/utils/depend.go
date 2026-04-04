package utils

import (
	"os/exec"
	"runtime"
)

var commandCache = make(map[string]bool)

func CommandExistsCached(name string) bool {
	if val, ok := commandCache[name]; ok {
		return val
	}

	_, err := exec.LookPath(name)
	exists := err == nil
	commandCache[name] = exists
	return exists
}

func CheckDepend() {
	traceroutecmd := "traceroute"
	if runtime.GOOS == "windows" {
		traceroutecmd = "tracert"
	}

	dependencies := []string{"ping", traceroutecmd, "speedtest"}

	for _, dep := range dependencies {
		if CommandExistsCached(dep) {
			PrintColor("green", dep+" is installed :)")
		} else {
			PrintColor("red", "you need to install "+dep)
		}
	}
}
