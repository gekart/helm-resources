package aggregate

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

// ParseCPU parses a Kubernetes CPU quantity ("100m", "0.5", "1", "1.5") and
// returns the value in millicores.
func ParseCPU(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	q, err := resource.ParseQuantity(s)
	if err != nil {
		return 0, fmt.Errorf("invalid cpu quantity %q: %w", s, err)
	}
	return q.MilliValue(), nil
}

// ParseMemory parses a Kubernetes memory quantity ("128Mi", "1Gi", "500M", "2G",
// "1024Ki", or a raw byte integer) and returns bytes.
func ParseMemory(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	q, err := resource.ParseQuantity(s)
	if err != nil {
		return 0, fmt.Errorf("invalid memory quantity %q: %w", s, err)
	}
	return q.Value(), nil
}

// FormatCPU renders a millicore value as a k8s-style CPU string: "1500m" unless
// it's zero or an exact whole core.
func FormatCPU(milli int64) string {
	if milli == 0 {
		return "0"
	}
	return fmt.Sprintf("%dm", milli)
}

// FormatMemory renders bytes with binary (Ki/Mi/Gi/Ti) prefixes. It picks the
// largest prefix that keeps the mantissa ≥ 1 and prints as an integer when the
// value is an exact multiple, or with one decimal place otherwise.
func FormatMemory(b int64) string {
	if b == 0 {
		return "0"
	}
	const (
		Ki = int64(1 << 10)
		Mi = int64(1 << 20)
		Gi = int64(1 << 30)
		Ti = int64(1 << 40)
	)
	units := []struct {
		v      int64
		suffix string
	}{
		{Ti, "Ti"}, {Gi, "Gi"}, {Mi, "Mi"}, {Ki, "Ki"},
	}
	for _, u := range units {
		if b < u.v {
			continue
		}
		if b%u.v == 0 {
			return fmt.Sprintf("%d%s", b/u.v, u.suffix)
		}
		return fmt.Sprintf("%.1f%s", float64(b)/float64(u.v), u.suffix)
	}
	return fmt.Sprintf("%d", b)
}
