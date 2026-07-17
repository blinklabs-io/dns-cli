package cli

import "github.com/blinklabs-io/dns-cli/internal/artifact"

// unsignedBuildData fills JSON result fields for path-prefix builders
// (--out writes <out>.unsigned.json + <out>.manifest.json).
func unsignedBuildData(outPrefix string, extra map[string]any) map[string]any {
	unsigned := outPrefix + ".unsigned.json"
	data := map[string]any{
		"out":      outPrefix,
		"unsigned": unsigned,
		"manifest": artifact.SiblingManifestPath(unsigned),
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

// firstNonEmptyFlag returns the first non-empty string (for flag aliases).
func firstNonEmptyFlag(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
