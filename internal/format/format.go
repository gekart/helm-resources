// Package format renders an aggregate.Report as table/json/yaml/csv.
package format

import (
	"fmt"
	"io"

	"github.com/gekart/helm-resources/internal/aggregate"
)

// Render writes the report to w in the requested format.
func Render(w io.Writer, rep aggregate.Report, format string) error {
	switch format {
	case "", "table":
		return renderTable(w, rep)
	case "json":
		return renderJSON(w, rep)
	case "yaml":
		return renderYAML(w, rep)
	case "csv":
		return renderCSV(w, rep)
	default:
		return fmt.Errorf("unknown output format %q (want one of: table, json, yaml, csv)", format)
	}
}
