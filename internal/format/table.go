package format

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/gekart/helm-resources/internal/aggregate"
)

func renderTable(w io.Writer, rep aggregate.Report) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	label := groupLabel(rep.GroupBy)
	fmt.Fprintf(tw, "%s\tCPU REQ\tCPU LIM\tMEM REQ\tMEM LIM\tPODS\n", strings.ToUpper(label))
	for _, g := range rep.Groups {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n",
			g.Name,
			aggregate.FormatCPU(g.Totals.Requests.CPUMilli),
			aggregate.FormatCPU(g.Totals.Limits.CPUMilli),
			aggregate.FormatMemory(g.Totals.Requests.MemoryBytes),
			aggregate.FormatMemory(g.Totals.Limits.MemoryBytes),
			g.Totals.Pods,
		)
	}
	fmt.Fprintf(tw, "TOTAL\t%s\t%s\t%s\t%s\t%d\n",
		aggregate.FormatCPU(rep.Grand.Requests.CPUMilli),
		aggregate.FormatCPU(rep.Grand.Limits.CPUMilli),
		aggregate.FormatMemory(rep.Grand.Requests.MemoryBytes),
		aggregate.FormatMemory(rep.Grand.Limits.MemoryBytes),
		rep.Grand.Pods,
	)
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(rep.Notes) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Notes:")
		for _, n := range rep.Notes {
			fmt.Fprintf(w, "  - %s\n", n)
		}
	}
	return nil
}

func groupLabel(groupBy string) string {
	switch groupBy {
	case "kind":
		return "kind"
	case "namespace":
		return "namespace"
	case "none":
		return "total"
	default:
		return "subchart"
	}
}
