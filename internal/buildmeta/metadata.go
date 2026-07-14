// Package buildmeta exposes build-time dependency metadata without import cycles.
package buildmeta

import "runtime/debug"

// ApolloRevision returns the resolved Apollo v2 module version from build info.
func ApolloRevision() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == "github.com/Salvionied/apollo/v2" {
				if d.Replace != nil && d.Replace.Version != "" {
					return d.Replace.Version
				}
				return d.Version
			}
		}
	}
	return "unknown"
}
