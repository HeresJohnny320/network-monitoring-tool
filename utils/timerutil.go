package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func ParseDuration(input string) (time.Duration, error) {
	input = strings.ToLower(input)
	re := regexp.MustCompile(`(\d+)([hms])`)
	matches := re.FindAllStringSubmatch(input, -1)

	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid duration format: %s", input)
	}

	var totalMs int64 = 0
	for _, match := range matches {
		value, _ := strconv.Atoi(match[1])
		unit := match[2]

		switch unit {
		case "h":
			totalMs += int64(value) * 60 * 60 * 1000
		case "m":
			totalMs += int64(value) * 60 * 1000
		case "s":
			totalMs += int64(value) * 1000
		default:
			return 0, fmt.Errorf("unknown time unit: %s", unit)
		}
	}

	return time.Duration(totalMs) * time.Millisecond, nil
}

func ScheduleRepeating(durationStr string, task func()) error {
	interval, err := ParseDuration(durationStr)
	if err != nil {
		return err
	}

	if interval <= 0 {
		return fmt.Errorf("duration must be positive")
	}

	ticker := time.NewTicker(interval)

	go task()

	go func() {
		for range ticker.C {
			task()
		}
	}()

	return nil
}

func ScheduleFromConfig(task func()) error {
	if Cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	PrintColor("cyan", "Running Tools Every:"+Cfg.RunEvery+"\n")
	return ScheduleRepeating(Cfg.RunEvery, task)
}
