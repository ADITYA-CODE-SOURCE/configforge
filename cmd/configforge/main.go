package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ADITYA-CODE-SOURCE/configforge/internal/generator"
	"github.com/ADITYA-CODE-SOURCE/configforge/internal/manifest"
	"github.com/ADITYA-CODE-SOURCE/configforge/internal/schema"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/engine"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/feature"
	"github.com/spf13/cobra"
)

var errDenied = fmt.Errorf("denied")

func main() {
	root := &cobra.Command{
		Use:           "configforge",
		Short:         "Declarative feature and policy engine for Go applications",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(validateCommand())
	root.AddCommand(generateCommand())
	root.AddCommand(schemaCommand())
	root.AddCommand(explainCommand())
	root.AddCommand(checkFeatureCommand())
	root.AddCommand(checkRouteCommand())

	if err := root.Execute(); err != nil {
		if errors.Is(err, errDenied) {
			os.Exit(2)
		}
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

func generateCommand() *cobra.Command {
	var manifestsDir string
	var outputDir string

	cmd := &cobra.Command{
		Use:     "generate --manifests ./manifests",
		Short:   "Generate defaults, env metadata, documentation, and schema from manifests",
		Example: "configforge generate --manifests ./manifests --output ./generated",
		RunE: func(cmd *cobra.Command, args []string) error {
			if manifestsDir == "" {
				return fmt.Errorf("--manifests is required")
			}
			if outputDir == "" {
				outputDir = "."
			}

			manifests, err := manifest.LoadDir(manifestsDir)
			if err != nil {
				return err
			}

			defaultsGo := generator.GenerateDefaults(manifests)
			defaultsPath := filepath.Join(outputDir, "pkg", "config", "generated.go")
			if err := os.MkdirAll(filepath.Dir(defaultsPath), 0755); err != nil {
				return fmt.Errorf("create output dir: %w", err)
			}
			if err := os.WriteFile(defaultsPath, defaultsGo, 0644); err != nil {
				return fmt.Errorf("write defaults: %w", err)
			}

			docsMarkdown := generator.GenerateDocs(manifests)
			docsPath := filepath.Join(outputDir, "docs", "configuration_options.md")
			if err := os.MkdirAll(filepath.Dir(docsPath), 0755); err != nil {
				return fmt.Errorf("create docs dir: %w", err)
			}
			if err := os.WriteFile(docsPath, docsMarkdown, 0644); err != nil {
				return fmt.Errorf("write docs: %w", err)
			}

			schemaJSON, err := schema.Generate()
			if err != nil {
				return fmt.Errorf("generate schema: %w", err)
			}
			schemaPath := filepath.Join(outputDir, "schemas", "config.schema.json")
			if err := os.MkdirAll(filepath.Dir(schemaPath), 0755); err != nil {
				return fmt.Errorf("create schema dir: %w", err)
			}
			if err := os.WriteFile(schemaPath, schemaJSON, 0644); err != nil {
				return fmt.Errorf("write schema: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "generated: %s\n", defaultsPath)
			fmt.Fprintf(cmd.OutOrStdout(), "generated: %s\n", docsPath)
			fmt.Fprintf(cmd.OutOrStdout(), "generated: %s\n", schemaPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&manifestsDir, "manifests", "", "Directory containing manifest YAML files")
	cmd.Flags().StringVar(&outputDir, "output", ".", "Output directory root")
	return cmd
}

func schemaCommand() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:     "schema --output schemas/config.schema.json",
		Short:   "Generate the JSON Schema for ConfigForge configuration",
		Example: "configforge schema --output schemas/config.schema.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputPath == "" {
				return fmt.Errorf("--output is required")
			}

			data, err := schema.Generate()
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
				return fmt.Errorf("create schema dir: %w", err)
			}
			if err := os.WriteFile(outputPath, data, 0644); err != nil {
				return fmt.Errorf("write schema: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "schema written to %s\n", outputPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&outputPath, "output", "", "Path for the generated JSON Schema file")
	return cmd
}

func explainCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:     "explain <field-path>",
		Short:   "Explain a configuration option by dotted path",
		Example: "configforge explain features.new_checkout.rollout_percentage --config config.yaml",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fieldPath := args[0]
			cfg, err := config.LoadFile(configPath)
			if err != nil {
				return err
			}
			explanation := explain(*cfg, fieldPath)
			fmt.Fprintln(cmd.OutOrStdout(), explanation)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to the YAML configuration file")
	return cmd
}

func explain(cfg config.Config, path string) string {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return "unknown path"
	}

	switch parts[0] {
	case "version":
		return fmt.Sprintf("%s: configuration version, currently %q. Must be %q.", path, cfg.Version, config.SupportedVersion)
	case "features":
		if len(parts) < 2 {
			return "features: map of feature flags keyed by name"
		}
		name := parts[1]
		flag, ok := cfg.Features[name]
		if !ok {
			return fmt.Sprintf("features.%s: feature not configured", name)
		}
		if len(parts) == 2 {
			return fmt.Sprintf("features.%s: enabled=%t rollout=%d%%", name, flag.Enabled, flag.RolloutPercentage)
		}
		switch parts[2] {
		case "enabled":
			return fmt.Sprintf("%s: boolean, currently %t", path, flag.Enabled)
		case "rollout_percentage":
			return fmt.Sprintf("%s: integer 0-100, currently %d", path, flag.RolloutPercentage)
		case "conditions":
			if len(parts) == 3 {
				return fmt.Sprintf("%s: targeting conditions (countries=%v users=%v roles=%v)", path, flag.Conditions.Countries, flag.Conditions.Users, flag.Conditions.Roles)
			}
			switch parts[3] {
			case "countries":
				return fmt.Sprintf("%s: %v", path, flag.Conditions.Countries)
			case "users":
				return fmt.Sprintf("%s: %v", path, flag.Conditions.Users)
			case "roles":
				return fmt.Sprintf("%s: %v", path, flag.Conditions.Roles)
			}
		}
	case "security":
		if len(parts) < 2 {
			return "security: route policy configuration"
		}
		if parts[1] == "routes" && len(parts) >= 3 {
			return fmt.Sprintf("%s: see routes in configuration", path)
		}
	case "privacy":
		switch parts[1] {
		case "redact_headers":
			return fmt.Sprintf("%s: %v (case-insensitive)", path, cfg.Privacy.RedactHeaders)
		case "redact_query_parameters":
			return fmt.Sprintf("%s: %v", path, cfg.Privacy.RedactQueryParameters)
		case "redact_json_fields":
			return fmt.Sprintf("%s: %v (dotted paths)", path, cfg.Privacy.RedactJSONFields)
		case "replacement":
			return fmt.Sprintf("%s: %q", path, cfg.Privacy.Replacement)
		}
	case "logging":
		switch parts[1] {
		case "level":
			return fmt.Sprintf("%s: %q (one of debug, info, warn, error)", path, cfg.Logging.Level)
		case "include_request_id":
			return fmt.Sprintf("%s: %t", path, cfg.Logging.IncludeRequestID)
		}
	}
	return fmt.Sprintf("%s: unknown configuration option", path)
}

func checkFeatureCommand() *cobra.Command {
	var configPath, featureName, userID, country string
	var roles []string

	cmd := &cobra.Command{
		Use:   "check-feature --config config.yaml --feature new_checkout --user user-101 --country IN",
		Short: "Evaluate a feature flag for a given user context",
		Example: `configforge check-feature --config config.yaml \
  --feature new_checkout --user user-101 --country IN`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if configPath == "" {
				return fmt.Errorf("--config is required")
			}
			if featureName == "" {
				return fmt.Errorf("--feature is required")
			}

			cfg, err := config.LoadFile(configPath)
			if err != nil {
				return err
			}
			e, err := engine.Compile(*cfg)
			if err != nil {
				return err
			}

			ctx := feature.EvaluationContext{
				UserID:  userID,
				Country: country,
				Roles:   roles,
			}
			dec := e.EvaluateFeature(featureName, ctx)

			fmt.Fprintf(cmd.OutOrStdout(), "enabled=%t\n", dec.Enabled)
			fmt.Fprintf(cmd.OutOrStdout(), "reason=%s\n", dec.Reason)
			fmt.Fprintf(cmd.OutOrStdout(), "rule=%s\n", dec.Rule)

			if !dec.Enabled {
				return errDenied
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to the YAML configuration file")
	cmd.Flags().StringVar(&featureName, "feature", "", "Feature flag name to evaluate")
	cmd.Flags().StringVar(&userID, "user", "", "User ID for evaluation context")
	cmd.Flags().StringVar(&country, "country", "", "Country code for evaluation context")
	cmd.Flags().StringSliceVar(&roles, "role", nil, "Roles for evaluation context (repeatable)")
	return cmd
}

func checkRouteCommand() *cobra.Command {
	var configPath, path, method string
	var authenticated bool
	var roles []string

	cmd := &cobra.Command{
		Use:   "check-route --config config.yaml --path /api/payments/create --method POST --authenticated --role customer",
		Short: "Evaluate a route policy for a given request",
		Example: `configforge check-route --config config.yaml \
  --path /api/payments/create --method POST --authenticated --role customer`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if configPath == "" {
				return fmt.Errorf("--config is required")
			}
			if path == "" {
				return fmt.Errorf("--path is required")
			}
			if method == "" {
				return fmt.Errorf("--method is required")
			}

			cfg, err := config.LoadFile(configPath)
			if err != nil {
				return err
			}
			e, err := engine.Compile(*cfg)
			if err != nil {
				return err
			}

			req := engine.Request{
				Method:        method,
				Path:          path,
				Authenticated: authenticated,
				Roles:         roles,
			}
			dec := e.EvaluateRequest(req)

			fmt.Fprintf(cmd.OutOrStdout(), "allowed=%t\n", dec.Allowed)
			fmt.Fprintf(cmd.OutOrStdout(), "reason=%s\n", dec.Reason)
			fmt.Fprintf(cmd.OutOrStdout(), "matched_policy=%s\n", dec.MatchedPolicy)

			if !dec.Allowed {
				return errDenied
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to the YAML configuration file")
	cmd.Flags().StringVar(&path, "path", "", "Request path to evaluate")
	cmd.Flags().StringVar(&method, "method", "", "HTTP method to evaluate")
	cmd.Flags().BoolVar(&authenticated, "authenticated", false, "Whether the request is authenticated")
	cmd.Flags().StringSliceVar(&roles, "role", nil, "Roles for the request (repeatable)")
	return cmd
}
