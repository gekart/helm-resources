package format

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gekart/helm-resources/internal/aggregate"
)

// TestRenderTable_TotalAlignsWithDataRows guards against the regression where
// a no-tab separator line in the middle of the table broke tabwriter's
// alignment, causing the TOTAL row to render with its own (tighter) spacing.
func TestRenderTable_TotalAlignsWithDataRows(t *testing.T) {
	rep := aggregate.Report{
		GroupBy: "subchart",
		Groups: []aggregate.Bucket{{
			Name: "nginx-ingress-controller",
			Totals: aggregate.Totals{
				Requests: aggregate.Resources{CPUMilli: 200, MemoryBytes: 256 * 1024 * 1024},
				Limits:   aggregate.Resources{CPUMilli: 300, MemoryBytes: 384 * 1024 * 1024},
				Pods:     2,
			},
		}},
		Grand: aggregate.Totals{
			Requests: aggregate.Resources{CPUMilli: 200, MemoryBytes: 256 * 1024 * 1024},
			Limits:   aggregate.Resources{CPUMilli: 300, MemoryBytes: 384 * 1024 * 1024},
			Pods:     2,
		},
	}

	var buf bytes.Buffer
	if err := Render(&buf, rep, "table"); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines (header, data, total), got %d:\n%s", len(lines), out)
	}

	// First column is left-justified and padded by tabwriter; with the bug
	// the TOTAL row had only a 2-space gap to the next column. Locking in
	// that the first-column width matches across data and total rows.
	header, data, total := lines[0], lines[1], lines[2]
	col2 := func(s string) int { return strings.Index(s, "CPU REQ") }
	if col2(header) <= 0 {
		t.Fatalf("CPU REQ not found in header: %q", header)
	}
	dataPrefix := strings.TrimRight(data[:col2(header)], " ")
	totalPrefix := strings.TrimRight(total[:col2(header)], " ")
	if dataPrefix != "nginx-ingress-controller" {
		t.Errorf("data first column: %q", dataPrefix)
	}
	if totalPrefix != "TOTAL" {
		t.Errorf("total first column: %q (col2 offset=%d)", totalPrefix, col2(header))
	}

	if strings.Contains(out, "---") {
		t.Errorf("unexpected separator line in output:\n%s", out)
	}
}
