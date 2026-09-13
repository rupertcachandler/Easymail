package eas

import (
	"testing"
	"time"
)

// SPEC: MS-ASCMD/format.datetime
func TestFormatDateTime_UTC(t *testing.T) {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	in := time.Date(2025, 4, 27, 19, 30, 5, 123456789, loc)
	got := FormatDateTime(in)
	want := "20250428T023005Z"
	if got != want {
		t.Fatalf("FormatDateTime = %q, want %q", got, want)
	}
}

// SPEC: MS-ASCMD/format.datetime
func TestParseDateTime_RoundTrip(t *testing.T) {
	in := "20250101T000000Z"
	parsed, err := ParseDateTime(in)
	if err != nil {
		t.Fatalf("ParseDateTime: %v", err)
	}
	if got := FormatDateTime(parsed); got != in {
		t.Fatalf("round-trip: got %q want %q", got, in)
	}
	if parsed.Location() != time.UTC {
		t.Fatalf("location = %v, want UTC", parsed.Location())
	}
}

// SPEC: MS-ASCMD/format.datetime
func TestParseDateTime_Fractional(t *testing.T) {
	got, err := ParseDateTime("20250101T000000.123Z")
	if err != nil {
		t.Fatalf("ParseDateTime fractional: %v", err)
	}
	want := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("ParseDateTime fractional = %v, want %v", got, want)
	}
}
