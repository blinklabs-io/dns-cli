package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/prereq"
	"github.com/blinklabs-io/dns-cli/internal/system"
	"github.com/blinklabs-io/dns-cli/internal/txbuilder"
	"github.com/spf13/cobra"
)

func newSystemCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Prepare, initialize, and bind Handshake DNS system validators",
	}
	cmd.AddCommand(newSystemPrepareCmd(g))
	cmd.AddCommand(newSystemInitCmd(g))
	cmd.AddCommand(newSystemBindCmd(g))
	return cmd
}

func newSystemPrepareCmd(g *GlobalFlags) *cobra.Command {
	var blueprint, registrarHNS, registrarKeyAlias, stakeKey, network, outDir, aikenPath string
	var force bool
	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Build and parameterize system validators into deployment.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			registrarKey := firstNonEmptyFlag(registrarHNS, registrarKeyAlias)
			if registrarHNS != "" && registrarKeyAlias != "" && registrarHNS != registrarKeyAlias {
				return WrapExit(ExitUsage, fmt.Errorf("--registrar-hns-key and --registrar-key must match when both are set"))
			}
			if registrarKey == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--registrar-hns-key (or --registrar-key) is required"))
			}
			if network == "" {
				network = "preprod"
			}
			resolvedBlueprint, err := prereq.EnsureBlueprintDir(blueprint, prereq.Options{
				StartDir:   firstNonEmptyPath(blueprint, "."),
				AssumeYes:  false,
				Stdout:     cmd.OutOrStdout(),
				Stderr:     cmd.ErrOrStderr(),
				ConfirmYes: nil, // non-interactive prepare: only auto-use existing clone
			})
			if err != nil {
				// If blueprint already exists on disk, use it; otherwise surface prereq error.
				if !prereq.ContractsOK(blueprint) {
					return WrapExit(ExitBuild, err)
				}
				resolvedBlueprint = blueprint
			}
			blueprint = resolvedBlueprint
			result, err := system.PrepareDeployment(cmd.Context(), system.PrepareOptions{
				BlueprintDir:    blueprint,
				RegistrarHNSKey: registrarKey,
				StakeKeyPath:    stakeKey,
				Network:         network,
				OutDir:          outDir,
				AikenBin:        aikenPath,
				Force:           force,
			})
			if err != nil {
				return WrapExit(ExitBuild, err)
			}
			validators := map[string]any{}
			for role, v := range result.Deployment.Validators {
				validators[role] = map[string]any{
					"policyId":   v.PolicyID,
					"address":    v.Address,
					"plutusFile": v.PlutusFile,
				}
			}
			return p.Success(Result{
				Command:   "system prepare",
				Network:   result.Deployment.Network,
				Operation: "system.prepare",
				Artifact:  result.DeploymentPath,
				Message:   "prepared parameterized system validators",
				Data: map[string]any{
					"deployment":      result.DeploymentPath,
					"outDir":          outDir,
					"stakeKeyHash":    result.Deployment.StakeKeyHash,
					"registrarHnsKey": result.Deployment.RegistrarHNSKey,
					"validators":      validators,
				},
			})
		},
	}
	cmd.Flags().StringVar(&blueprint, "blueprint", "", "Aiken project directory containing aiken.toml")
	cmd.Flags().StringVar(&registrarHNS, "registrar-hns-key", "", "registrar HNS key JSON (registrar.hns)")
	cmd.Flags().StringVar(&registrarKeyAlias, "registrar-key", "", "alias for --registrar-hns-key")
	cmd.Flags().StringVar(&stakeKey, "stake-key", "", "stake.vkey envelope or wallet directory containing stake.vkey")
	cmd.Flags().StringVar(&network, "network", "preprod", "network profile (preprod only)")
	cmd.Flags().StringVar(&outDir, "out-dir", "", "output directory for plutus envelopes and deployment.json")
	cmd.Flags().StringVar(&aikenPath, "aiken", "", "path to aiken binary (default: aiken on PATH)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing deployment artifacts")
	_ = cmd.MarkFlagRequired("blueprint")
	_ = cmd.MarkFlagRequired("stake-key")
	_ = cmd.MarkFlagRequired("out-dir")
	return cmd
}

func newSystemInitCmd(g *GlobalFlags) *cobra.Command {
	var configPath, deploymentPath, actor, out string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Build unsigned tx publishing three system reference scripts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			if actor == "" {
				actor = "bootstrap"
			}
			if out == "" {
				return WrapExit(ExitUsage, fmt.Errorf("--out is required"))
			}
			if configPath != "" {
				g.ConfigPath = configPath
			}
			eff, err := loadReadyEffective(cmd, g)
			if err != nil {
				return err
			}
			dep, err := system.LoadDeploymentJSON(deploymentPath)
			if err != nil {
				return WrapExit(ExitValidation, err)
			}
			bctx, err := txbuilder.NewFundingContext(cmd.Context(), eff)
			if err != nil {
				return mapContextErr(err)
			}
			outBuild, err := txbuilder.SystemInit(cmd.Context(), bctx, txbuilder.SystemInitOptions{
				Deployment:       dep,
				DeploymentDir:    filepath.Dir(deploymentPath),
				BootstrapActor:   actor,
				OutPrefix:        out,
				ContractRevision: ContractRevision,
			})
			if err != nil {
				return WrapExit(ExitBuild, err)
			}
			return p.Success(Result{
				Command:   "system init",
				Network:   eff.Profile.Network.Name,
				Operation: "system.init",
				Artifact:  outBuild.EnvelopePath,
				Message:   "built unsigned system init transaction",
				Data: map[string]any{
					"deployment": deploymentPath,
					"actor":      actor,
					"out":        out,
					"bodyHash":   outBuild.BodyHash,
					"manifest":   outBuild.ManifestPath,
				},
			})
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "dns-cli JSON config (required)")
	cmd.Flags().StringVar(&deploymentPath, "deployment", "", "deployment.json from system prepare")
	cmd.Flags().StringVar(&actor, "actor", "bootstrap", "bootstrap funding/signing actor")
	cmd.Flags().StringVar(&out, "out", "", "output path prefix for unsigned envelope and manifest (writes <out>.unsigned.json + <out>.manifest.json)")
	_ = cmd.MarkFlagRequired("config")
	_ = cmd.MarkFlagRequired("deployment")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

func newSystemBindCmd(g *GlobalFlags) *cobra.Command {
	var baseConfig, deployment, txID, actorDir, provider, out string
	var force bool
	cmd := &cobra.Command{
		Use:   "bind",
		Short: "Merge deployment + init tx into a runnable dns-cli config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := printerFromFlags(g, cmd)
			if err != nil {
				return err
			}
			doc, err := system.BindConfig(system.BindOptions{
				BaseConfigPath: baseConfig,
				DeploymentPath: deployment,
				TxID:           txID,
				ActorDir:       actorDir,
				Provider:       provider,
				OutPath:        out,
				Force:          force,
			})
			if err != nil {
				msg := err.Error()
				switch {
				case strings.Contains(msg, "required"),
					strings.Contains(msg, "must be"),
					strings.Contains(msg, "unsupported"),
					strings.Contains(msg, "already exists"):
					return WrapExit(ExitValidation, err)
				default:
					return WrapExit(ExitConfig, err)
				}
			}
			profile := doc.DefaultProfile
			prof := doc.Profiles[profile]
			return p.Success(Result{
				Command:   "system bind",
				Network:   "preprod",
				Operation: "system.bind",
				Artifact:  out,
				Message:   "wrote bound dns-cli config",
				Data: map[string]any{
					"out":      out,
					"provider": provider,
					"txId":     strings.ToLower(strings.TrimSpace(txID)),
					"contracts": map[string]any{
						"tldRegistrarPolicyId": prof.Contracts.TLDRegistrarPolicyID,
						"tldReferencePolicyId": prof.Contracts.TLDReferencePolicyID,
						"sldReferencePolicyId": prof.Contracts.SLDReferencePolicyID,
						"referenceUtxos":       prof.Contracts.ReferenceUtxos,
					},
				},
			})
		},
	}
	cmd.Flags().StringVar(&baseConfig, "base-config", "", "template dns-cli JSON config")
	cmd.Flags().StringVar(&deployment, "deployment", "", "deployment.json from system prepare")
	cmd.Flags().StringVar(&txID, "tx-id", "", "submitted system init transaction id (hex)")
	cmd.Flags().StringVar(&actorDir, "actor-dir", "", "directory containing actor wallet folders")
	cmd.Flags().StringVar(&provider, "provider", "", "provider type (blockfrost|utxorpc)")
	cmd.Flags().StringVar(&out, "out", "", "output config path")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing output config")
	_ = cmd.MarkFlagRequired("base-config")
	_ = cmd.MarkFlagRequired("deployment")
	_ = cmd.MarkFlagRequired("tx-id")
	_ = cmd.MarkFlagRequired("actor-dir")
	_ = cmd.MarkFlagRequired("provider")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

func firstNonEmptyPath(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "."
}
