// Package version carries build information for the AI Skope Server.
package version

import "runtime/debug"

// Version is the semantic version, overridden at build time with
// -ldflags "-X github.com/ai-skope/aiss/internal/version.Version=0.1.0".
var Version = "0.1.0-dev"

// Commit is the git revision, filled from build info when not set by ldflags.
var Commit = ""

// APIVersion is the HTTP API contract version served under /v1.
const APIVersion = 1

func init() {
	if Commit != "" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			if len(s.Value) > 12 {
				Commit = s.Value[:12]
			} else {
				Commit = s.Value
			}
		}
	}
}

// String renders "0.3.0 (abc123)" or just the version when the commit is unknown.
func String() string {
	if Commit == "" {
		return Version
	}
	return Version + " (" + Commit + ")"
}
