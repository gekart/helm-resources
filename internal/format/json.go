package format

import (
	"encoding/json"
	"io"

	"github.com/gekart/helm-resources/internal/aggregate"
)

// flatJSON is the fixed schema required by the acceptance criteria when
// --group-by=none is used. It has exactly these top-level numeric fields:
// cpuRequestsMilli, cpuLimitsMilli, memoryRequestsBytes, memoryLimitsBytes, pods.
type flatJSON struct {
	CPURequestsMilli    int64    `json:"cpuRequestsMilli"`
	CPULimitsMilli      int64    `json:"cpuLimitsMilli"`
	MemoryRequestsBytes int64    `json:"memoryRequestsBytes"`
	MemoryLimitsBytes   int64    `json:"memoryLimitsBytes"`
	Pods                int64    `json:"pods"`
	Notes               []string `json:"notes,omitempty"`
}

func renderJSON(w io.Writer, rep aggregate.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if rep.GroupBy == "none" {
		return enc.Encode(flatJSON{
			CPURequestsMilli:    rep.Grand.Requests.CPUMilli,
			CPULimitsMilli:      rep.Grand.Limits.CPUMilli,
			MemoryRequestsBytes: rep.Grand.Requests.MemoryBytes,
			MemoryLimitsBytes:   rep.Grand.Limits.MemoryBytes,
			Pods:                rep.Grand.Pods,
			Notes:               rep.Notes,
		})
	}
	return enc.Encode(rep)
}
