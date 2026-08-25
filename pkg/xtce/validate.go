package xtce

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Semantic validation.
//
// There is no XSD validator in the Go standard library and this package takes
// no dependencies, so these checks are written by hand. They are not a
// substitute for schema validation and do not try to be: they cover the five
// mistakes that make a database unusable rather than merely non-conforming.
//
//	a reference that names nothing        the parameter cannot be decoded
//	a container inheriting in a circle    walking the chain never ends
//	two things sharing a name             a reference to it is ambiguous
//	a malformed reference                 nothing can be done with it
//	an illegal encoding enumeration       every packet would fail to decode
//
// A file that breaks the XSD some other way will load and pass. If you need
// schema conformance, run xmllint against the OMG schema before loading.

// ValidationError is one problem found in a database, with enough context to
// find it in the file.
type ValidationError struct {
	// SpaceSystem is the qualified name of the system holding the problem.
	SpaceSystem string
	// Element names what was wrong, such as "Parameter" or "ContainerRefEntry".
	Element string
	// Detail is the specific complaint.
	Detail string
	// Err is the sentinel, for errors.Is.
	Err error
}

// Error renders the problem.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.SpaceSystem, e.Element, e.Detail)
}

// Unwrap exposes the sentinel.
func (e *ValidationError) Unwrap() error { return e.Err }

// ValidationErrors is every problem found in one pass.
//
// Validation collects rather than stopping at the first problem, because
// someone fixing a database wants the whole list, not one line at a time.
type ValidationErrors []*ValidationError

// Error summarises the problems.
func (e ValidationErrors) Error() string {
	if len(e) == 1 {
		return e[0].Error()
	}
	out := fmt.Sprintf("%d problems in the database:", len(e))
	for _, problem := range e {
		out += "\n  " + problem.Error()
	}
	return out
}

// Is reports whether any problem matches the target, so errors.Is finds a
// sentinel anywhere in the list.
func (e ValidationErrors) Is(target error) bool {
	for _, problem := range e {
		if errors.Is(problem.Err, target) {
			return true
		}
	}
	return false
}

// Validate checks that a database hangs together.
//
// It returns ValidationErrors, so errors.Is finds any sentinel among them and
// a type assertion gets the whole list.
func (s *SpaceSystem) Validate() error {
	var problems ValidationErrors

	s.Walk(func(system *SpaceSystem) bool {
		problems = append(problems, system.checkDuplicateNames()...)
		problems = append(problems, system.checkParameterTypeRefs()...)
		problems = append(problems, system.checkEntryRefs()...)
		problems = append(problems, system.checkEncodings()...)
		return true
	})

	// Inheritance is checked over the whole tree at once rather than per
	// system. Checking it per container meant re-walking the tree to find each
	// base's home, which made validation cubic in the number of containers: an
	// 80 KB file of chained containers took 200 ms, and the size cap allows
	// files hundreds of times larger. Building the graph once and colouring it
	// makes the same check linear.
	problems = append(problems, s.checkInheritance()...)

	if len(problems) == 0 {
		return nil
	}
	return problems
}

// problem builds a ValidationError for this SpaceSystem.
func (s *SpaceSystem) problem(element, detail string, err error) *ValidationError {
	return &ValidationError{
		SpaceSystem: s.QualifiedName(),
		Element:     element,
		Detail:      detail,
		Err:         err,
	}
}

// checkDuplicateNames enforces the schema's uniqueness keys within one
// SpaceSystem.
//
// The keys are per SpaceSystem, not per document: two systems may each have a
// Voltage, and that is the point of the tree. What they may not have is two
// Voltages in the same system — and because parameterNameKey selects from both
// TelemetryMetaData and CommandMetaData, the two sides share one namespace.
func (s *SpaceSystem) checkDuplicateNames() []*ValidationError {
	var problems []*ValidationError

	problems = append(problems, checkUnique(s, "Parameter", parameterNames(s.Parameters()))...)
	problems = append(problems, checkUnique(s, "ParameterType", typeNames(s.ParameterTypes()))...)
	problems = append(problems, checkUnique(s, "SequenceContainer", containerNames(s.Containers()))...)
	problems = append(problems, checkUnique(s, "MetaCommand", commandNames(s.MetaCommands()))...)

	// Sibling SpaceSystems must be distinguishable too, or a path reference
	// through them is ambiguous.
	subNames := make([]string, 0, len(s.SubSystems))
	for _, child := range s.SubSystems {
		subNames = append(subNames, child.Name)
	}
	problems = append(problems, checkUnique(s, "SpaceSystem", subNames)...)

	return problems
}

// checkUnique reports each name that appears more than once.
func checkUnique(s *SpaceSystem, element string, names []string) []*ValidationError {
	seen := make(map[string]bool, len(names))
	var problems []*ValidationError

	for _, name := range names {
		if name == "" {
			problems = append(problems, s.problem(element, "has no name", ErrDuplicateName))
			continue
		}
		if seen[name] {
			problems = append(problems, s.problem(element,
				fmt.Sprintf("%q is defined more than once", name), ErrDuplicateName))
			continue
		}
		seen[name] = true
	}
	return problems
}

func parameterNames(params []*Parameter) []string {
	names := make([]string, 0, len(params))
	for _, p := range params {
		names = append(names, p.Name)
	}
	return names
}

func typeNames(types []ParameterType) []string {
	names := make([]string, 0, len(types))
	for _, t := range types {
		names = append(names, t.TypeName())
	}
	return names
}

func containerNames(containers []*SequenceContainer) []string {
	names := make([]string, 0, len(containers))
	for _, c := range containers {
		names = append(names, c.Name)
	}
	return names
}

func commandNames(commands []*MetaCommand) []string {
	names := make([]string, 0, len(commands))
	for _, c := range commands {
		names = append(names, c.Name)
	}
	return names
}

// checkParameterTypeRefs makes sure every parameter has a type.
func (s *SpaceSystem) checkParameterTypeRefs() []*ValidationError {
	var problems []*ValidationError

	for _, param := range s.Parameters() {
		if param.ParameterTypeRef == "" {
			problems = append(problems, s.problem("Parameter",
				fmt.Sprintf("%q has no parameterTypeRef", param.Name), ErrUnresolvedReference))
			continue
		}
		if _, err := s.ResolveParameterType(param.ParameterTypeRef); err != nil {
			problems = append(problems, s.problem("Parameter",
				fmt.Sprintf("%q references type %q, which does not resolve",
					param.Name, param.ParameterTypeRef), sentinelOf(err)))
		}
	}
	return problems
}

// checkEntryRefs makes sure every container entry names something real.
func (s *SpaceSystem) checkEntryRefs() []*ValidationError {
	var problems []*ValidationError

	for _, container := range s.Containers() {
		for i, entry := range container.EntryList.Entries {
			switch entry.Kind {
			case EntryParameterRef:
				if _, err := s.ResolveParameter(entry.Ref); err != nil {
					problems = append(problems, s.problem("ParameterRefEntry",
						fmt.Sprintf("container %q entry %d references parameter %q, which does not resolve",
							container.Name, i, entry.Ref), sentinelOf(err)))
				}
			case EntryContainerRef:
				if _, err := s.ResolveContainer(entry.Ref); err != nil {
					problems = append(problems, s.problem("ContainerRefEntry",
						fmt.Sprintf("container %q entry %d references container %q, which does not resolve",
							container.Name, i, entry.Ref), sentinelOf(err)))
				}
			case EntryOther:
				// An entry kind this package does not model. Its reference is
				// not checked, because what it points at depends on semantics
				// that are not modeled either.
			}
		}
	}
	return problems
}

// The legal members of the schema's encoding enumerations, from the XTCE 1.2
// XSD: IntegerEncodingType, FloatEncodingType, StringEncodingType and
// BitOrderType. An empty attribute is always legal — it means the default.
var (
	integerEncodings = map[string]bool{
		"unsigned": true, "signMagnitude": true, "twosComplement": true,
		"onesComplement": true, "BCD": true, "packedBCD": true,
	}
	floatEncodings = map[string]bool{
		"IEEE754_1985": true, "IEEE754": true, "MILSTD_1750A": true,
		"DEC": true, "IBM": true, "TI": true,
	}
	stringEncodings = map[string]bool{
		"US-ASCII": true, "ISO-8859-1": true, "Windows-1252": true,
		"UTF-8": true, "UTF-16": true, "UTF-16LE": true, "UTF-16BE": true,
		"UTF-32": true, "UTF-32LE": true, "UTF-32BE": true,
	}
	bitOrders = map[string]bool{
		"mostSignificantBitFirst": true, "leastSignificantBitFirst": true,
	}
)

// checkEncodings makes sure every data encoding's enumerated attributes —
// encoding, bitOrder, byteOrder — hold legal values.
//
// A misspelled member is otherwise invisible until decode time, where it
// surfaces as ErrUnsupportedEncoding on every packet — indistinguishable from
// an encoding this package genuinely does not support. Catching it here names
// the type and the attribute instead.
func (s *SpaceSystem) checkEncodings() []*ValidationError {
	var problems []*ValidationError

	for _, t := range s.ParameterTypes() {
		encoding := t.Encoding()
		if encoding == nil {
			continue
		}

		var element, kind string
		var common commonEncoding
		var member bool
		switch {
		case encoding.Integer != nil:
			element, common = "IntegerDataEncoding", encoding.Integer.commonEncoding
			kind, member = encoding.Integer.Encoding, integerEncodings[encoding.Integer.EncodingOrDefault()]
		case encoding.Float != nil:
			element, common = "FloatDataEncoding", encoding.Float.commonEncoding
			kind, member = encoding.Float.Encoding, floatEncodings[encoding.Float.EncodingOrDefault()]
		case encoding.String != nil:
			element, common = "StringDataEncoding", encoding.String.commonEncoding
			kind, member = encoding.String.Encoding, stringEncodings[encoding.String.EncodingOrDefault()]
		case encoding.Binary != nil:
			// BinaryDataEncoding has no encoding attribute of its own.
			element, common, member = "BinaryDataEncoding", encoding.Binary.commonEncoding, true
		}

		if !member {
			problems = append(problems, s.problem(element,
				fmt.Sprintf("type %q has encoding %q, which is not a member of the schema's enumeration",
					t.TypeName(), kind), ErrInvalidEncoding))
		}
		if common.BitOrder != "" && !bitOrders[common.BitOrder] {
			problems = append(problems, s.problem(element,
				fmt.Sprintf("type %q has bitOrder %q; the schema allows mostSignificantBitFirst and leastSignificantBitFirst",
					t.TypeName(), common.BitOrder), ErrInvalidEncoding))
		}
		if common.ByteOrder != "" && !legalByteOrder(common.ByteOrder) {
			problems = append(problems, s.problem(element,
				fmt.Sprintf("type %q has byteOrder %q; the schema allows mostSignificantByteFirst, leastSignificantByteFirst, or a comma-separated byte list",
					t.TypeName(), common.ByteOrder), ErrInvalidEncoding))
		}
	}
	return problems
}

// legalByteOrder reports whether a byteOrder attribute is legal. The schema's
// ByteOrderType is a union: the two common orders, or the arbitrary form — a
// comma-separated list of byte positions 0 through 15.
func legalByteOrder(order string) bool {
	if order == "mostSignificantByteFirst" || order == "leastSignificantByteFirst" {
		return true
	}
	for _, position := range strings.Split(order, ",") {
		n, err := strconv.Atoi(position)
		if err != nil || n < 0 || n > 15 {
			return false
		}
	}
	return true
}

// checkInheritance resolves every BaseContainer once and looks for cycles.
//
// The work is done as a graph rather than a chain walk per container. Each
// container is a node, each BaseContainer an edge, and a depth-first colouring
// finds every cycle in one pass. A container proved acyclic stays proved, so
// twenty containers sharing a base cost one walk of that base, not twenty.
func (s *SpaceSystem) checkInheritance() []*ValidationError {
	root := s.Root()

	// holders maps each container to the SpaceSystem it lives in, because a
	// BaseContainer reference resolves relative to that system.
	holders := make(map[*SequenceContainer]*SpaceSystem)
	var containers []*SequenceContainer
	root.Walk(func(system *SpaceSystem) bool {
		for _, container := range system.Containers() {
			holders[container] = system
			containers = append(containers, container)
		}
		return true
	})

	var problems []*ValidationError

	// base is the edge map: which container each one extends.
	base := make(map[*SequenceContainer]*SequenceContainer, len(containers))
	for _, container := range containers {
		if container.BaseContainer == nil {
			continue
		}
		holder := holders[container]
		ref := container.BaseContainer.ContainerRef
		parent, err := holder.ResolveContainer(ref)
		if err != nil {
			problems = append(problems, holder.problem("BaseContainer",
				fmt.Sprintf("container %q extends %q, which does not resolve",
					container.Name, ref), ErrUnresolvedReference))
			continue
		}
		base[container] = parent
	}

	// Colours: absent means unvisited, grey means on the current path, black
	// means proved acyclic. Meeting grey is a cycle; meeting black is a
	// shortcut.
	const (
		grey  = 1
		black = 2
	)
	colour := make(map[*SequenceContainer]int, len(containers))

	for _, start := range containers {
		if colour[start] != 0 {
			continue
		}

		// Walk this chain iteratively, marking as we go.
		var path []*SequenceContainer
		current := start
		for colour[current] != black {
			if colour[current] == grey {
				problems = append(problems, holders[current].problem("BaseContainer",
					fmt.Sprintf("container %q inherits in a circle", current.Name),
					ErrContainerCycle))
				break
			}
			colour[current] = grey
			path = append(path, current)

			next, ok := base[current]
			if !ok {
				break
			}
			current = next
		}

		// Everything on this path is settled now.
		for _, container := range path {
			colour[container] = black
		}
	}

	return problems
}

// sentinelOf pulls the sentinel out of a wrapped error so ValidationError
// carries something errors.Is can match.
func sentinelOf(err error) error {
	switch {
	case errors.Is(err, ErrUnresolvedReference):
		return ErrUnresolvedReference
	case errors.Is(err, ErrInvalidReference):
		return ErrInvalidReference
	case errors.Is(err, ErrContainerCycle):
		return ErrContainerCycle
	case errors.Is(err, ErrDuplicateName):
		return ErrDuplicateName
	default:
		return err
	}
}
