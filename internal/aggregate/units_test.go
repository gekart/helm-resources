package aggregate

import "testing"

func TestParseCPU(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"100m", 100},
		{"0.5", 500},
		{"1", 1000},
		{"1.5", 1500},
		{"2", 2000},
		{"250m", 250},
	}
	for _, tc := range tests {
		got, err := ParseCPU(tc.in)
		if err != nil {
			t.Fatalf("ParseCPU(%q) error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ParseCPU(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseCPU_Invalid(t *testing.T) {
	for _, in := range []string{"abc", "1x", "--1"} {
		if _, err := ParseCPU(in); err == nil {
			t.Errorf("ParseCPU(%q): expected error", in)
		}
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"128Mi", 128 * 1024 * 1024},
		{"1Gi", 1024 * 1024 * 1024},
		{"1024Ki", 1024 * 1024},
		{"500M", 500 * 1000 * 1000},
		{"2G", 2 * 1000 * 1000 * 1000},
		{"1024", 1024},
	}
	for _, tc := range tests {
		got, err := ParseMemory(tc.in)
		if err != nil {
			t.Fatalf("ParseMemory(%q) error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ParseMemory(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseMemory_Invalid(t *testing.T) {
	for _, in := range []string{"abc", "1Xi", "--1"} {
		if _, err := ParseMemory(in); err == nil {
			t.Errorf("ParseMemory(%q): expected error", in)
		}
	}
}

func TestFormatCPU(t *testing.T) {
	cases := map[int64]string{
		0:    "0",
		100:  "100m",
		1500: "1500m",
	}
	for in, want := range cases {
		if got := FormatCPU(in); got != want {
			t.Errorf("FormatCPU(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatMemory(t *testing.T) {
	cases := map[int64]string{
		0:                  "0",
		1024:               "1Ki",
		1024 * 1024:        "1Mi",
		1024 * 1024 * 1024: "1Gi",
		128 * 1024 * 1024:  "128Mi",
		1536 * 1024 * 1024: "1.5Gi",
		512:                "512",
	}
	for in, want := range cases {
		if got := FormatMemory(in); got != want {
			t.Errorf("FormatMemory(%d) = %q, want %q", in, got, want)
		}
	}
}
