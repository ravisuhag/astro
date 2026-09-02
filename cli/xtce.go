package cli

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ravisuhag/astro/pkg/xtce"
	"github.com/spf13/cobra"
)

func xtceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xtce <command>",
		Short: "XTCE mission database operations",
		Long:  "Validate, inspect, and decode packets against an XTCE 1.2 mission database (OMG XTCE / CCSDS 660.1-G-2).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		xtceValidateCmd(),
		xtceListCmd(),
		xtceLayoutCmd(),
		xtceDecodeCmd(),
		xtceMatchCmd(),
	)
	return cmd
}

func xtceValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Check an XTCE database for the errors the schema cannot catch",
		Long: "Load an XTCE database and run the semantic checks: unresolved references, duplicate names, container inheritance cycles, and enumeration members outside the schema's sets.\n\n" +
			"This is not schema validation. The Go standard library has no XSD validator and this package takes no dependencies, so a file that breaks the schema some other way will load and pass here. For real conformance run xmllint --schema SpaceSystem.xsd over the file first.",
		Example: `  # Validate a database
  astro xtce validate mission.xml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := xtce.LoadFile(args[0])
			if err != nil {
				return fmt.Errorf("loading %s: %w", args[0], err)
			}
			if err := db.Validate(); err != nil {
				return fmt.Errorf("validating %s: %w", args[0], err)
			}

			systems, parameters, containers := countDatabase(db)
			fmt.Printf("%s is valid.\n", args[0])
			fmt.Printf("  %d space system(s), %d parameter(s), %d container(s)\n",
				systems, parameters, containers)
			return nil
		},
	}
	return cmd
}

func xtceListCmd() *cobra.Command {
	var (
		outputFmt string
		kind      string
	)

	cmd := &cobra.Command{
		Use:   "list <file>",
		Short: "List the space systems, parameters and containers a database defines",
		Long:  "Walk an XTCE database and list what it defines, by qualified name.",
		Example: `  # Everything
  astro xtce list mission.xml

  # Just the containers, which is what you need for decode and match
  astro xtce list mission.xml --kind containers

  # As JSON
  astro xtce list mission.xml --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch kind {
			case "all", "systems", "parameters", "containers":
			default:
				return fmt.Errorf("unknown kind: %s (use 'all', 'systems', 'parameters', or 'containers')", kind)
			}

			db, err := xtce.LoadFile(args[0])
			if err != nil {
				return fmt.Errorf("loading %s: %w", args[0], err)
			}

			listing := databaseListing(db)

			switch outputFmt {
			case "json":
				filtered := map[string][]string{}
				if kind == "all" || kind == "systems" {
					filtered["systems"] = listing.Systems
				}
				if kind == "all" || kind == "parameters" {
					filtered["parameters"] = listing.Parameters
				}
				if kind == "all" || kind == "containers" {
					filtered["containers"] = listing.Containers
				}
				b, err := json.MarshalIndent(filtered, "", "  ")
				if err != nil {
					return fmt.Errorf("encoding JSON output: %w", err)
				}
				fmt.Println(string(b))
			case "text":
				if kind == "all" || kind == "systems" {
					printNames("Space systems", listing.Systems)
				}
				if kind == "all" || kind == "parameters" {
					printNames("Parameters", listing.Parameters)
				}
				if kind == "all" || kind == "containers" {
					printNames("Containers", listing.Containers)
				}
			default:
				return fmt.Errorf("unknown format: %s (use 'text' or 'json')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")
	cmd.Flags().StringVar(&kind, "kind", "all", "What to list: all, systems, parameters, or containers")
	return cmd
}

func xtceLayoutCmd() *cobra.Command {
	var outputFmt string

	cmd := &cobra.Command{
		Use:   "layout <file> <container>",
		Short: "Show the fields a container lays out, with bit offsets",
		Long: "Flatten a container into the fields a packet of that shape carries, in packet order, with each field's bit offset and width.\n\n" +
			"This is what decode reads a packet against, so it is the thing to look at when a decode does not come out the way the database led you to expect.",
		Example: `  # The field map of one container
  astro xtce layout mission.xml /Sat/Telemetry

  # As JSON
  astro xtce layout mission.xml /Sat/Telemetry --format json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := xtce.LoadFile(args[0])
			if err != nil {
				return fmt.Errorf("loading %s: %w", args[0], err)
			}

			layout, _, err := layoutFor(db, args[1], nil)
			if err != nil {
				return fmt.Errorf("laying out %s: %w", args[1], err)
			}

			switch outputFmt {
			case "json":
				b, err := json.MarshalIndent(layoutToJSON(layout), "", "  ")
				if err != nil {
					return fmt.Errorf("encoding JSON output: %w", err)
				}
				fmt.Println(string(b))
			case "text":
				printLayout(layout)
			default:
				return fmt.Errorf("unknown format: %s (use 'text' or 'json')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")
	return cmd
}

func xtceDecodeCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		container string
		raw       bool
	)

	cmd := &cobra.Command{
		Use:   "decode <file> [packet-file]",
		Short: "Decode a packet against a container in the database",
		Long: "Read a packet against a named container and print each field's value.\n\n" +
			"Values are the engineering ones by default: calibrated numbers and enumeration labels. Use --raw for the counts as the packet carried them.\n\n" +
			"A container whose shape depends on its own contents — a delimited string, a blob sized by a length field, a packet-decided repeat count — is resolved against the packet being decoded, and the text output says so. The resulting field map describes that packet only.",
		Example: `  # Decode a hex packet from stdin
  astro spp encode --apid 100 --type tm --data 0102 | astro xtce decode mission.xml --container /Sat/Telemetry

  # From a binary capture, as JSON
  astro xtce decode mission.xml packet.bin --input bin --container /Sat/Telemetry --format json`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if container == "" {
				return fmt.Errorf("--container is required; run 'astro xtce list --kind containers' to see the names")
			}

			db, err := xtce.LoadFile(args[0])
			if err != nil {
				return fmt.Errorf("loading %s: %w", args[0], err)
			}

			data, err := readInput(args[1:], inputFmt)
			if err != nil {
				return err
			}

			layout, resolved, err := layoutFor(db, container, data)
			if err != nil {
				return fmt.Errorf("laying out %s: %w", container, err)
			}
			if resolved && outputFmt == "text" {
				fmt.Println("Layout resolved against this packet: the container's shape depends on its contents.")
			}

			packet, err := layout.Extract(data)
			if err != nil {
				return fmt.Errorf("extracting the packet: %w", err)
			}

			return printXTCEPacket(packet, raw, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")
	cmd.Flags().StringVar(&container, "container", "", "Qualified container name to decode against (required)")
	cmd.Flags().BoolVar(&raw, "raw", false, "Show the raw values rather than the calibrated ones")
	return cmd
}

func xtceMatchCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		root      string
		raw       bool
	)

	cmd := &cobra.Command{
		Use:   "match <file> [packet-file]",
		Short: "Work out which container a packet is, then decode it",
		Long: "Search down from a root container for the one whose restriction criteria the packet satisfies, and decode it against that.\n\n" +
			"This is what a ground station does with an unlabelled packet. The deepest match wins, so a packet that is both a telemetry packet and a housekeeping telemetry packet is reported as the latter.",
		Example: `  # Identify and decode a packet
  astro xtce match mission.xml --root /Sat/Packet < packet.hex

  # Just the container name
  astro xtce match mission.xml --root /Sat/Packet --format name < packet.hex`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if root == "" {
				return fmt.Errorf("--root is required; run 'astro xtce list --kind containers' to see the names")
			}

			db, err := xtce.LoadFile(args[0])
			if err != nil {
				return fmt.Errorf("loading %s: %w", args[0], err)
			}

			rootContainer, err := db.FindContainer(root)
			if err != nil {
				return fmt.Errorf("finding %s: %w", root, err)
			}

			data, err := readInput(args[1:], inputFmt)
			if err != nil {
				return err
			}

			if outputFmt == "name" {
				match, err := db.MatchFrom(rootContainer, data)
				if err != nil {
					return fmt.Errorf("matching the packet: %w", err)
				}
				fmt.Println(match.Name)
				return nil
			}

			packet, err := db.Match(rootContainer, data)
			if err != nil {
				return fmt.Errorf("matching the packet: %w", err)
			}

			if outputFmt == "text" {
				fmt.Printf("Matched container: %s\n", packet.Layout.Container.Name)
			}
			return printXTCEPacket(packet, raw, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or name")
	cmd.Flags().StringVar(&root, "root", "", "Qualified name of the container to search down from (required)")
	cmd.Flags().BoolVar(&raw, "raw", false, "Show the raw values rather than the calibrated ones")
	return cmd
}

// layoutFor returns a container's layout, resolving it against the packet
// when the database alone cannot settle it.
//
// Most containers fix every field's width, and for those the static layout is
// both cheaper and independent of any packet. Some do not — a delimited
// string, a length-sized blob, a packet-decided repeat count — and those need
// the packet. A caller should not have to know which kind they have, so this
// tries the cheap path and falls back.
//
// packet may be nil, for a caller that only wants the static shape; the
// fallback then reports why the layout could not be settled.
func layoutFor(db *xtce.SpaceSystem, name string, packet []byte) (*xtce.Layout, bool, error) {
	container, err := db.FindContainer(name)
	if err != nil {
		return nil, false, err
	}

	layout, err := container.Layout()
	if err == nil {
		return layout, false, nil
	}
	if packet == nil {
		return nil, false, err
	}
	if !errors.Is(err, xtce.ErrDynamicSize) && !errors.Is(err, xtce.ErrUnsupportedEntry) {
		return nil, false, err
	}

	// The container's shape depends on the packet, so read it against this
	// one. The layout that comes back describes this packet only.
	resolved, resolveErr := container.ResolveLayout(packet)
	if resolveErr != nil {
		return nil, false, resolveErr
	}
	return resolved, true, nil
}

// databaseNames is what list reports.
type databaseNames struct {
	Systems    []string `json:"systems"`
	Parameters []string `json:"parameters"`
	Containers []string `json:"containers"`
}

// databaseListing walks the tree and collects qualified names, sorted so the
// output is stable between runs.
func databaseListing(db *xtce.SpaceSystem) databaseNames {
	var names databaseNames

	// Qualified names throughout, because that is what the other
	// subcommands take: the output of list is meant to be usable as the
	// argument to layout, decode and match.
	db.Root().Walk(func(system *xtce.SpaceSystem) bool {
		path := system.QualifiedName()
		names.Systems = append(names.Systems, path)

		if system.TelemetryMetaData != nil {
			if set := system.TelemetryMetaData.ParameterSet; set != nil {
				for _, parameter := range set.Parameters {
					names.Parameters = append(names.Parameters, path+"/"+parameter.Name)
				}
			}
			if set := system.TelemetryMetaData.ContainerSet; set != nil {
				for _, container := range set.SequenceContainers {
					name := path + "/" + container.Name
					if container.Abstract {
						name += " (abstract)"
					}
					names.Containers = append(names.Containers, name)
				}
			}
		}
		return true
	})

	sort.Strings(names.Systems)
	sort.Strings(names.Parameters)
	sort.Strings(names.Containers)
	return names
}

// countDatabase is the summary validate prints.
func countDatabase(db *xtce.SpaceSystem) (systems, parameters, containers int) {
	names := databaseListing(db)
	return len(names.Systems), len(names.Parameters), len(names.Containers)
}

func printNames(heading string, names []string) {
	fmt.Printf("%s (%d):\n", heading, len(names))
	for _, name := range names {
		fmt.Printf("  %s\n", name)
	}
	fmt.Println()
}

// fieldJSON is one row of a layout.
type fieldJSON struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	BitOffset uint   `json:"bit_offset"`
	BitSize   uint   `json:"bit_size"`
	Container string `json:"container"`
}

type layoutJSON struct {
	Container string      `json:"container"`
	BitSize   uint        `json:"bit_size"`
	MinOctets int         `json:"min_octets"`
	Fields    []fieldJSON `json:"fields"`
}

func layoutToJSON(layout *xtce.Layout) layoutJSON {
	out := layoutJSON{
		Container: layout.Container.Name,
		BitSize:   layout.BitSize,
		MinOctets: int((layout.BitSize + 7) / 8),
	}
	for _, field := range layout.Fields {
		out.Fields = append(out.Fields, fieldJSON{
			Name:      field.Name,
			Type:      fieldTypeName(field),
			BitOffset: field.BitOffset,
			BitSize:   field.BitSize,
			Container: field.Container.Name,
		})
	}
	return out
}

// fieldTypeName names a field's parameter type, tolerating a field whose type
// did not resolve.
func fieldTypeName(field xtce.Field) string {
	if field.Type == nil {
		return "unresolved"
	}
	return field.Type.TypeKind()
}

func printLayout(layout *xtce.Layout) {
	fmt.Printf("Container: %s\n", layout.Container.Name)
	fmt.Printf("Size: %d bits (%d octets minimum)\n",
		layout.BitSize, (layout.BitSize+7)/8)
	fmt.Println(strings.Repeat("─", 72))
	fmt.Printf("%-8s %-8s %-12s %s\n", "OFFSET", "WIDTH", "TYPE", "NAME")
	fmt.Println(strings.Repeat("─", 72))

	for _, field := range layout.Fields {
		fmt.Printf("%-8d %-8d %-12s %s\n",
			field.BitOffset, field.BitSize, fieldTypeName(field), field.Name)
	}
}

// valueJSON is one decoded field.
type valueJSON struct {
	Name      string `json:"name"`
	BitOffset uint   `json:"bit_offset"`
	BitSize   uint   `json:"bit_size"`
	Value     any    `json:"value,omitempty"`
	Error     string `json:"error,omitempty"`
}

func printXTCEPacket(packet *xtce.Packet, raw bool, outputFmt string) error {
	switch outputFmt {
	case "json":
		rows := make([]valueJSON, 0, len(packet.Values))
		for _, value := range packet.Values {
			row := valueJSON{
				Name:      value.Field.Name,
				BitOffset: value.Field.BitOffset,
				BitSize:   value.Field.BitSize,
			}
			if value.Err != nil {
				row.Error = value.Err.Error()
			} else {
				row.Value = jsonSafeValue(pick(value, raw))
			}
			rows = append(rows, row)
		}
		b, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding JSON output: %w", err)
		}
		fmt.Println(string(b))

	case "text":
		fmt.Println(strishRepeat(72))
		fmt.Printf("%-8s %-8s %-28s %s\n", "OFFSET", "WIDTH", "NAME", "VALUE")
		fmt.Println(strishRepeat(72))

		failed := 0
		for _, value := range packet.Values {
			if value.Err != nil {
				failed++
				fmt.Fprintf(os.Stderr, "  %s: %v\n", value.Field.Name, value.Err)
				continue
			}
			fmt.Printf("%-8d %-8d %-28s %v\n",
				value.Field.BitOffset, value.Field.BitSize,
				value.Field.Name, displayValue(pick(value, raw)))
		}
		if failed > 0 {
			fmt.Printf("\n%d field(s) could not be decoded; see stderr.\n", failed)
		}

	default:
		return fmt.Errorf("unknown format: %s (use 'text' or 'json')", outputFmt)
	}
	return nil
}

// pick chooses the raw or the engineering value.
func pick(value xtce.Value, raw bool) any {
	if raw {
		return value.Raw
	}
	return value.Engineering
}

// displayValue renders a decoded value for the text output.
//
// A binary field is a run of octets, and Go prints a []byte as a list of
// decimal numbers, which nobody reads bytes as. Hex is what the rest of these
// commands use.
func displayValue(value any) any {
	if data, ok := value.([]byte); ok {
		return hex.EncodeToString(data)
	}
	return value
}

// jsonSafeValue turns a decoded value into something json.Marshal renders
// usefully. A []byte would otherwise come out base64, which is not how a
// binary field is read.
func jsonSafeValue(value any) any {
	if data, ok := value.([]byte); ok {
		return hex.EncodeToString(data)
	}
	return value
}

// strishRepeat is the rule the other inspectors draw.
func strishRepeat(width int) string {
	return strings.Repeat("─", width)
}
