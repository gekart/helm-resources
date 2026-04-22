package format

import (
	"encoding/csv"
	"io"
	"strconv"

	"github.com/gekart/helm-resources/internal/aggregate"
)

func renderCSV(w io.Writer, rep aggregate.Report) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{
		groupLabel(rep.GroupBy),
		"cpu_requests_milli",
		"cpu_limits_milli",
		"memory_requests_bytes",
		"memory_limits_bytes",
		"pods",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, g := range rep.Groups {
		row := []string{
			g.Name,
			strconv.FormatInt(g.Totals.Requests.CPUMilli, 10),
			strconv.FormatInt(g.Totals.Limits.CPUMilli, 10),
			strconv.FormatInt(g.Totals.Requests.MemoryBytes, 10),
			strconv.FormatInt(g.Totals.Limits.MemoryBytes, 10),
			strconv.FormatInt(g.Totals.Pods, 10),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	total := []string{
		"TOTAL",
		strconv.FormatInt(rep.Grand.Requests.CPUMilli, 10),
		strconv.FormatInt(rep.Grand.Limits.CPUMilli, 10),
		strconv.FormatInt(rep.Grand.Requests.MemoryBytes, 10),
		strconv.FormatInt(rep.Grand.Limits.MemoryBytes, 10),
		strconv.FormatInt(rep.Grand.Pods, 10),
	}
	return cw.Write(total)
}
