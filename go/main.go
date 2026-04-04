package main

import (
	"net/http"
	"network_monitor_tool/tools"
	"network_monitor_tool/utils"
)

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
		http.ServeFile(w, r, "static/index.html")
	})

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	utils.PrintColor("cyan", "Server running on :8080")
	http.ListenAndServe(":8080", nil)

}
