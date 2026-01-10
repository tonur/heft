package scan

import "fmt"

type Confidence string

type SourceKind string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"

	SourceRendered SourceKind = "rendered-manifest"
)

type ImageFinding struct {
	Name       string     `yaml:"name" json:"name"`
	Confidence Confidence `yaml:"confidence" json:"confidence"`
	Source     SourceKind `yaml:"source" json:"source"`
	File       string     `yaml:"file,omitempty" json:"file,omitempty"`
	Line       int        `yaml:"line,omitempty" json:"line,omitempty"`
}

type ScanResult struct {
	Images []ImageFinding `yaml:"images" json:"images"`
}

// Options controls a scan invocation.
type Options struct {
	// ChartPath is the chart reference passed to helm template.
	// It can be a local directory, a .tgz file, an HTTP(S) URL to a chart
	// archive, or an OCI reference.
	ChartPath           string
	Values              []string // Helm --set / --set-string values
	ValuesFiles         []string // Helm --values files
	HelmBin             string
	DisableHelmDeps     bool
	IncludeOptionalDeps bool
	MinConfidence       Confidence
	Verbose             bool
}

// Run is deprecated; the CLI is now implemented with Cobra in internal/cli.
// It is kept only for compatibility with any existing callers and will be
// removed in a future version.
func Run() error {
	return fmt.Errorf("scan.Run is deprecated; use the heft CLI entrypoint instead")
}
