package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// Build information, set at link time by the release workflow:
//
//	go build -ldflags "-X github.com/ravisuhag/astro/cli.version=v1.2.3 ..."
//
// When they are unset — a `go install` or a local build — the values are
// recovered from the module's build info instead, so a binary can always say
// where it came from.
var (
	version = ""
	commit  = ""
	date    = ""
)

// BuildInfo describes the running binary.
type BuildInfo struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	Platform  string
}

// buildInfo assembles the build information, falling back to what the Go
// toolchain stamped into the binary.
func buildInfo() BuildInfo {
	info := BuildInfo{
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}

	read, ok := debug.ReadBuildInfo()
	if !ok {
		if info.Version == "" {
			info.Version = "unknown"
		}
		return info
	}

	if info.Version == "" {
		info.Version = read.Main.Version
		// A binary built straight from a checkout reports this rather than a
		// tag, which is honest but unhelpful on its own; the commit below
		// fills the gap.
		if info.Version == "" || info.Version == "(devel)" {
			info.Version = "devel"
		}
	}
	// Whether the working tree was dirty describes the checkout this binary
	// was built from. It only means anything when the commit came from that
	// checkout too — a commit passed in at link time refers to whatever the
	// release workflow tagged, so decorating it here would be a lie.
	stamped := info.Commit != ""
	dirty := false

	for _, setting := range read.Settings {
		switch setting.Key {
		case "vcs.revision":
			if info.Commit == "" {
				info.Commit = setting.Value
			}
		case "vcs.time":
			if info.Date == "" {
				info.Date = setting.Value
			}
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}

	if dirty && !stamped {
		info.Commit += "-dirty"
	}
	return info
}

// String renders the build information on one line.
//
// It carries no program name: cobra's --version template already prefixes
// one, and the version subcommand adds its own.
func (b BuildInfo) String() string {
	out := b.Version
	if b.Commit != "" {
		out += " (" + shortCommit(b.Commit) + ")"
	}
	return out + " " + b.Platform + " " + b.GoVersion
}

// shortCommit abbreviates a revision the way git does, keeping the -dirty
// marker if buildInfo appended one.
func shortCommit(c string) string {
	suffix := ""
	if rest, found := strings.CutSuffix(c, "-dirty"); found {
		c, suffix = rest, "-dirty"
	}
	if len(c) > 7 {
		c = c[:7]
	}
	return c + suffix
}

// versionCmd reports the build information.
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version and build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := buildInfo()

			var b strings.Builder
			fmt.Fprintf(&b, "astro %s\n", info.Version)
			if info.Commit != "" {
				fmt.Fprintf(&b, "  commit:   %s\n", info.Commit)
			}
			if info.Date != "" {
				fmt.Fprintf(&b, "  built:    %s\n", info.Date)
			}
			fmt.Fprintf(&b, "  go:       %s\n", info.GoVersion)
			fmt.Fprintf(&b, "  platform: %s\n", info.Platform)

			_, err := cmd.OutOrStdout().Write([]byte(b.String()))
			return err
		},
	}
}
