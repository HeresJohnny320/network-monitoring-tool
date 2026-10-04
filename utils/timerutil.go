package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var intervalRe = regexp.MustCompile(`(\d+)([dhms])`)

// ParseInterval accepts Go durations ("1h30m") plus days ("1d").
func ParseInterval(input string) (time.Duration, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	if d, err := time.ParseDuration(input); err == nil {
		return d, nil
	}
	if strings.TrimSpace(intervalRe.ReplaceAllString(input, "")) != "" || input == "" {
		return 0, fmt.Errorf("invalid duration %q (try 30m, 1h, 1h30m or 1d)", input)
	}

	var total time.Duration
	for _, match := range intervalRe.FindAllStringSubmatch(input, -1) {
		value, _ := strconv.Atoi(match[1])
		switch match[2] {
		case "d":
			total += time.Duration(value) * 24 * time.Hour
		case "h":
			total += time.Duration(value) * time.Hour
		case "m":
			total += time.Duration(value) * time.Minute
		case "s":
			total += time.Duration(value) * time.Second
		}
	}
	return total, nil
}

// NextRunTime returns when the next run should happen. With align, runs land on
// multiples of the interval counted from local midnight (1h -> every hour on the hour).
func NextRunTime(now time.Time, interval time.Duration, align bool) time.Time {
	if !align || interval > 24*time.Hour {
		return now.Add(interval)
	}
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	elapsed := now.Sub(midnight)
	next := midnight.Add((elapsed/interval + 1) * interval)
	// Don't run past midnight with a partial slot; start fresh from the next midnight.
	if nextMidnight := midnight.AddDate(0, 0, 1); next.After(nextMidnight) {
		next = nextMidnight
	}
	return next
}

// Scheduler runs a task repeatedly and can be rescheduled when settings change.
type Scheduler struct {
	mu       sync.Mutex
	task     func()
	timer    *time.Timer
	next     time.Time
	interval time.Duration
	align    bool
	stopped  bool
}

func NewScheduler(task func()) *Scheduler {
	return &Scheduler{task: task}
}

// Start schedules the task; with runNow it also runs once immediately.
func (s *Scheduler) Start(interval time.Duration, align bool, runNow bool) {
	if runNow {
		go s.task()
	}
	s.Reschedule(interval, align)
}

func (s *Scheduler) Reschedule(interval time.Duration, align bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.interval = interval
	s.align = align
	s.armLocked()
}

func (s *Scheduler) armLocked() {
	if s.timer != nil {
		s.timer.Stop()
	}
	s.next = NextRunTime(time.Now(), s.interval, s.align)
	s.timer = time.AfterFunc(time.Until(s.next), s.fire)
}

func (s *Scheduler) fire() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.armLocked()
	s.mu.Unlock()
	s.task()
}

func (s *Scheduler) NextRun() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.next
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.timer != nil {
		s.timer.Stop()
	}
}
