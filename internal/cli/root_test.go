package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// buildTestRoot constructs a Cobra root/scan command wired similarly to
// Execute, but replaces the scan RunE with a stub so we can assert flag
// parsing without invoking the real scanner.
func buildTestRoot(runScan func(cmd *cobra.Command, args []string) error) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "heft",
		Short: "Scan Helm charts for container images",
	}

	scanCmd := &cobra.Command{
		Use:   "scan <chart-ref>",
		Short: "Scan a Helm chart for container images",
		Args:  cobra.ExactArgs(1),
		RunE:  runScan,
	}

	scanCmd.Flags().String("min-confidence", "low", "minimum image confidence to include (low|medium|high)")
	scanCmd.Flags().Bool("no-helm-deps", false, "disable automatic 'helm dependency build'")
	scanCmd.Flags().Bool("include-optional-deps", false, "include optional chart dependencies when scanning")
	scanCmd.Flags().BoolP("verbose", "v", false, "enable verbose logging")
	scanCmd.Flags().StringArray("set", nil, "set Helm values (key=val, repeatable)")
	scanCmd.Flags().StringArray("set-string", nil, "set Helm string values (key=val, repeatable)")
	scanCmd.Flags().StringArrayP("values", "f", nil, "values file (repeatable)")

	rootCmd.AddCommand(scanCmd)
	return rootCmd
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

	rootCmd := buildTestRoot(func(cmd *cobra.Command, args []string) error {
		gotChartRef = args[0]
		gotMinConf, _ = cmd.Flags().GetString("min-confidence")
		gotNoHelmDeps, _ = cmd.Flags().GetBool("no-helm-deps")
		gotIncludeOptionalDep, _ = cmd.Flags().GetBool("include-optional-deps")
		gotVerbose, _ = cmd.Flags().GetBool("verbose")
		gotSet, _ = cmd.Flags().GetStringArray("set")
		gotSetString, _ = cmd.Flags().GetStringArray("set-string")
		gotValues, _ = cmd.Flags().GetStringArray("values")
		return nil
	})

	rootCmd.SetArgs([]string{
		"scan", "my-chart", "--min-confidence=high", "--no-helm-deps",
		"--include-optional-deps", "-v",
		"--set", "foo=bar", "--set-string", "baz=qux",
		"-f", "values.yaml",
	})

	if err := rootCmd.Execute(); err != nil {
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
	rootCmd := buildTestRoot(func(cmd *cobra.Command, args []string) error { return nil })

	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "scan        Scan a Helm chart for container images") &&
		!strings.Contains(out, "scan	Scan a Helm chart for container images") {
		t.Fatalf("expected help output to mention scan subcommand, got: %s", out)
	}
}
