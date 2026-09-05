package cli

import (
	"embed"
	"os"
	"path/filepath"
	"testing"
)

// nonProtocolCommands are the root subcommands that are not protocol
// commands and so have no entry in the manual's protocols map. "help" is
// included because cobra adds it once Execute runs, even though it is not
// present on a freshly built root command.
var nonProtocolCommands = map[string]bool{
	"help":       true,
	"completion": true,
	"manual":     true,
	"version":    true,
	"reference":  true,
}

// TestManualRegistryMatchesCommandTree asserts the three-way invariant this
// binary depends on at runtime: every protocol command registered on the
// root has exactly one entry in the manual's protocols map, and no map
// entry names a protocol that is not actually registered. Cobra and the
// manual map are otherwise free to drift apart silently — nothing but a
// user running `astro manual <cmd>` would notice, and only at runtime.
func TestManualRegistryMatchesCommandTree(t *testing.T) {
	root := New(embed.FS{})

	registered := make(map[string]bool)
	for _, cmd := range root.Commands() {
		name := cmd.Name()
		if nonProtocolCommands[name] {
			continue
		}
		registered[name] = true
	}

	for name := range registered {
		if _, ok := protocols[name]; !ok {
			t.Errorf("command %q is registered on the root but has no entry in the manual's protocols map", name)
		}
	}

	for name := range protocols {
		if !registered[name] {
			t.Errorf("manual protocols map has entry %q but no matching command is registered on the root", name)
		}
	}
}

// TestManualFilenamesResolve asserts every filename the protocols map points
// at actually exists under docs/content/cli, which is what main.go embeds
// into the shipped binary. A mapped name that does not resolve here would
// build, vet and test clean, then fail at runtime the first time someone
// runs `astro manual <that command>`.
func TestManualFilenamesResolve(t *testing.T) {
	const docsDir = "../docs/content/cli"

	if _, err := os.Stat(docsDir); err != nil {
		t.Fatalf("expected %s to exist relative to the cli package: %v", docsDir, err)
	}

	for protocol, filename := range protocols {
		path := filepath.Join(docsDir, filename)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("manual protocol %q maps to %q, which does not resolve: %v", protocol, path, err)
		}
	}
}
