package util

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// ParseDuration parses human-friendly duration strings: "5m", "30s", "1h", "2h30m", "90" (seconds).
// Returns the equivalent time.Duration.
func ParseDuration(s string) (time.Duration, error) {
	s = trim(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Bare number -> seconds
	if isDigits(s) {
		secs, err := strconv.Atoi(s)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(secs) * time.Second, nil
	}

	// Parse duration with units (Nh, Nm, Ns)
	re := regexp.MustCompile(`(\d+)([smh])`)
	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid duration format: %s (use: 5m, 30s, 1h, 2h30m)", s)
	}

	var total time.Duration
	for _, m := range matches {
		val, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, fmt.Errorf("invalid number in duration: %s", m[1])
		}
		switch m[2] {
		case "s":
			total += time.Duration(val) * time.Second
		case "m":
			total += time.Duration(val) * time.Minute
		case "h":
			total += time.Duration(val) * time.Hour
		}
	}
	return total, nil
}

func trim(s string) string {
	for _, r := range s {
		if r == ' ' || r == '\t' {
			s = s[1:]
		} else {
			break
		}
	}
	return s
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
