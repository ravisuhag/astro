package cli

import (
	"embed"
	"fmt"
	"strings"

	"github.com/raystack/salt/cli/printer"
	"github.com/spf13/cobra"
)

// protocols maps CLI protocol names to their doc filenames.
var protocols = map[string]string{
	"spp":  "spp.md",
	"epp":  "epp.md",
	"tm":   "tm.md",
	"tc":   "tc.md",
	"time": "time.md",
	"cadu": "cadu.md",
	"cltu": "cltu.md",
}

func manualCmd(docsFS embed.FS) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manual [protocol]",
		Short: "Display protocol reference manual",
		Long:  "Display the CLI reference manual for a protocol. Run without arguments to list available topics.",
		Annotations: map[string]string{
			"group": "help",
		},
		ValidArgs: protocolNames(),
		Args:      cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return printManualIndex()
			}
			return printManual(docsFS, args[0])
		},
	}

	return cmd
}

func printManualIndex() error {
	var sb strings.Builder
	sb.WriteString("# Astro Manual\n\n")
	sb.WriteString("Available protocol manuals:\n\n")
	sb.WriteString("| Protocol | Command |\n")
	sb.WriteString("|----------|---------|\n")
	sb.WriteString("| Space Packet Protocol | `astro manual spp` |\n")
	sb.WriteString("| Encapsulation Packet Protocol | `astro manual epp` |\n")
	sb.WriteString("| TM Transfer Frames | `astro manual tm` |\n")
	sb.WriteString("| TC Transfer Frames | `astro manual tc` |\n")
	sb.WriteString("| Time Code Formats | `astro manual time` |\n")
	sb.WriteString("| Channel Access Data Units | `astro manual cadu` |\n")
	sb.WriteString("| Command Link Transmission Units | `astro manual cltu` |\n")

	out, err := printer.Markdown(sb.String())
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func printManual(docsFS embed.FS, protocol string) error {
	filename, ok := protocols[protocol]
	if !ok {
		return fmt.Errorf("unknown protocol %q — run 'astro manual' to see available topics", protocol)
	}

	content, err := docsFS.ReadFile("docs/cli/" + filename)
	if err != nil {
		return fmt.Errorf("reading manual for %s: %w", protocol, err)
	}

	out, err := printer.Markdown(string(content))
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func protocolNames() []string {
	names := make([]string, 0, len(protocols))
	for k := range protocols {
		names = append(names, k)
	}
	return names
}
