package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"helm.sh/helm/v4/pkg/cli/values"

	"github.com/gekart/helm-resources/internal/aggregate"
	"github.com/gekart/helm-resources/internal/format"
	"github.com/gekart/helm-resources/internal/parse"
	"github.com/gekart/helm-resources/internal/render"
)

// version is set via -ldflags during build.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := pflag.NewFlagSet("helm-resources", pflag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		valueFiles   []string
		setValues    []string
		setStrings   []string
		setFiles     []string
		setJSON      []string
		namespace    string
		groupBy      string
		output       string
		nodes        int
		includeInit  bool
		warnMissing  bool
		stdin        bool
		includeHooks bool
		showVersion  bool
		releaseName  string
	)

	fs.StringArrayVarP(&valueFiles, "values", "f", nil, "values file(s), same semantics as `helm template`")
	fs.StringArrayVar(&setValues, "set", nil, "inline value overrides")
	fs.StringArrayVar(&setStrings, "set-string", nil, "inline string overrides")
	fs.StringArrayVar(&setFiles, "set-file", nil, "values from files")
	fs.StringArrayVar(&setJSON, "set-json", nil, "JSON value overrides")
	fs.StringVarP(&namespace, "namespace", "n", "", "release namespace (affects templating, not grouping)")
	fs.StringVar(&releaseName, "release-name", "release-name", "release name used for templating")
	fs.StringVar(&groupBy, "group-by", "subchart", "subchart | kind | namespace | none")
	fs.StringVarP(&output, "output", "o", "table", "table | json | yaml | csv")
	fs.IntVar(&nodes, "nodes", 0, "node count for DaemonSet multiplication (0 = report per-node)")
	fs.BoolVar(&includeInit, "include-init", true, "count init containers in totals")
	fs.BoolVar(&warnMissing, "warn-missing", true, "print warnings for containers with no resources")
	fs.BoolVar(&includeHooks, "include-hooks", false, "include helm.sh/hook-annotated workloads in totals")
	fs.BoolVar(&stdin, "stdin", false, "read pre-rendered manifests from stdin instead of rendering")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: helm resources CHART [flags]")
		fmt.Fprintln(os.Stderr, "       helm resources --stdin < manifests.yaml")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if showVersion {
		fmt.Println(version)
		return nil
	}

	if err := validateGroupBy(groupBy); err != nil {
		return err
	}

	// Step 1: obtain manifests — either from stdin or by rendering a chart.
	var manifests string
	if stdin {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		manifests = string(b)
	} else {
		if fs.NArg() < 1 {
			fs.Usage()
			return fmt.Errorf("missing chart argument")
		}
		chartPath := fs.Arg(0)

		valueOpts := values.Options{
			ValueFiles:   valueFiles,
			StringValues: setStrings,
			Values:       setValues,
			FileValues:   setFiles,
			JSONValues:   setJSON,
		}
		m, err := render.Render(render.Options{
			ChartPath:   chartPath,
			Namespace:   namespace,
			ReleaseName: releaseName,
			ValueOpts:   valueOpts,
		})
		if err != nil {
			return err
		}
		manifests = m
	}

	// Step 2: parse.
	workloads, notes, err := parse.Parse(strings.NewReader(manifests), parse.ParseOptions{})
	if err != nil {
		return err
	}
	for _, n := range notes {
		fmt.Fprintln(os.Stderr, "note:", n)
	}

	// Step 3: aggregate.
	report, err := aggregate.Aggregate(workloads, aggregate.Options{
		GroupBy:      groupBy,
		Nodes:        nodes,
		IncludeInit:  includeInit,
		WarnMissing:  warnMissing,
		IncludeHooks: includeHooks,
	})
	if err != nil {
		return err
	}

	// Step 4: format. Warnings → stderr; results → stdout.
	for _, wmsg := range report.Warnings {
		fmt.Fprintln(os.Stderr, "warning:", wmsg)
	}
	return format.Render(os.Stdout, report, output)
}

func validateGroupBy(g string) error {
	switch g {
	case "subchart", "kind", "namespace", "none":
		return nil
	}
	return fmt.Errorf("invalid --group-by %q (want one of: subchart, kind, namespace, none)", g)
}
