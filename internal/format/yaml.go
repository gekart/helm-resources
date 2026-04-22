package format

import (
	"io"

	"sigs.k8s.io/yaml"

	"github.com/gekart/helm-resources/internal/aggregate"
)

func renderYAML(w io.Writer, rep aggregate.Report) error {
	b, err := yaml.Marshal(rep)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}
