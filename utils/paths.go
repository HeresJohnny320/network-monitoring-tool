package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const appDirName = "heresjohnnys320_network_monitor_tool"

var dataDirOverride string

// SetDataDir overrides where config.json and database.db live (used by -data-dir,
// the service installers, Docker and pfSense).
func SetDataDir(dir string) {
	dataDirOverride = dir
}

// DataDir returns (and creates) the directory holding config.json and database.db.
func DataDir() (string, error) {
	dir := dataDirOverride
	if dir == "" {
		dir = os.Getenv("NETMON_DATA_DIR")
	}
	if dir == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(configDir, appDirName)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// ExtendPath adds the usual install locations to PATH. Services (systemd, launchd,
// FreeBSD rc.d) start with a minimal PATH that often misses /usr/local/bin, which is
// where speedtest usually lives.
func ExtendPath() {
	if runtime.GOOS == "windows" {
		return
	}
	extra := []string{"/usr/local/bin", "/usr/local/sbin", "/opt/homebrew/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"}
	current := filepath.SplitList(os.Getenv("PATH"))
	have := map[string]bool{}
	for _, p := range current {
		have[p] = true
	}
	for _, p := range extra {
		if !have[p] {
			current = append(current, p)
		}
	}
	os.Setenv("PATH", strings.Join(current, string(os.PathListSeparator)))
}

// LookCommand finds an executable, checking the directory of this binary first so
// tools shipped next to it (e.g. speedtest.exe on Windows) are picked up.
func LookCommand(name string) (string, bool) {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), name)
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(candidate), ".exe") {
			candidate += ".exe"
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	path, err := exec.LookPath(name)
	return path, err == nil
}

// TracerouteCommand is "tracert" on Windows and "traceroute" everywhere else.
func TracerouteCommand() string {
	if runtime.GOOS == "windows" {
		return "tracert"
	}
	return "traceroute"
}
