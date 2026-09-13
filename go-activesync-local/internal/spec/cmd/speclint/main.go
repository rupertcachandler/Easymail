// speclint validates the spec-coverage matrix against // SPEC: markers in
// every *_test.go file under the module root.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/remdev/go-activesync/internal/spec"
)

func main() {
	csvPath := flag.String("csv", "internal/spec/coverage.csv", "path to the coverage matrix")
	root := flag.String("root", ".", "module root to scan for _test.go files")
	flag.Parse()

	reqs, err := spec.LoadCSV(*csvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load csv: %v\n", err)
		os.Exit(2)
	}
	markers, err := spec.ScanTree(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan tree: %v\n", err)
		os.Exit(2)
	}
	issues := spec.Verify(reqs, markers)
	if len(issues) == 0 {
		fmt.Printf("spec-lint: %d requirements, %d markers, all green\n", len(reqs), len(markers))
		return
	}
	for _, i := range issues {
		fmt.Fprintln(os.Stderr, i)
	}
	fmt.Fprintf(os.Stderr, "spec-lint: %d issue(s)\n", len(issues))
	os.Exit(1)
}
