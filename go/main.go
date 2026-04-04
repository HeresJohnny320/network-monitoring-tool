package main

import (
	"embed"
	"io/fs"
	"net/http"
	"network_monitor_tool/tools"
	"network_monitor_tool/utils"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	if err := utils.LoadConfig(); err != nil {
		utils.PrintColor("red", "Error loading config:", err.Error())
		return
	}
	utils.InitDatabase()
	if err := utils.CreateTables(); err != nil {
		utils.PrintColor("red", "Error creating tables:", err.Error())
		return
	}
	utils.CheckDepend()

	utils.ScheduleFromConfig(func() {
		if utils.CommandExistsCached("ping") {
			tools.PingHosts(utils.Cfg.PingHost)
		}
		if utils.CommandExistsCached("tracert") || utils.CommandExistsCached("traceroute") {
			tools.RunTracerouteForHosts(utils.Cfg.TracerouteHost)
		}
		if utils.CommandExistsCached("speedtest") {
			tools.RunSpeedtest(utils.Cfg.SpeedtestServerID)
		}
	})

	http.HandleFunc("/ping", tools.GetAllPingResults)
	http.HandleFunc("/traceroute", tools.GetAllTraceroutes)
	http.HandleFunc("/speedtest", tools.GetAllSpeedtestResults)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "Index not found: "+err.Error(), 500)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})

	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	utils.PrintColor("cyan", "Server running on :8080")
	http.ListenAndServe(":8080", nil)
}
