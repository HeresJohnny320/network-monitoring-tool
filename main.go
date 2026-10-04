package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"network_monitor_tool/server"
	"network_monitor_tool/tools"
	"network_monitor_tool/utils"
)

//go:embed static
var staticFiles embed.FS

// Set at build time: go build -ldflags "-X main.version=v1.5.0"
var version = "dev"

func main() {
	dataDir := flag.String("data-dir", "", "directory for config.json and database.db (default: your user config dir, or $NETMON_DATA_DIR)")
	listen := flag.String("listen", "", "address to serve the web UI on, overrides config.json (e.g. :8080 or 127.0.0.1:8080)")
	noBrowser := flag.Bool("no-browser", false, "don't open the dashboard in a browser on first run")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("network_monitor_tool", version, runtime.GOOS+"/"+runtime.GOARCH)
		return
	}
	if *dataDir != "" {
		utils.SetDataDir(*dataDir)
	}
	utils.ExtendPath()
	utils.PrintColor("cyan", "Network Monitor "+version+" ("+runtime.GOOS+"/"+runtime.GOARCH+")")

	firstRun, err := utils.LoadConfig()
	if err != nil {
		utils.PrintColor("red", "Error loading config: "+err.Error())
		os.Exit(1)
	}
	cfg := utils.GetConfig()
	utils.PrintColor("cyan", "Config: "+utils.ConfigPath())

	if err := utils.InitDatabase(cfg); err != nil {
		utils.PrintColor("red", "Error opening database: "+err.Error())
		os.Exit(1)
	}
	defer utils.CloseDatabase()
	utils.PrintColor("cyan", "Database: "+utils.DatabaseDescription(cfg))
	checkDependencies(cfg)

	addr := cfg.Listen
	if *listen != "" {
		addr = *listen
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		utils.PrintColor("red", "Can't listen on "+addr+": "+err.Error()+" (is it already running? change \"listen\" in config.json or use -listen)")
		os.Exit(1)
	}

	runner := tools.NewRunner()
	scheduler := utils.NewScheduler(func() { runner.Run() })
	interval, _ := utils.ParseInterval(cfg.RunEvery)
	scheduler.Start(interval, cfg.AlignToClock, true)
	utils.PrintColor("cyan", "Running tests every "+cfg.RunEvery+", next scheduled run "+scheduler.NextRun().Format("15:04:05"))

	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	srv := &http.Server{
		Handler: (&server.Server{
			Version:   version,
			Static:    staticSub,
			Runner:    runner,
			Scheduler: scheduler,
			Listen:    addr,
			Started:   time.Now(),
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	url := dashboardURL(ln.Addr())
	utils.PrintColor("bright_green", "Dashboard: "+url)
	if firstRun && !*noBrowser {
		openBrowser(url)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			utils.PrintColor("red", "Web server error: "+err.Error())
			stop()
		}
	}()

	<-ctx.Done()
	utils.PrintColor("cyan", "Shutting down...")
	scheduler.Stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

func checkDependencies(c utils.Config) {
	for _, dep := range []string{"ping", utils.TracerouteCommand()} {
		if _, ok := utils.LookCommand(dep); ok {
			utils.PrintColor("green", dep+" is installed :)")
		} else {
			utils.PrintColor("red", "you need to install "+dep)
		}
	}
	if engine := tools.DetectSpeedtest(c); engine.Name != "" {
		utils.PrintColor("green", "speedtest is installed ("+engine.Name+": "+engine.Path+") :)")
	} else {
		utils.PrintColor("red", "you need to install speedtest: https://www.speedtest.net/apps/cli")
	}
}

func dashboardURL(addr net.Addr) string {
	host, port, _ := net.SplitHostPort(addr.String())
	if ip := net.ParseIP(host); ip == nil || ip.IsUnspecified() {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return
		}
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
