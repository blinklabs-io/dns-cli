package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage dns-cli configuration",
	}
	cmd.AddCommand(newConfigInitCmd(g))
	cmd.AddCommand(newConfigShowCmd(g))
	cmd.AddCommand(newConfigValidateCmd(g))
	return cmd
}

func newConfigInitCmd(g *GlobalFlags) *cobra.Command {
	var network, provider string
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starter JSON config file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if network == "" {
				network = "preview"
			}
			if provider == "" {
				provider = "utxorpc"
			}
			out := g.ConfigPath
			if out == "" {
				out = config.DefaultConfigPath
			}
			if _, err := os.Stat(out); err == nil && !force {
				return WrapExit(ExitConfig, fmt.Errorf("config file already exists: %s (use --force to overwrite)", out))
			}
			doc, err := config.DefaultDocument(network, provider)
			if err != nil {
				return WrapExit(ExitConfig, err)
			}
			if err := config.ApplyStarterRelativePaths(doc, out); err != nil {
				return WrapExit(ExitConfig, err)
			}
			if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil && filepath.Dir(out) != "." {
				return WrapExit(ExitConfig, err)
			}
			raw, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				return WrapExit(ExitInternal, err)
			}
			raw = append(raw, '\n')
			if err := os.WriteFile(out, raw, 0o600); err != nil {
				return WrapExit(ExitConfig, err)
			}
			return p.Success(Result{
				Command:   "config init",
				Network:   network,
				Message:   fmt.Sprintf("wrote config to %s", out),
				Operation: "config.init",
				Data: map[string]any{
					"path":     out,
					"provider": provider,
				},
			})
		},
	}
	cmd.Flags().StringVar(&network, "network", "preview", "network profile to initialize (preview|preprod|mainnet)")
	cmd.Flags().StringVar(&provider, "provider", "utxorpc", "default provider (utxorpc|blockfrost)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config file")
	return cmd
}

func newConfigShowCmd(g *GlobalFlags) *cobra.Command {
	var redact bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the effective configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			eff, err := loadEffective(g)
			if err != nil {
				return WrapExit(ExitConfig, err)
			}
			view := config.RedactedView(eff, redact)
			return p.Success(Result{
				Command:   "config show",
				Network:   eff.Profile.Network.Name,
				Operation: "config.show",
				Data:      view,
			})
		},
	}
	cmd.Flags().BoolVar(&redact, "redact", true, "redact secret-related fields (always recommended)")
	return cmd
}

func newConfigValidateCmd(g *GlobalFlags) *cobra.Command {
	var online bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration locally (and optionally online)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			var eff *config.Effective
			if online {
				eff, err = loadReadyEffective(cmd, g)
				if err != nil {
					return err
				}
			} else {
				eff, err = loadEffective(g)
				if err != nil {
					return WrapExit(ExitConfig, err)
				}
			}
			if err := config.ValidateOffline(eff); err != nil {
				return WrapExit(ExitValidation, err)
			}
			if online {
				ctx, cancel := context.WithTimeout(cmd.Context(), g.Timeout)
				defer cancel()
				if err := runConfigValidateOnline(ctx, eff); err != nil {
					return WrapExit(ExitProvider, err)
				}
			}
			return p.Success(Result{
				Command:   "config validate",
				Network:   eff.Profile.Network.Name,
				Operation: "config.validate",
				Message:   "configuration is valid",
			})
		},
	}
	cmd.Flags().BoolVar(&online, "online", false, "also validate provider connectivity and reference UTxOs")
	return cmd
}

func loadEffective(g *GlobalFlags) (*config.Effective, error) {
	path := g.ConfigPath
	if path == "" {
		path = config.DefaultConfigPath
	}
	overrides := config.Overrides{
		Network:     g.Network,
		Provider:    g.Provider,
		ArtifactDir: g.ArtifactDir,
	}
	return config.Load(path, overrides)
}
