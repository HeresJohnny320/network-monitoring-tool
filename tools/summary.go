package tools

import (
	"math"
	"sort"

	"network_monitor_tool/utils"
)

type Stats struct {
	Count  int     `json:"count"`
	Avg    float64 `json:"avg"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Median float64 `json:"median"`
}

func computeStats(values []float64) Stats {
	if len(values) == 0 {
		return Stats{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}
	return Stats{Count: len(sorted), Avg: sum / float64(len(sorted)), Min: sorted[0], Max: sorted[len(sorted)-1], Median: median}
}

type SpeedSummary struct {
	Count    int           `json:"count"`
	Download Stats         `json:"download_mbps"`
	Upload   Stats         `json:"upload_mbps"`
	Latency  Stats         `json:"ping_ms"`
	Latest   *SpeedtestRow `json:"latest"` // newest overall, not limited to the range

	PlanDownloadMbps float64 `json:"plan_download_mbps"`
	PlanUploadMbps   float64 `json:"plan_upload_mbps"`
	// Share of tests that reached at least 80% of the plan speed (the threshold
	// regulators and most ISPs' own "typical speed" promises are based on).
	DownloadMeetsPlanPct *float64 `json:"download_meets_plan_pct"`
	UploadMeetsPlanPct   *float64 `json:"upload_meets_plan_pct"`
}

type HostSummary struct {
	Host      string   `json:"host"`
	Count     int      `json:"count"`
	Success   int      `json:"success"`
	UptimePct float64  `json:"uptime_pct"`
	Latency   Stats    `json:"latency_ms"`
	AvgLoss   float64  `json:"avg_loss_pct"`
	AvgJitter float64  `json:"avg_jitter_ms"`
	Last      *PingRow `json:"last"`
}

type PingSummary struct {
	Count     int           `json:"count"`
	Success   int           `json:"success"`
	UptimePct float64       `json:"uptime_pct"`
	Latency   Stats         `json:"latency_ms"`
	AvgLoss   float64       `json:"avg_loss_pct"`
	AvgJitter float64       `json:"avg_jitter_ms"`
	Hosts     []HostSummary `json:"hosts"`
}

type Summary struct {
	Speedtest   SpeedSummary       `json:"speedtest"`
	Ping        PingSummary        `json:"ping"`
	Traceroutes []TracerouteResult `json:"traceroutes"` // latest per host
}

const planThreshold = 0.8

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(float64(part)/float64(total)*1000) / 10
}

func BuildSummary(r TimeRange, c utils.Config) (Summary, error) {
	var s Summary

	speeds, err := QuerySpeedtests(r, 0)
	if err != nil {
		return s, err
	}
	var down, up, lat []float64
	downOK, upOK := 0, 0
	for _, t := range speeds {
		down = append(down, t.DownloadMbps)
		up = append(up, t.UploadMbps)
		lat = append(lat, t.PingMs)
		if c.PlanDownloadMbps > 0 && t.DownloadMbps >= c.PlanDownloadMbps*planThreshold {
			downOK++
		}
		if c.PlanUploadMbps > 0 && t.UploadMbps >= c.PlanUploadMbps*planThreshold {
			upOK++
		}
	}
	s.Speedtest = SpeedSummary{
		Count:            len(speeds),
		Download:         computeStats(down),
		Upload:           computeStats(up),
		Latency:          computeStats(lat),
		PlanDownloadMbps: c.PlanDownloadMbps,
		PlanUploadMbps:   c.PlanUploadMbps,
	}
	if c.PlanDownloadMbps > 0 && len(speeds) > 0 {
		v := pct(downOK, len(speeds))
		s.Speedtest.DownloadMeetsPlanPct = &v
	}
	if c.PlanUploadMbps > 0 && len(speeds) > 0 {
		v := pct(upOK, len(speeds))
		s.Speedtest.UploadMeetsPlanPct = &v
	}
	if latest, err := QuerySpeedtests(TimeRange{}, 1); err == nil && len(latest) > 0 {
		s.Speedtest.Latest = &latest[0]
	}

	pings, err := QueryPings(r, "")
	if err != nil {
		return s, err
	}
	type acc struct {
		rows    []PingRow
		success int
		lat     []float64
		loss    float64
		jitter  float64
		jitterN int
	}
	byHost := map[string]*acc{}
	var order []string
	var allLat []float64
	totalLoss, totalJitter, jitterN, success := 0.0, 0.0, 0, 0
	for _, p := range pings {
		a := byHost[p.Host]
		if a == nil {
			a = &acc{}
			byHost[p.Host] = a
			order = append(order, p.Host)
		}
		a.rows = append(a.rows, p)
		a.loss += p.LossPct
		totalLoss += p.LossPct
		if p.Success {
			a.success++
			success++
			a.lat = append(a.lat, p.TimeMs)
			allLat = append(allLat, p.TimeMs)
			a.jitter += p.JitterMs
			a.jitterN++
			totalJitter += p.JitterMs
			jitterN++
		}
	}
	s.Ping = PingSummary{
		Count:     len(pings),
		Success:   success,
		UptimePct: pct(success, len(pings)),
		Latency:   computeStats(allLat),
		Hosts:     []HostSummary{},
	}
	if len(pings) > 0 {
		s.Ping.AvgLoss = totalLoss / float64(len(pings))
	}
	if jitterN > 0 {
		s.Ping.AvgJitter = totalJitter / float64(jitterN)
	}
	sort.Strings(order)
	for _, host := range order {
		a := byHost[host]
		last := a.rows[len(a.rows)-1]
		h := HostSummary{
			Host:      host,
			Count:     len(a.rows),
			Success:   a.success,
			UptimePct: pct(a.success, len(a.rows)),
			Latency:   computeStats(a.lat),
			AvgLoss:   a.loss / float64(len(a.rows)),
			Last:      &last,
		}
		if a.jitterN > 0 {
			h.AvgJitter = a.jitter / float64(a.jitterN)
		}
		s.Ping.Hosts = append(s.Ping.Hosts, h)
	}

	// Latest traceroute for each host (from the most recent handful of runs).
	recent, err := QueryTraceroutes(TimeRange{}, 50)
	if err != nil {
		return s, err
	}
	s.Traceroutes = []TracerouteResult{}
	seen := map[string]bool{}
	for _, t := range recent {
		if !seen[t.Host] {
			seen[t.Host] = true
			s.Traceroutes = append(s.Traceroutes, t)
		}
	}
	return s, nil
}
