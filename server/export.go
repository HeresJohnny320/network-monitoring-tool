package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"network_monitor_tool/tools"
)

func fmtFloat(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func fmtFloatPtr(v *float64) string {
	if v == nil {
		return ""
	}
	return fmtFloat(*v)
}

// handleExport serves ?type=speedtest|ping|traceroute|all&format=csv|json as a download.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	tr, err := parseRange(r, 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad date: "+err.Error())
		return
	}
	kind := r.URL.Query().Get("type")
	format := r.URL.Query().Get("format")
	if format != "json" {
		format = "csv"
	}
	if kind == "all" {
		format = "json"
	}

	var data any
	switch kind {
	case "speedtest":
		data, err = tools.QuerySpeedtests(tr, 0)
	case "ping":
		data, err = tools.QueryPings(tr, r.URL.Query().Get("host"))
	case "traceroute":
		data, err = tools.QueryTraceroutes(tr, 0)
	case "all":
		var all struct {
			Speedtest  []tools.SpeedtestRow     `json:"speedtest"`
			Ping       []tools.PingRow          `json:"ping"`
			Traceroute []tools.TracerouteResult `json:"traceroute"`
		}
		if all.Speedtest, err = tools.QuerySpeedtests(tr, 0); err == nil {
			if all.Ping, err = tools.QueryPings(tr, ""); err == nil {
				all.Traceroute, err = tools.QueryTraceroutes(tr, 0)
			}
		}
		data = all
	default:
		writeError(w, http.StatusBadRequest, "type must be speedtest, ping, traceroute or all")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	filename := fmt.Sprintf("network_%s_%s.%s", kind, time.Now().Format("2006-01-02"), format)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(data)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	cw := csv.NewWriter(w)
	defer cw.Flush()
	switch rows := data.(type) {
	case []tools.SpeedtestRow:
		cw.Write([]string{"timestamp", "download_mbps", "upload_mbps", "ping_ms", "jitter_ms", "packet_loss_pct", "isp", "server_id", "server_host", "server_location", "result_url"})
		for _, t := range rows {
			cw.Write([]string{t.Timestamp, fmtFloat(t.DownloadMbps), fmtFloat(t.UploadMbps), fmtFloat(t.PingMs), fmtFloatPtr(t.JitterMs),
				fmtFloatPtr(t.PacketLoss), t.ISP, t.ServerID, t.ServerHost, t.ServerLocation, t.ResultURL})
		}
	case []tools.PingRow:
		cw.Write([]string{"timestamp", "host", "success", "latency_ms", "jitter_ms", "loss_pct"})
		for _, p := range rows {
			cw.Write([]string{p.Timestamp, p.Host, strconv.FormatBool(p.Success), fmtFloat(p.TimeMs), fmtFloat(p.JitterMs), fmtFloat(p.LossPct)})
		}
	case []tools.TracerouteResult:
		cw.Write([]string{"timestamp", "traceroute_id", "host", "hop", "ip", "time1_ms", "time2_ms", "time3_ms"})
		for _, t := range rows {
			for _, h := range t.Hops {
				rec := []string{t.Timestamp, strconv.FormatInt(t.ID, 10), t.Host, strconv.Itoa(h.Hop), h.IP}
				for i := 0; i < 3; i++ {
					if i < len(h.Times) {
						rec = append(rec, fmtFloatPtr(h.Times[i]))
					} else {
						rec = append(rec, "")
					}
				}
				cw.Write(rec)
			}
		}
	}
}
