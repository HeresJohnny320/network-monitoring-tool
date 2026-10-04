package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"network_monitor_tool/tools"
	"network_monitor_tool/utils"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	c := utils.GetConfig()
	run := s.Runner.Status()

	// Only report problems for tests that are switched on.
	enabled := map[string]bool{"ping": c.RunPing, "traceroute": c.Runtraceroute, "speedtest": c.Runspeedtest}
	for tool := range run.Errors {
		if !enabled[tool] {
			delete(run.Errors, tool)
		}
	}

	_, hasPing := utils.LookCommand("ping")
	_, hasTrace := utils.LookCommand(utils.TracerouteCommand())
	nextRun := ""
	if t := s.Scheduler.NextRun(); !t.IsZero() {
		nextRun = t.UTC().Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version":     s.Version,
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"started":     s.Started.UTC().Format(time.RFC3339),
		"next_run":    nextRun,
		"run_every":   c.RunEvery,
		"run":         run,
		"database":    utils.DatabaseDescription(c),
		"config_path": utils.ConfigPath(),
		"listen":      s.Listen,
		"dependencies": map[string]any{
			"ping":       hasPing,
			"traceroute": hasTrace,
			"speedtest":  tools.DetectSpeedtest(c),
		},
	})
}

// configView never sends passwords back to the browser, only whether one is set.
type configView struct {
	utils.Config
	SQLPasswordSet bool `json:"sql_password_set"`
	UIPasswordSet  bool `json:"ui_password_set"`
}

func viewOf(c utils.Config) configView {
	v := configView{Config: c, SQLPasswordSet: c.SQLPassword != "", UIPasswordSet: c.UIPassword != ""}
	v.SQLPassword = ""
	v.UIPassword = ""
	return v
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, viewOf(utils.GetConfig()))
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	old := utils.GetConfig()

	// Start from the current config so a partial update only changes what was sent.
	in := struct {
		utils.Config
		ClearSQLPassword bool `json:"clear_sql_password"`
		ClearUIPassword  bool `json:"clear_ui_password"`
	}{Config: old}
	in.SQLPassword, in.UIPassword = "", ""

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	next := in.Config
	// Blank password fields mean "keep the current one".
	if next.SQLPassword == "" && !in.ClearSQLPassword {
		next.SQLPassword = old.SQLPassword
	}
	if next.UIPassword == "" && !in.ClearUIPassword {
		next.UIPassword = old.UIPassword
	}

	next.Normalize()
	if err := next.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dbChanged := !next.DatabaseSettingsEqual(old)
	if dbChanged {
		if err := utils.InitDatabase(next); err != nil {
			writeError(w, http.StatusBadRequest, "couldn't switch database: "+err.Error())
			return
		}
	}
	if err := utils.SaveConfig(next); err != nil {
		if dbChanged {
			utils.InitDatabase(old)
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if next.RunEvery != old.RunEvery || next.AlignToClock != old.AlignToClock {
		interval, _ := utils.ParseInterval(next.RunEvery)
		s.Scheduler.Reschedule(interval, next.AlignToClock)
	}
	utils.PrintColor("cyan", "Settings updated from the web UI")

	writeJSON(w, http.StatusOK, map[string]any{
		"config":           viewOf(next),
		"restart_required": next.Listen != s.Listen,
	})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if s.Runner.Status().Running {
		writeError(w, http.StatusConflict, "a test run is already in progress")
		return
	}
	go s.Runner.Run()
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	tr, err := parseRange(r, 24*time.Hour)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad date: "+err.Error())
		return
	}
	summary, err := tools.BuildSummary(tr, utils.GetConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleSpeedtests(w http.ResponseWriter, r *http.Request) {
	tr, err := parseRange(r, 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad date: "+err.Error())
		return
	}
	rows, err := tools.QuerySpeedtests(tr, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handlePings(w http.ResponseWriter, r *http.Request) {
	tr, err := parseRange(r, 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad date: "+err.Error())
		return
	}
	rows, err := tools.QueryPings(tr, r.URL.Query().Get("host"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleTraceroutes(w http.ResponseWriter, r *http.Request) {
	tr, err := parseRange(r, 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad date: "+err.Error())
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := tools.QueryTraceroutes(tr, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// Legacy endpoints: same shape as before (strings, bytes/sec) so old scripts still work.

func (s *Server) legacyPing(w http.ResponseWriter, r *http.Request) {
	rows, err := tools.QueryPings(tools.TimeRange{}, "")
	if err != nil {
		http.Error(w, "DB query error", http.StatusInternalServerError)
		return
	}
	type pingDBResult struct {
		Host      string `json:"host"`
		Success   string `json:"success"`
		TimeMs    string `json:"timems"`
		Timestamp string `json:"timestamp"`
	}
	out := make([]pingDBResult, 0, len(rows))
	for _, p := range rows {
		out = append(out, pingDBResult{p.Host, strconv.FormatBool(p.Success), fmt.Sprintf("%.0f", p.TimeMs), p.Timestamp})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) legacySpeedtest(w http.ResponseWriter, r *http.Request) {
	rows, err := tools.QuerySpeedtests(tools.TimeRange{}, 0)
	if err != nil {
		http.Error(w, "DB query error", http.StatusInternalServerError)
		return
	}
	type speedtestResult struct {
		Download       string `json:"download"`
		Upload         string `json:"upload"`
		Ping           string `json:"ping"`
		ServerID       string `json:"serverId"`
		ServerHost     string `json:"serverHost"`
		ServerLocation string `json:"serverLocation"`
		Timestamp      string `json:"timestamp"`
	}
	out := make([]speedtestResult, 0, len(rows))
	for _, t := range rows {
		out = append(out, speedtestResult{
			Download:       fmt.Sprintf("%.0f", t.DownloadMbps*1e6/8),
			Upload:         fmt.Sprintf("%.0f", t.UploadMbps*1e6/8),
			Ping:           fmt.Sprintf("%.2f", t.PingMs),
			ServerID:       t.ServerID,
			ServerHost:     t.ServerHost,
			ServerLocation: t.ServerLocation,
			Timestamp:      t.Timestamp,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) legacyTraceroute(w http.ResponseWriter, r *http.Request) {
	rows, err := tools.QueryTraceroutes(tools.TimeRange{}, 0)
	if err != nil {
		http.Error(w, "DB query error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}
