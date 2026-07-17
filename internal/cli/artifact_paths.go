package cli

import "github.com/blinklabs-io/dns-cli/internal/artifact"

// unsignedArtifactData returns JSON-friendly paths for a build --out prefix.
// Artifact is the unsigned envelope path returned by builders.
func unsignedArtifactData(outPrefix, artifactPath string, extra map[string]any) map[string]any {
	data := map[string]any{
		"out":      outPrefix,
		"unsigned": artifactPath,
		"manifest": artifact.SiblingManifestPath(artifactPath),
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}
