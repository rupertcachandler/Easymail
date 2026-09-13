// Command covergate fails when per-package coverage falls below the
// thresholds defined in the project plan:
//
//	wbxml/        ≥ 90%
//	eas/          ≥ 90%
//	client/       ≥ 80%
//	autodiscover/ ≥ 80%
//
// It accepts one or more `go test -coverprofile` files on the command line
// (or the default cover.out) and aggregates per-package coverage from them.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type counts struct {
	covered int
	total   int
}

var thresholds = map[string]float64{
	"wbxml":        90,
	"eas":          90,
	"client":       80,
	"autodiscover": 80,
}

func main() {
	flag.Parse()
	files := flag.Args()
	if len(files) == 0 {
		files = []string{"cover.out"}
	}

	stats := map[string]*counts{}
	for _, f := range files {
		if err := readProfile(f, stats); err != nil {
			fatalf("covergate: %v", err)
		}
	}

	if len(stats) == 0 {
		fatalf("covergate: no coverage data found")
	}

	failed := 0
	pkgs := make([]string, 0, len(stats))
	for p := range stats {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)

	for _, pkg := range pkgs {
		c := stats[pkg]
		if c.total == 0 {
			continue
		}
		pct := 100 * float64(c.covered) / float64(c.total)
		thr, hasThr := thresholds[pkg]
		var mark string
		switch {
		case !hasThr:
			mark = "--"
		case pct+1e-9 < thr:
			mark = "FAIL"
			failed++
		default:
			mark = "OK"
		}
		fmt.Printf("%-15s %6.2f%% (%d/%d)  threshold=%.0f%%  %s\n", pkg, pct, c.covered, c.total, thr, mark)
	}

	if failed > 0 {
		fatalf("covergate: %d package(s) below threshold", failed)
	}
}

func readProfile(path string, stats map[string]*counts) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false
			if strings.HasPrefix(line, "mode:") {
				continue
			}
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Format: <import-path>/<file>:<startLine.col>,<endLine.col> <numStmt> <count>
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		fileRef := line[:colon]
		fields := strings.Fields(line[colon+1:])
		if len(fields) < 3 {
			continue
		}
		num, err1 := strconv.Atoi(fields[1])
		count, err2 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil {
			continue
		}
		pkg := topLevelPackage(fileRef)
		if pkg == "" {
			continue
		}
		c := stats[pkg]
		if c == nil {
			c = &counts{}
			stats[pkg] = c
		}
		c.total += num
		if count > 0 {
			c.covered += num
		}
	}
	return sc.Err()
}

// topLevelPackage extracts the first directory component after the module path.
// e.g. github.com/remdev/go-activesync/wbxml/marshal.go -> "wbxml".
func topLevelPackage(fileRef string) string {
	const module = "github.com/remdev/go-activesync/"
	if !strings.HasPrefix(fileRef, module) {
		return ""
	}
	rest := fileRef[len(module):]
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return ""
	}
	return rest[:slash]
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
