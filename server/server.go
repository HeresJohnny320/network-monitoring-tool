package server

import (
	"crypto/subtle"
	"encoding/json"
	"io/fs"
	"mime"
	"net/http"
	"strings"
	"time"

	"network_monitor_tool/tools"
	"network_monitor_tool/utils"
)

type Server struct {
	Version   string
	Static    fs.FS // contents of the static/ directory
	Runner    *tools.Runner
	Scheduler *utils.Scheduler
	Listen    string // address actually bound, to tell when a restart is needed
	Started   time.Time
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Original endpoints, kept so existing scripts keep working.
	mux.HandleFunc("GET /ping", s.legacyPing)
	mux.HandleFunc("GET /speedtest", s.legacySpeedtest)
	mux.HandleFunc("GET /traceroute", s.legacyTraceroute)

	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("POST /api/config", s.handleSaveConfig)
	mux.HandleFunc("POST /api/run", s.handleRun)
	mux.HandleFunc("GET /api/summary", s.handleSummary)
	mux.HandleFunc("GET /api/speedtest", s.handleSpeedtests)
	mux.HandleFunc("GET /api/ping", s.handlePings)
	mux.HandleFunc("GET /api/traceroute", s.handleTraceroutes)
	mux.HandleFunc("GET /api/export", s.handleExport)

	static := http.StripPrefix("/static/", http.FileServer(http.FS(s.Static)))
	mux.Handle("GET /static/", noCache(static))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(s.Static, "index.html")
		if err != nil {
			http.Error(w, "Index not found: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(data)
	})

	return s.withAuth(withSecurity(mux))
}

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

// withSecurity blocks cross-site requests from changing anything: mutating requests
// must be JSON, which a browser won't send cross-origin without a CORS preflight
// (and we never answer one).
func withSecurity(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if mt != "application/json" {
				writeError(w, http.StatusUnsupportedMediaType, "requests must be application/json")
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

// withAuth enforces the optional ui_password (any username).
func (s *Server) withAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		password := utils.GetConfig().UIPassword
		if password != "" {
			_, given, ok := r.BasicAuth()
			if !ok || subtle.ConstantTimeCompare([]byte(given), []byte(password)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="Network Monitor", charset="UTF-8"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parseRange reads ?from=&to= (RFC3339 or YYYY-MM-DD). If neither is given and
// defaultWindow > 0, it returns the last defaultWindow.
func parseRange(r *http.Request, defaultWindow time.Duration) (tools.TimeRange, error) {
	var tr tools.TimeRange
	parse := func(v string, endOfDay bool) (time.Time, error) {
		v = strings.TrimSpace(v)
		if v == "" {
			return time.Time{}, nil
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t, nil
		}
		t, err := time.ParseInLocation("2006-01-02", v, time.Local)
		if err != nil {
			return time.Time{}, err
		}
		if endOfDay {
			t = t.Add(24*time.Hour - time.Second)
		}
		return t, nil
	}
	var err error
	if tr.From, err = parse(r.URL.Query().Get("from"), false); err != nil {
		return tr, err
	}
	if tr.To, err = parse(r.URL.Query().Get("to"), true); err != nil {
		return tr, err
	}
	if tr.From.IsZero() && tr.To.IsZero() && defaultWindow > 0 {
		tr.From = time.Now().Add(-defaultWindow)
	}
	return tr, nil
}
