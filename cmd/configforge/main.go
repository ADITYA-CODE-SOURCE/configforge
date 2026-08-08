package main

import (
	"fmt"
	"os"

	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "configforge",
		Short:         "Declarative feature and policy engine for Go applications",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(validateCommand())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func validateCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:     "validate --config config.yaml",
		Short:   "Validate a ConfigForge configuration file",
		Example: "configforge validate --config examples/configs/default.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			if configPath == "" {
				return fmt.Errorf("--config is required")
			}

			if _, err := config.LoadFile(configPath); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s is valid\n", configPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to the YAML configuration file")
	return cmd
}
