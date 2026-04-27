// Package render wraps the Helm 4 SDK to produce a rendered manifest string
// from a chart on disk. It performs a client-side dry-run, never contacting a
// live cluster.
package render

import (
	"fmt"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/release"
)

// Options configures a render.
type Options struct {
	ChartPath   string
	Namespace   string
	ReleaseName string
	ValueOpts   values.Options
}

// Render loads the chart at opts.ChartPath (or repo alias, .tgz, OCI ref) and
// returns the rendered multi-doc YAML manifest.
func Render(opts Options) (string, error) {
	settings := cli.New()
	if opts.Namespace != "" {
		settings.SetNamespace(opts.Namespace)
	}

	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), settings.Namespace(), ""); err != nil {
		return "", fmt.Errorf("init helm config: %w", err)
	}

	inst := action.NewInstall(cfg)
	inst.DryRunStrategy = action.DryRunClient
	inst.ReleaseName = opts.ReleaseName
	if inst.ReleaseName == "" {
		inst.ReleaseName = "release-name"
	}
	inst.Namespace = opts.Namespace
	if inst.Namespace == "" {
		inst.Namespace = "default"
	}
	inst.IncludeCRDs = false
	inst.DisableHooks = false // hooks emit in the manifest; aggregate filters them via annotation

	providers := getter.All(settings)
	vals, err := opts.ValueOpts.MergeValues(providers)
	if err != nil {
		return "", fmt.Errorf("merge values: %w", err)
	}

	located, err := inst.LocateChart(opts.ChartPath, settings)
	if err != nil {
		return "", fmt.Errorf("locate chart: %w", err)
	}
	chrt, err := loader.Load(located)
	if err != nil {
		return "", fmt.Errorf("load chart: %w", err)
	}

	rel, err := inst.Run(chrt, vals)
	if err != nil {
		return "", fmt.Errorf("render chart: %w", err)
	}
	acc, err := release.NewAccessor(rel)
	if err != nil {
		return "", fmt.Errorf("read release: %w", err)
	}
	return acc.Manifest(), nil
}
