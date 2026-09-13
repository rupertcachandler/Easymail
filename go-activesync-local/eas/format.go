package eas

import (
	"fmt"
	"strings"
	"time"
)

// dateTimeLayout is the canonical EAS DateTime pattern (UTC).
const dateTimeLayout = "20060102T150405Z"

// FormatDateTime returns t in EAS UTC pattern YYYYMMDDTHHMMSSZ.
func FormatDateTime(t time.Time) string {
	return t.UTC().Format(dateTimeLayout)
}

// ParseDateTime parses an EAS DateTime in UTC. It accepts the canonical
// YYYYMMDDTHHMMSSZ layout and the variant with a fractional seconds suffix
// (YYYYMMDDTHHMMSS.fffZ) which some servers emit.
func ParseDateTime(s string) (time.Time, error) {
	core := s
	if i := strings.IndexByte(core, '.'); i > 0 && strings.HasSuffix(core, "Z") {
		core = core[:i] + "Z"
	}
	t, err := time.ParseInLocation(dateTimeLayout, core, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("eas: invalid DateTime %q", s)
	}
	return t.Truncate(time.Second), nil
}
