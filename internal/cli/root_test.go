package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/tonur/heft/internal/scan"
)

// buildTestRoot constructs a Cobra root/scan command wired similarly to
// Execute, but replaces the scan RunE with a stub so we can assert flag
// parsing without invoking the real scanner.
func buildTestRoot(runScan func(command *cobra.Command, arguments []string) error) *cobra.Command {
	rootCommand := &cobra.Command{
		Use:   "heft",
		Short: "Scan Helm charts for container images",
	}

	scanCommand := &cobra.Command{
		Use:   "scan <chart-ref>",
		Short: "Scan a Helm chart for container images",
		Args:  cobra.ExactArgs(1),
		RunE:  runScan,
	}

	scanCommand.Flags().String("min-confidence", "low", "minimum image confidence to include (low|medium|high)")
	scanCommand.Flags().Bool("no-helm-deps", false, "disable automatic 'helm dependency build'")
	scanCommand.Flags().Bool("include-optional-deps", false, "include optional chart dependencies when scanning")
	scanCommand.Flags().BoolP("verbose", "v", false, "enable verbose logging")
	scanCommand.Flags().StringArray("set", nil, "set Helm values (key=val, repeatable)")
	scanCommand.Flags().StringArray("set-string", nil, "set Helm string values (key=val, repeatable)")
	scanCommand.Flags().StringArrayP("values", "f", nil, "values file (repeatable)")

	rootCommand.AddCommand(scanCommand)
	return rootCommand
}

func TestScanCommandParsesFlags(t *testing.T) {
	var (
		gotChartRef           string
		gotMinConf            string
		gotNoHelmDeps         bool
		gotIncludeOptionalDep bool
		gotVerbose            bool
		gotSet                []string
		gotSetString          []string
		gotValues             []string
	)

	rootCommand := buildTestRoot(func(command *cobra.Command, arguments []string) error {
		gotChartRef = arguments[0]
		gotMinConf, _ = command.Flags().GetString("min-confidence")
		gotNoHelmDeps, _ = command.Flags().GetBool("no-helm-deps")
		gotIncludeOptionalDep, _ = command.Flags().GetBool("include-optional-deps")
		gotVerbose, _ = command.Flags().GetBool("verbose")
		gotSet, _ = command.Flags().GetStringArray("set")
		gotSetString, _ = command.Flags().GetStringArray("set-string")
		gotValues, _ = command.Flags().GetStringArray("values")
		return nil
	})

	rootCommand.SetArgs([]string{
		"scan", "my-chart", "--min-confidence=high", "--no-helm-deps",
		"--include-optional-deps", "-v",
		"--set", "foo=bar", "--set-string", "baz=qux",
		"-f", "values.yaml",
	})

	if err := rootCommand.Execute(); err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if gotChartRef != "my-chart" {
		t.Fatalf("expected chartRef=my-chart, got %q", gotChartRef)
	}
	if gotMinConf != "high" {
		t.Fatalf("expected min-confidence=high, got %q", gotMinConf)
	}
	if !gotNoHelmDeps {
		t.Fatalf("expected no-helm-deps=true")
	}
	if !gotIncludeOptionalDep {
		t.Fatalf("expected include-optional-deps=true")
	}
	if !gotVerbose {
		t.Fatalf("expected verbose=true")
	}
	if len(gotSet) != 1 || gotSet[0] != "foo=bar" {
		t.Fatalf("expected set=[foo=bar], got %v", gotSet)
	}
	if len(gotSetString) != 1 || gotSetString[0] != "baz=qux" {
		t.Fatalf("expected set-string=[baz=qux], got %v", gotSetString)
	}
	if len(gotValues) != 1 || gotValues[0] != "values.yaml" {
		t.Fatalf("expected values=[values.yaml], got %v", gotValues)
	}
}

func TestRootHelpIncludesScan(t *testing.T) {
	rootCommand := buildTestRoot(func(command *cobra.Command, arguments []string) error { return nil })

	buf := &bytes.Buffer{}
	rootCommand.SetOut(buf)
	rootCommand.SetArgs([]string{"--help"})

	if err := rootCommand.Execute(); err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "scan        Scan a Helm chart for container images") &&
		!strings.Contains(out, "scan\tScan a Helm chart for container images") {
		t.Fatalf("expected help output to mention scan subcommand, got: %s", out)
	}
}

// TestNewRootCommandWiresScanOptions verifies that the real CLI wiring
// passes the expected options to scanFunction.
func TestNewRootCommandWiresScanOptions(t *testing.T) {
	old := scanFunction
	defer func() { scanFunction = old }()

	var gotOptions scan.Options
	scanFunction = func(opts scan.Options) (*scan.ScanResult, error) {
		gotOptions = opts
		return &scan.ScanResult{}, nil
	}

	command := newRootCommand()
	command.SetArgs([]string{
		"scan", "my-chart",
		"--min-confidence=high",
		"--no-helm-deps",
		"--include-optional-deps",
		"-v",
		"--set", "foo=bar",
		"--set-string", "baz=qux",
		"-f", "values.yaml",
	})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if gotOptions.ChartPath != "my-chart" {
		t.Fatalf("expected ChartPath=my-chart, got %q", gotOptions.ChartPath)
	}
	if gotOptions.MinConfidence != scan.ConfidenceHigh {
		t.Fatalf("expected MinConfidence=high, got %q", gotOptions.MinConfidence)
	}
	if !gotOptions.DisableHelmDeps {
		t.Fatalf("expected DisableHelmDeps=true")
	}
	if !gotOptions.IncludeOptionalDeps {
		t.Fatalf("expected IncludeOptionalDeps=true")
	}
	if !gotOptions.Verbose {
		t.Fatalf("expected Verbose=true")
	}
}
