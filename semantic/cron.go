package semantic

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // tz names resolve the same on every machine
)

// Schedule diagnostics (DESIGN §5.9). The rules mirror the platform's
// runtime parser (robfig/cron v3 ParseStandard via gocron, plus "@every"):
// 5 fields or a descriptor; seconds and TZ= prefixes are refused.

var cronDescriptors = map[string]bool{"@yearly": true, "@annually": true, "@monthly": true,
	"@weekly": true, "@daily": true, "@midnight": true, "@hourly": true}

var monthNames = map[string]int{"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12}
var dowNames = map[string]int{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}

// ValidateCron returns nil if expr is a schedule the runtime accepts.
func ValidateCron(expr string) error {
	s := strings.TrimSpace(expr)
	if s == "" {
		return fmt.Errorf("empty expression")
	}
	if strings.HasPrefix(s, "TZ=") || strings.HasPrefix(s, "CRON_TZ=") {
		return fmt.Errorf("the time zone goes in tz \"<IANA>\", not in the expression")
	}
	f := strings.Fields(s)
	if f[0] == "@every" {
		if len(f) != 2 {
			return fmt.Errorf("use \"@every <interval>\", e.g. \"@every 30s\"")
		}
		return validateEvery(f[1])
	}
	if strings.HasPrefix(f[0], "@") {
		if len(f) == 1 && cronDescriptors[f[0]] {
			return nil
		}
		return fmt.Errorf("unknown descriptor %q", f[0])
	}
	if len(f) != 5 {
		if len(f) == 6 || len(f) == 7 {
			return fmt.Errorf("expected 5 fields, got %d (seconds are not supported)", len(f))
		}
		return fmt.Errorf("expected 5 fields, got %d", len(f))
	}
	bounds := [5][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 6}}
	names := [5]map[string]int{nil, nil, nil, monthNames, dowNames}
	for i, field := range f {
		if err := validateCronField(field, bounds[i][0], bounds[i][1], names[i], i == 2 || i == 4); err != nil {
			return fmt.Errorf("field %d (%q): %v", i+1, field, err)
		}
	}
	return nil
}

func validateEvery(d string) error {
	if strings.HasSuffix(d, "d") {
		if n, err := strconv.ParseInt(strings.TrimSuffix(d, "d"), 10, 64); err == nil {
			if n <= 0 {
				return fmt.Errorf("interval must be positive")
			}
			return nil
		}
	}
	dur, err := time.ParseDuration(d)
	if err != nil {
		return fmt.Errorf("cannot parse interval %q", d)
	}
	if dur < time.Second {
		return fmt.Errorf("interval must be at least 1s")
	}
	return nil
}

func cronValue(s string, names map[string]int) (int, bool) {
	if v, ok := names[strings.ToLower(s)]; ok {
		return v, true
	}
	v, err := strconv.Atoi(s)
	return v, err == nil
}

func validateCronField(field string, lo, hi int, names map[string]int, allowQ bool) error {
	for _, part := range strings.Split(field, ",") {
		expr, step, hasStep := strings.Cut(part, "/")
		if hasStep {
			n, err := strconv.Atoi(step)
			if err != nil || n <= 0 {
				return fmt.Errorf("bad step %q", step)
			}
		}
		if expr == "*" || (allowQ && expr == "?") {
			continue
		}
		a, b, isRange := strings.Cut(expr, "-")
		start, ok := cronValue(a, names)
		if !ok {
			return fmt.Errorf("bad value %q", a)
		}
		end := start
		if isRange {
			if end, ok = cronValue(b, names); !ok {
				return fmt.Errorf("bad value %q", b)
			}
		}
		if start < lo || end > hi || start > end {
			return fmt.Errorf("value out of range %d-%d", lo, hi)
		}
	}
	return nil
}

// ValidateTZ returns nil if tz is an IANA zone name. "" and "Local" are
// refused: a stored schedule must name its zone.
func ValidateTZ(tz string) error {
	if tz == "" || tz == "Local" || tz != strings.TrimSpace(tz) {
		return fmt.Errorf("%q is not an IANA time zone name", tz)
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("%q is not an IANA time zone name", tz)
	}
	return nil
}
