package spec

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// SPEC: SPEC-LINT/self-test
func TestParseCSV_Valid(t *testing.T) {
	const csv = `spec_id,doc,section,requirement,status
A/1,DOC-A,§1,requirement one,required
A/2,DOC-A,§2,requirement two,optional
B/1,DOC-B,§1,requirement three,out_of_scope
`
	got, err := ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	want := []Requirement{
		{ID: "A/1", Doc: "DOC-A", Section: "§1", Requirement: "requirement one", Status: StatusRequired},
		{ID: "A/2", Doc: "DOC-A", Section: "§2", Requirement: "requirement two", Status: StatusOptional},
		{ID: "B/1", Doc: "DOC-B", Section: "§1", Requirement: "requirement three", Status: StatusOutOfScope},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCSV got %+v want %+v", got, want)
	}
}

// SPEC: SPEC-LINT/self-test
func TestParseCSV_RejectsBadStatus(t *testing.T) {
	const csv = `spec_id,doc,section,requirement,status
A/1,DOC-A,§1,r,bogus
`
	if _, err := ParseCSV(strings.NewReader(csv)); err == nil {
		t.Fatalf("ParseCSV: expected error for bogus status")
	}
}

// SPEC: SPEC-LINT/self-test
func TestParseCSV_RejectsDuplicateID(t *testing.T) {
	const csv = `spec_id,doc,section,requirement,status
A/1,DOC-A,§1,r,required
A/1,DOC-A,§2,other,required
`
	if _, err := ParseCSV(strings.NewReader(csv)); err == nil {
		t.Fatalf("ParseCSV: expected error for duplicate spec_id")
	}
}

// SPEC: SPEC-LINT/self-test
func TestScanMarkers_FindsAll(t *testing.T) {
	const file = `package x

// SPEC: A/1
func TestOne(t *testing.T) {}

// not a marker: SPEC A/2
func TestTwo(t *testing.T) {}

// SPEC: A/3
// SPEC: B/1
func TestThree(t *testing.T) {}
`
	got := ScanMarkers("x.go", []byte(file))
	sort.Slice(got, func(i, j int) bool { return got[i].ID < got[j].ID })
	want := []Marker{
		{ID: "A/1", File: "x.go", Line: 3},
		{ID: "A/3", File: "x.go", Line: 9},
		{ID: "B/1", File: "x.go", Line: 10},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ScanMarkers got %+v want %+v", got, want)
	}
}

// SPEC: SPEC-LINT/self-test
func TestVerify_AllCovered(t *testing.T) {
	reqs := []Requirement{{ID: "A/1", Status: StatusRequired}}
	markers := []Marker{{ID: "A/1", File: "x.go", Line: 1}}
	issues := Verify(reqs, markers)
	if len(issues) != 0 {
		t.Fatalf("Verify: want no issues, got %v", issues)
	}
}

// SPEC: SPEC-LINT/self-test
func TestVerify_RequiredUncovered(t *testing.T) {
	reqs := []Requirement{{ID: "A/1", Status: StatusRequired}}
	issues := Verify(reqs, nil)
	if len(issues) != 1 || issues[0].Kind != IssueUncovered || issues[0].SpecID != "A/1" {
		t.Fatalf("Verify: want 1 uncovered issue for A/1, got %v", issues)
	}
}

// SPEC: SPEC-LINT/self-test
func TestVerify_OptionalUncovered_NoIssue(t *testing.T) {
	reqs := []Requirement{{ID: "A/1", Status: StatusOptional}}
	issues := Verify(reqs, nil)
	if len(issues) != 0 {
		t.Fatalf("Verify: optional uncovered must not be an issue, got %v", issues)
	}
}

// SPEC: SPEC-LINT/self-test
func TestVerify_OutOfScopeUncovered_NoIssue(t *testing.T) {
	reqs := []Requirement{{ID: "A/1", Status: StatusOutOfScope}}
	issues := Verify(reqs, nil)
	if len(issues) != 0 {
		t.Fatalf("Verify: out_of_scope uncovered must not be an issue, got %v", issues)
	}
}

// SPEC: SPEC-LINT/self-test
func TestVerify_UnknownMarker(t *testing.T) {
	reqs := []Requirement{{ID: "A/1", Status: StatusRequired}}
	markers := []Marker{
		{ID: "A/1", File: "x.go", Line: 1},
		{ID: "A/2", File: "x.go", Line: 2},
	}
	issues := Verify(reqs, markers)
	if len(issues) != 1 || issues[0].Kind != IssueUnknownMarker || issues[0].SpecID != "A/2" {
		t.Fatalf("Verify: want unknown marker issue for A/2, got %v", issues)
	}
}
