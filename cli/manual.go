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
	"usdl": "usdl.md",
	"aos":  "aos.md",
	"xtce": "xtce.md",
	"pus":  "pus.md",
	"ldc":  "ldc.md",
	"rhc":  "rhc.md",
	"cfdp": "cfdp.md",
	"ltp":  "ltp.md",
	"bp":   "bp.md",
	"sle":  "sle.md",
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
	sb.WriteString("| Unified Space Data Link Protocol | `astro manual usdl` |\n")
	sb.WriteString("| AOS Space Data Link Protocol | `astro manual aos` |\n")
	sb.WriteString("| XTCE Mission Databases | `astro manual xtce` |\n")
	sb.WriteString("| PUS Packet Utilisation Services | `astro manual pus` |\n")
	sb.WriteString("| Lossless Data Compression | `astro manual ldc` |\n")
	sb.WriteString("| Robust Housekeeping Compression | `astro manual rhc` |\n")
	sb.WriteString("| CCSDS File Delivery Protocol | `astro manual cfdp` |\n")
	sb.WriteString("| Licklider Transmission Protocol | `astro manual ltp` |\n")
	sb.WriteString("| Bundle Protocol | `astro manual bp` |\n")
	sb.WriteString("| Space Link Extension | `astro manual sle` |\n")

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

	content, err := docsFS.ReadFile("docs/content/cli/" + filename)
	if err != nil {
		return fmt.Errorf("reading manual for %s: %w", protocol, err)
	}

	out, err := printer.Markdown(stripFrontmatter(string(content)))
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

// stripFrontmatter removes the leading YAML frontmatter block that the docs
// site reads for page titles and sidebar order. The terminal has no use for
// it, and rendering it would print the raw YAML above the manual page.
func stripFrontmatter(s string) string {
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	if _, rest, ok := strings.Cut(s[4:], "\n---\n"); ok {
		return strings.TrimLeft(rest, "\n")
	}
	return s
}

func protocolNames() []string {
	names := make([]string, 0, len(protocols))
	for k := range protocols {
		names = append(names, k)
	}
	return names
}
