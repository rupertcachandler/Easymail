// Package spec parses the spec-coverage matrix and the // SPEC: markers
// emitted by tests across the module, then reports any required spec
// requirement that is not covered by at least one test.
package spec

import (
	"encoding/csv"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Status is the lifecycle status of a spec requirement.
type Status string

const (
	StatusRequired   Status = "required"
	StatusOptional   Status = "optional"
	StatusOutOfScope Status = "out_of_scope"
)

// Requirement is one row from the spec-coverage matrix.
type Requirement struct {
	ID          string
	Doc         string
	Section     string
	Requirement string
	Status      Status
}

// Marker is a // SPEC: <id> reference found in a Go source file.
type Marker struct {
	ID   string
	File string
	Line int
}

// IssueKind classifies a coverage problem.
type IssueKind int

const (
	IssueUncovered IssueKind = iota + 1
	IssueUnknownMarker
)

// Issue is a coverage problem detected by Verify.
type Issue struct {
	Kind   IssueKind
	SpecID string
	Detail string
}

func (i Issue) String() string {
	switch i.Kind {
	case IssueUncovered:
		return fmt.Sprintf("uncovered required requirement: %s", i.SpecID)
	case IssueUnknownMarker:
		return fmt.Sprintf("unknown spec marker: %s (%s)", i.SpecID, i.Detail)
	default:
		return fmt.Sprintf("unknown issue %d for %s", i.Kind, i.SpecID)
	}
}

// ParseCSV reads a coverage CSV and returns its rows.
//
// The expected header is: spec_id,doc,section,requirement,status.
func ParseCSV(r io.Reader) ([]Requirement, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 5
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	wantHeader := []string{"spec_id", "doc", "section", "requirement", "status"}
	for i, h := range wantHeader {
		if header[i] != h {
			return nil, fmt.Errorf("header[%d] = %q, want %q", i, header[i], h)
		}
	}

	seen := map[string]struct{}{}
	var out []Requirement
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		req := Requirement{
			ID:          strings.TrimSpace(row[0]),
			Doc:         strings.TrimSpace(row[1]),
			Section:     strings.TrimSpace(row[2]),
			Requirement: strings.TrimSpace(row[3]),
			Status:      Status(strings.TrimSpace(row[4])),
		}
		switch req.Status {
		case StatusRequired, StatusOptional, StatusOutOfScope:
		default:
			return nil, fmt.Errorf("row %s: invalid status %q", req.ID, req.Status)
		}
		if req.ID == "" {
			return nil, fmt.Errorf("row with empty spec_id")
		}
		if _, dup := seen[req.ID]; dup {
			return nil, fmt.Errorf("duplicate spec_id %q", req.ID)
		}
		seen[req.ID] = struct{}{}
		out = append(out, req)
	}
	return out, nil
}

// LoadCSV parses the coverage CSV at path.
func LoadCSV(path string) ([]Requirement, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseCSV(f)
}

// markerPrefix is the comment prefix recognised by ScanMarkers; only the form
// "// SPEC: <id>" (a single-line comment, with mandatory colon and space)
// counts to keep the surface narrow and unambiguous.
const markerPrefix = "// SPEC: "

// ScanMarkers extracts // SPEC: <id> markers from a Go source file.
//
// It parses content as Go and inspects only real comment groups, so markers
// embedded inside string literals (for example test fixtures) are ignored.
// Files that fail to parse fall back to a line scan, which is good enough
// for malformed inputs where we still want best-effort marker discovery.
func ScanMarkers(file string, content []byte) []Marker {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, content, parser.ParseComments)
	if err != nil {
		return scanMarkersByLine(file, content)
	}
	var out []Marker
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			text := c.Text
			if !strings.HasPrefix(text, "//") {
				continue
			}
			body := strings.TrimLeft(text[2:], " \t")
			const tag = "SPEC: "
			if !strings.HasPrefix(body, tag) {
				continue
			}
			id := strings.TrimSpace(body[len(tag):])
			if id == "" {
				continue
			}
			pos := fset.Position(c.Slash)
			out = append(out, Marker{ID: id, File: file, Line: pos.Line})
		}
	}
	return out
}

func scanMarkersByLine(file string, content []byte) []Marker {
	var out []Marker
	for i, raw := range strings.Split(string(content), "\n") {
		s := strings.TrimLeft(raw, " \t")
		if !strings.HasPrefix(s, markerPrefix) {
			continue
		}
		id := strings.TrimSpace(s[len(markerPrefix):])
		if id == "" {
			continue
		}
		out = append(out, Marker{ID: id, File: file, Line: i + 1})
	}
	return out
}

// ScanTree walks root, scanning every *_test.go file for SPEC markers.
func ScanTree(root string) ([]Marker, error) {
	var out []Marker
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == ".git" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out = append(out, ScanMarkers(path, b)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Verify cross-references requirements against markers and reports issues.
//
//   - A required requirement with no marker yields an IssueUncovered.
//   - A marker referencing an id that is absent from the matrix yields an
//     IssueUnknownMarker.
//   - Optional / out_of_scope requirements without markers are silent.
func Verify(reqs []Requirement, markers []Marker) []Issue {
	known := make(map[string]Requirement, len(reqs))
	for _, r := range reqs {
		known[r.ID] = r
	}
	covered := make(map[string]bool, len(markers))
	var issues []Issue
	for _, m := range markers {
		covered[m.ID] = true
		if _, ok := known[m.ID]; !ok {
			issues = append(issues, Issue{
				Kind:   IssueUnknownMarker,
				SpecID: m.ID,
				Detail: fmt.Sprintf("%s:%d", m.File, m.Line),
			})
		}
	}
	for _, r := range reqs {
		if r.Status != StatusRequired {
			continue
		}
		if !covered[r.ID] {
			issues = append(issues, Issue{Kind: IssueUncovered, SpecID: r.ID})
		}
	}
	return issues
}
