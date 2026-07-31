package xtce

import (
	"fmt"
	"strings"
)

// Humanize renders the database as an indented tree.
//
// The shape is one block per SpaceSystem: a count line, then the parameters
// with their types and encoded widths, then the containers with their entries
// in packet order. It is meant to be read next to the XML when working out why
// a database does not say what someone thought it said.
func (s *SpaceSystem) Humanize() string {
	var b strings.Builder
	s.humanize(&b, 0)
	return strings.TrimRight(b.String(), "\n")
}

// humanize writes one SpaceSystem and its children.
func (s *SpaceSystem) humanize(b *strings.Builder, depth int) {
	pad := strings.Repeat("  ", depth)

	fmt.Fprintf(b, "%sSpaceSystem %s\n", pad, s.QualifiedName())
	if s.ShortDescription != "" {
		fmt.Fprintf(b, "%s  %s\n", pad, s.ShortDescription)
	}

	params := s.Parameters()
	types := s.ParameterTypes()
	containers := s.Containers()
	commands := s.MetaCommands()

	fmt.Fprintf(b, "%s  %d parameters, %d types, %d containers, %d commands\n",
		pad, len(params), len(types), len(containers), len(commands))

	if len(params) > 0 {
		fmt.Fprintf(b, "%s  Parameters\n", pad)
		for _, param := range params {
			fmt.Fprintf(b, "%s    %s\n", pad, s.describeParameter(param))
		}
	}

	if len(containers) > 0 {
		fmt.Fprintf(b, "%s  Containers\n", pad)
		for _, container := range containers {
			fmt.Fprintf(b, "%s    %s\n", pad, describeContainer(container))
			for _, entry := range container.EntryList.Entries {
				fmt.Fprintf(b, "%s      %s\n", pad, entry)
			}
		}
	}

	if len(commands) > 0 {
		fmt.Fprintf(b, "%s  Commands\n", pad)
		for _, command := range commands {
			arguments := 0
			if command.ArgumentList != nil {
				arguments = len(command.ArgumentList.Arguments)
			}
			fmt.Fprintf(b, "%s    %s (%d arguments)\n", pad, command.Name, arguments)
		}
	}

	for _, child := range s.SubSystems {
		child.humanize(b, depth+1)
	}
}

// describeParameter renders one parameter: its name, its type, and how wide it
// is on the wire.
//
// The width comes from the encoding rather than the type, because those differ
// and the encoding is the one that matters for reading a packet: a parameter
// may be a 32-bit float in software and twelve bits in the downlink.
func (s *SpaceSystem) describeParameter(param *Parameter) string {
	line := param.Name

	resolved, err := s.ResolveParameterType(param.ParameterTypeRef)
	if err != nil {
		return fmt.Sprintf("%s -> %s (unresolved)", line, param.ParameterTypeRef)
	}

	line += " -> " + resolved.TypeName() + " (" + resolved.TypeKind()
	if bits, ok := resolved.Encoding().SizeInBits(); ok {
		line += fmt.Sprintf(", %d bits", bits)
	} else {
		line += ", size not fixed"
	}
	line += ")"

	if calibrator := calibratorOf(resolved); calibrator != nil {
		line += ", " + calibrator.Kind() + " calibrator"
	}
	return line
}

// calibratorOf returns the type's default calibrator, if it has one. Only the
// numeric encodings carry them.
func calibratorOf(t ParameterType) *Calibrator {
	encoding := t.Encoding()
	if encoding == nil {
		return nil
	}
	switch {
	case encoding.Integer != nil:
		return encoding.Integer.DefaultCalibrator
	case encoding.Float != nil:
		return encoding.Float.DefaultCalibrator
	default:
		return nil
	}
}

// describeContainer renders a container's header line.
func describeContainer(container *SequenceContainer) string {
	line := container.Name
	if container.Abstract {
		line += " (abstract)"
	}
	if container.BaseContainer != nil {
		line += " extends " + container.BaseContainer.ContainerRef
	}
	return fmt.Sprintf("%s, %d entries", line, len(container.EntryList.Entries))
}
