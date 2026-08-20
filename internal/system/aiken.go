package system

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner invokes the Aiken CLI for blueprint operations.
type Runner interface {
	Version(ctx context.Context) (string, error)
	Apply(ctx context.Context, workdir, inBlueprint, outBlueprint, module, validator, cborHexParam string) error
	Convert(ctx context.Context, workdir, blueprint, module, validator string) ([]byte, error)
	Hash(ctx context.Context, workdir, blueprint, module, validator string) (string, error)
}

// CLIRunner executes aiken via os/exec.
type CLIRunner struct {
	Bin string
}

// NewCLIRunner returns a runner using bin (default "aiken").
func NewCLIRunner(bin string) *CLIRunner {
	if strings.TrimSpace(bin) == "" {
		bin = "aiken"
	}
	return &CLIRunner{Bin: bin}
}

func (r *CLIRunner) Version(ctx context.Context) (string, error) {
	out, err := r.run(ctx, "", nil, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *CLIRunner) Apply(ctx context.Context, workdir, inBlueprint, outBlueprint, module, validator, cborHexParam string) error {
	args := []string{"blueprint", "apply"}
	inPath := resolveUnder(workdir, inBlueprint)
	outPath := resolveUnder(workdir, outBlueprint)
	if inPath != "" {
		args = append(args, "--in", inPath)
	}
	if outPath != "" {
		args = append(args, "--out", outPath)
	}
	if module != "" {
		args = append(args, "--module", module)
	}
	if validator != "" {
		args = append(args, "--validator", validator)
	}
	args = append(args, cborHexParam)
	_, err := r.run(ctx, workdir, nil, args...)
	return err
}

func resolveUnder(workdir, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	if workdir == "" {
		return path
	}
	return filepath.Join(workdir, path)
}

func (r *CLIRunner) Convert(ctx context.Context, workdir, blueprint, module, validator string) ([]byte, error) {
	if blueprint != "" {
		base := filepath.Base(blueprint)
		if base != "plutus.json" {
			return r.convertFromBlueprintFile(ctx, workdir, blueprint, module, validator)
		}
	}
	args := []string{"blueprint", "convert", "--to", "cardano-cli"}
	if module != "" {
		args = append(args, "--module", module)
	}
	if validator != "" {
		args = append(args, "--validator", validator)
	}
	return r.run(ctx, workdir, nil, args...)
}

func (r *CLIRunner) convertFromBlueprintFile(ctx context.Context, workdir, blueprint, module, validator string) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "aiken-convert-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	src := blueprint
	if !filepath.IsAbs(src) {
		src = filepath.Join(workdir, blueprint)
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return nil, err
	}
	dst := filepath.Join(tmpDir, "plutus.json")
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return nil, err
	}
	// Minimal aiken.toml so convert resolves the project.
	toml := "name = \"tmp/convert\"\nversion = \"0.0.0\"\nplutus = \"v3\"\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "aiken.toml"), []byte(toml), 0o644); err != nil {
		return nil, err
	}
	args := []string{"blueprint", "convert", "--to", "cardano-cli"}
	if module != "" {
		args = append(args, "--module", module)
	}
	if validator != "" {
		args = append(args, "--validator", validator)
	}
	return r.run(ctx, tmpDir, nil, args...)
}

func (r *CLIRunner) Hash(ctx context.Context, workdir, blueprint, module, validator string) (string, error) {
	args := []string{"blueprint", "hash"}
	if blueprint != "" {
		args = append(args, "--in", blueprint)
	}
	if module != "" {
		args = append(args, "--module", module)
	}
	if validator != "" {
		args = append(args, "--validator", validator)
	}
	out, err := r.run(ctx, workdir, nil, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *CLIRunner) run(ctx context.Context, workdir string, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, r.Bin, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("aiken %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}
