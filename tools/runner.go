package tools

import (
	"sync"
	"time"

	"network_monitor_tool/utils"
)

// Runner runs one round of tests at a time and remembers how it went.
type Runner struct {
	mu        sync.Mutex
	running   bool
	step      string
	lastStart time.Time
	lastEnd   time.Time
	errors    map[string]string
}

type RunStatus struct {
	Running   bool              `json:"running"`
	Step      string            `json:"step"`
	LastStart string            `json:"last_start,omitempty"`
	LastEnd   string            `json:"last_end,omitempty"`
	Errors    map[string]string `json:"errors"`
}

func NewRunner() *Runner {
	return &Runner{errors: map[string]string{}}
}

func (r *Runner) Status() RunStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := RunStatus{Running: r.running, Step: r.step, Errors: map[string]string{}}
	if !r.lastStart.IsZero() {
		s.LastStart = r.lastStart.UTC().Format(time.RFC3339)
	}
	if !r.lastEnd.IsZero() {
		s.LastEnd = r.lastEnd.UTC().Format(time.RFC3339)
	}
	for k, v := range r.errors {
		s.Errors[k] = v
	}
	return s
}

func (r *Runner) setStep(step string) {
	r.mu.Lock()
	r.step = step
	r.mu.Unlock()
}

func (r *Runner) setError(tool string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.errors[tool] = err.Error()
	} else {
		delete(r.errors, tool)
	}
}

// Run does one round of tests. It returns false (and does nothing) if a round is
// already in progress, so a slow speed test never stacks up behind itself.
func (r *Runner) Run() bool {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		utils.PrintColor("yellow", "Previous run still in progress, skipping")
		return false
	}
	r.running = true
	r.lastStart = time.Now()
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.running = false
		r.step = ""
		r.lastEnd = time.Now()
		r.mu.Unlock()
	}()

	c := utils.GetConfig()

	// Ping and traceroute are light, so run them together; the speed test runs on
	// its own afterwards so it doesn't skew (or get skewed by) the latency numbers.
	var wg sync.WaitGroup
	r.setStep("ping & traceroute")
	if c.RunPing && len(c.PingHost) > 0 {
		if _, ok := utils.LookCommand("ping"); ok {
			wg.Add(1)
			go func() {
				defer wg.Done()
				PingHosts(c.PingHost, c.PingCount)
			}()
			r.setError("ping", nil)
		} else {
			r.setError("ping", errMissing("ping"))
		}
	}
	if c.Runtraceroute && len(c.TracerouteHost) > 0 {
		if _, ok := utils.LookCommand(utils.TracerouteCommand()); ok {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r.setError("traceroute", RunTracerouteForHosts(c.TracerouteHost))
			}()
		} else {
			r.setError("traceroute", errMissing(utils.TracerouteCommand()))
		}
	}
	wg.Wait()

	if c.Runspeedtest {
		r.setStep("speedtest")
		r.setError("speedtest", RunSpeedtest(c))
	}

	Prune(c.RetentionDays)
	utils.PrintColor("cyan", "Run finished")
	return true
}

type missingError string

func (m missingError) Error() string { return string(m) + " is not installed" }

func errMissing(cmd string) error { return missingError(cmd) }
