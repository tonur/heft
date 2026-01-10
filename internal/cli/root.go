package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/tonur/heft/internal/scan"
)

// Execute is the entry point for the heft CLI.
func Execute() {
	heftCommand := &cobra.Command{
		Use:   "heft",
		Short: "Scan Helm charts for container images",
	}

	// Define the scan subcommand.
	scanCommand := &cobra.Command{
		Use:   "scan <chart-ref>",
		Short: "Scan a Helm chart for container images",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chartRef := args[0]

			minConfidenceString, _ := cmd.Flags().GetString("min-confidence")
			noHelmDeps, _ := cmd.Flags().GetBool("no-helm-deps")
			includeOptionalDeps, _ := cmd.Flags().GetBool("include-optional-deps")
			verbose, _ := cmd.Flags().GetBool("verbose")
			setVals, _ := cmd.Flags().GetStringArray("set")
			setStringVals, _ := cmd.Flags().GetStringArray("set-string")
			valuesFiles, _ := cmd.Flags().GetStringArray("values")
			fValues, _ := cmd.Flags().GetStringArray("f")

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

			result, err := scan.Scan(options)
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

	if err := heftCommand.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
