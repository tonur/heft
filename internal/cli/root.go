package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/tonur/heft/internal/scan"
)

// scanFunction is the function used by the CLI to run a scan.
// It is a variable to allow tests to inject a fake implementation.
var scanFunction = scan.Scan

// newRootCommand constructs the root heft command with the scan subcommand
// wired to call scanFunction.
func newRootCommand() *cobra.Command {
	heftCommand := &cobra.Command{
		Use:   "heft",
		Short: "Scan Helm charts for container images",
	}

	// Define the scan subcommand.
	scanCommand := &cobra.Command{
		Use:   "scan <chart-ref>",
		Short: "Scan a Helm chart for container images",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			chartRef := arguments[0]

			minConfidenceString, _ := command.Flags().GetString("min-confidence")
			noHelmDeps, _ := command.Flags().GetBool("no-helm-deps")
			includeOptionalDeps, _ := command.Flags().GetBool("include-optional-deps")
			verbose, _ := command.Flags().GetBool("verbose")
			setVals, _ := command.Flags().GetStringArray("set")
			setStringVals, _ := command.Flags().GetStringArray("set-string")
			valuesFiles, _ := command.Flags().GetStringArray("values")
			fValues, _ := command.Flags().GetStringArray("f")

			// Combine -f and --values inputs.
			valuesFiles = append(valuesFiles, fValues...)

			// Map min-confidence string to Confidence type.
			minConfidence := scan.ConfidenceLow
			switch minConfidenceString {
			case string(scan.ConfidenceHigh):
				minConfidence = scan.ConfidenceHigh
			case string(scan.ConfidenceMedium):
				minConfidence = scan.ConfidenceMedium
			case string(scan.ConfidenceLow):
				minConfidence = scan.ConfidenceLow
			}

			// Build combined Helm values flags in the same format
			// expected by scan.Options ("--set=key=val" style).
			var helmValues []string
			for _, v := range setVals {
				helmValues = append(helmValues, "--set="+v)
			}
			for _, v := range setStringVals {
				helmValues = append(helmValues, "--set-string="+v)
			}

			var helmValuesFiles []string
			for _, vf := range valuesFiles {
				helmValuesFiles = append(helmValuesFiles, "--values="+vf)
			}

			options := scan.Options{
				ChartPath:           chartRef,
				Values:              helmValues,
				ValuesFiles:         helmValuesFiles,
				HelmBin:             "helm",
				DisableHelmDeps:     noHelmDeps,
				IncludeOptionalDeps: includeOptionalDeps,
				MinConfidence:       minConfidence,
				Verbose:             verbose,
			}

			result, err := scanFunction(options)
			if err != nil {
				return err
			}

			encoder := yaml.NewEncoder(os.Stdout)
			defer encoder.Close()
			if err := encoder.Encode(result); err != nil {
				return fmt.Errorf("encode result: %w", err)
			}
			return nil
		},
	}

	scanCommand.Flags().String("min-confidence", string(scan.ConfidenceLow), "minimum image confidence to include (low|medium|high)")
	scanCommand.Flags().Bool("no-helm-deps", false, "disable automatic 'helm dependency build'")
	scanCommand.Flags().Bool("include-optional-deps", false, "include optional chart dependencies when scanning")
	scanCommand.Flags().BoolP("verbose", "v", false, "enable verbose logging")
	scanCommand.Flags().StringArray("set", nil, "set Helm values (key=val, repeatable)")
	scanCommand.Flags().StringArray("set-string", nil, "set Helm string values (key=val, repeatable)")
	scanCommand.Flags().StringArrayP("values", "f", nil, "values file (repeatable)")

	heftCommand.AddCommand(scanCommand)
	return heftCommand
}

// exitFunction is used by Execute to terminate the process. It is a
// variable so tests can stub it and observe exit behavior.
var exitFunction = os.Exit

// executeCommand runs the provided command and returns any error.
// It is a variable so tests can stub it.
var executeCommand = func(command *cobra.Command) error {
	return command.Execute()
}

// Execute is the entry point for the heft CLI.
func Execute() {
	command := newRootCommand()
	if err := executeCommand(command); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		exitFunction(1)
	}
}
