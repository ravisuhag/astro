package xtce

import (
	"fmt"
	"strings"
)

// Name references and how they resolve.
//
// XTCE names things by path. The schema's NameReferenceType restricts a
// reference to the pattern
//
//	/?(([^./:\[\]]+|\.|\.\.)/)*([^./:\[\]]+)+
//
// which allows three shapes, and they behave the way file paths do:
//
//	/Spacecraft/Power/BusVoltage    absolute, from the root
//	../Bus/PacketID                 relative, going up a level first
//	BusVoltage                      bare
//
// The bare form is the one with a wrinkle. It is not simply "in this
// SpaceSystem": a bare name is searched for in the referencing system and then
// in each ancestor up to the root, so a type defined once at the top is usable
// everywhere below. That search is what makes a shared type library work, and
// it is why resolution needs to know where a reference was written, not just
// what it says.

// Root walks up to the top of the tree.
func (s *SpaceSystem) Root() *SpaceSystem {
	current := s
	for current.parent != nil {
		current = current.parent
	}
	return current
}

// QualifiedName is the absolute path of this SpaceSystem, starting with a
// slash.
func (s *SpaceSystem) QualifiedName() string {
	if s.parent == nil {
		return "/" + s.Name
	}
	return s.parent.QualifiedName() + "/" + s.Name
}

// Walk visits this SpaceSystem and every system below it, depth first. Return
// false from fn to stop the walk.
func (s *SpaceSystem) Walk(fn func(*SpaceSystem) bool) {
	s.walk(fn)
}

// walk does the visiting and reports whether to keep going.
func (s *SpaceSystem) walk(fn func(*SpaceSystem) bool) bool {
	if !fn(s) {
		return false
	}
	for _, child := range s.SubSystems {
		if !child.walk(fn) {
			return false
		}
	}
	return true
}

// splitRef breaks a reference into its path segments and says whether it is
// absolute.
func splitRef(ref string) (segments []string, absolute bool, err error) {
	if ref == "" {
		return nil, false, fmt.Errorf("%w: empty", ErrInvalidReference)
	}
	absolute = strings.HasPrefix(ref, "/")
	trimmed := strings.TrimPrefix(ref, "/")
	if trimmed == "" {
		return nil, false, fmt.Errorf("%w: %q names nothing", ErrInvalidReference, ref)
	}

	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" {
			return nil, false, fmt.Errorf("%w: %q has an empty segment", ErrInvalidReference, ref)
		}
		if segment != "." && segment != ".." && strings.ContainsAny(segment, ":[]") {
			return nil, false, fmt.Errorf("%w: %q has an illegal character", ErrInvalidReference, ref)
		}
		segments = append(segments, segment)
	}
	return segments, absolute, nil
}

// findSubSystem looks for a direct child by name.
func (s *SpaceSystem) findSubSystem(name string) *SpaceSystem {
	for _, child := range s.SubSystems {
		if child.Name == name {
			return child
		}
	}
	return nil
}

// resolveRef follows a reference from the system it was written in, and
// returns the system holding the named thing along with the bare name.
//
// It does not look the name up — the caller does that, because the same path
// walk serves parameters, types and containers. What it settles is the harder
// half: which SpaceSystem to look in.
func resolveRef(from *SpaceSystem, ref string) (holder *SpaceSystem, name string, err error) {
	segments, absolute, err := splitRef(ref)
	if err != nil {
		return nil, "", err
	}

	// The last segment is the thing's own name; the ones before it are the
	// path to the system holding it.
	name = segments[len(segments)-1]
	path := segments[:len(segments)-1]

	if name == "." || name == ".." {
		return nil, "", fmt.Errorf("%w: %q ends in a path segment", ErrInvalidReference, ref)
	}

	current := from
	if absolute {
		current = from.Root()
		// An absolute reference starts with the root system's own name, the
		// way /Spacecraft/Power does. Skip it when it matches.
		if len(path) > 0 && path[0] == current.Name {
			path = path[1:]
		}
	}

	for _, segment := range path {
		switch segment {
		case ".":
			// Stay where we are.
		case "..":
			if current.parent == nil {
				return nil, "", fmt.Errorf("%w: %q goes above the root", ErrInvalidReference, ref)
			}
			current = current.parent
		default:
			child := current.findSubSystem(segment)
			if child == nil {
				return nil, "", fmt.Errorf("%w: %q has no SpaceSystem %q",
					ErrUnresolvedReference, ref, segment)
			}
			current = child
		}
	}

	return current, name, nil
}

// hasPath reports whether the reference names a path rather than a bare name.
// A bare name is the only kind that searches ancestors.
func hasPath(ref string) bool {
	return strings.Contains(ref, "/")
}

// ResolveParameter follows a parameter reference written in this SpaceSystem.
func (s *SpaceSystem) ResolveParameter(ref string) (*Parameter, error) {
	holder, name, err := resolveRef(s, ref)
	if err != nil {
		return nil, err
	}

	if param := holder.localParameter(name); param != nil {
		return param, nil
	}
	// A bare name searches upwards; a path does not.
	if !hasPath(ref) {
		for ancestor := holder.parent; ancestor != nil; ancestor = ancestor.parent {
			if param := ancestor.localParameter(name); param != nil {
				return param, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: no parameter %q from %s", ErrUnresolvedReference, ref, s.QualifiedName())
}

// ResolveParameterType follows a parameter-type reference written in this
// SpaceSystem.
func (s *SpaceSystem) ResolveParameterType(ref string) (ParameterType, error) {
	holder, name, err := resolveRef(s, ref)
	if err != nil {
		return nil, err
	}

	if t := holder.localParameterType(name); t != nil {
		return t, nil
	}
	if !hasPath(ref) {
		for ancestor := holder.parent; ancestor != nil; ancestor = ancestor.parent {
			if t := ancestor.localParameterType(name); t != nil {
				return t, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: no parameter type %q from %s", ErrUnresolvedReference, ref, s.QualifiedName())
}

// ResolveContainer follows a container reference written in this SpaceSystem.
func (s *SpaceSystem) ResolveContainer(ref string) (*SequenceContainer, error) {
	holder, name, err := resolveRef(s, ref)
	if err != nil {
		return nil, err
	}

	if c := holder.localContainer(name); c != nil {
		return c, nil
	}
	if !hasPath(ref) {
		for ancestor := holder.parent; ancestor != nil; ancestor = ancestor.parent {
			if c := ancestor.localContainer(name); c != nil {
				return c, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: no container %q from %s", ErrUnresolvedReference, ref, s.QualifiedName())
}

// localParameter looks in this SpaceSystem only, across both metadata sides.
//
// Both sides, because the schema's parameterNameKey covers
// TelemetryMetaData/ParameterSet and CommandMetaData/ParameterSet together:
// they share one namespace.
func (s *SpaceSystem) localParameter(name string) *Parameter {
	for _, set := range s.parameterSets() {
		for _, param := range set.Parameters {
			if param.Name == name {
				return param
			}
		}
	}
	return nil
}

// parameterSets returns the parameter sets of both metadata sides.
func (s *SpaceSystem) parameterSets() []*ParameterSet {
	var sets []*ParameterSet
	if s.TelemetryMetaData != nil && s.TelemetryMetaData.ParameterSet != nil {
		sets = append(sets, s.TelemetryMetaData.ParameterSet)
	}
	if s.CommandMetaData != nil && s.CommandMetaData.ParameterSet != nil {
		sets = append(sets, s.CommandMetaData.ParameterSet)
	}
	return sets
}

// parameterTypeSets returns the parameter-type sets of both metadata sides,
// which likewise share one namespace.
func (s *SpaceSystem) parameterTypeSets() []*ParameterTypeSet {
	var sets []*ParameterTypeSet
	if s.TelemetryMetaData != nil && s.TelemetryMetaData.ParameterTypeSet != nil {
		sets = append(sets, s.TelemetryMetaData.ParameterTypeSet)
	}
	if s.CommandMetaData != nil && s.CommandMetaData.ParameterTypeSet != nil {
		sets = append(sets, s.CommandMetaData.ParameterTypeSet)
	}
	return sets
}

// localParameterType looks in this SpaceSystem only.
func (s *SpaceSystem) localParameterType(name string) ParameterType {
	for _, set := range s.parameterTypeSets() {
		for _, t := range set.All() {
			if t.TypeName() == name {
				return t
			}
		}
	}
	return nil
}

// localContainer looks in this SpaceSystem only.
func (s *SpaceSystem) localContainer(name string) *SequenceContainer {
	if s.TelemetryMetaData == nil || s.TelemetryMetaData.ContainerSet == nil {
		return nil
	}
	for _, c := range s.TelemetryMetaData.ContainerSet.SequenceContainers {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// All returns every parameter type in the set, whatever its kind, in a stable
// order: integers, floats, enumerations, strings, binaries, booleans, times.
//
// The order is this package's, not the document's. The schema makes
// ParameterTypeSet a Set, so document order carries no meaning, and a stable
// order makes Humanize output comparable between runs.
func (p *ParameterTypeSet) All() []ParameterType {
	if p == nil {
		return nil
	}
	var all []ParameterType
	for _, t := range p.IntegerTypes {
		all = append(all, t)
	}
	for _, t := range p.FloatTypes {
		all = append(all, t)
	}
	for _, t := range p.EnumeratedTypes {
		all = append(all, t)
	}
	for _, t := range p.StringTypes {
		all = append(all, t)
	}
	for _, t := range p.BinaryTypes {
		all = append(all, t)
	}
	for _, t := range p.BooleanTypes {
		all = append(all, t)
	}
	for _, t := range p.AbsoluteTimeTypes {
		all = append(all, t)
	}
	return all
}

// Len reports how many parameter types the set holds.
func (p *ParameterTypeSet) Len() int { return len(p.All()) }

// FindParameter looks up a parameter by its qualified name, from the root of
// the tree.
//
// This is the entry point for a caller who has a name out of a display page or
// a configuration file. Unlike ResolveParameter it does not search ancestors:
// a qualified name says exactly where the parameter is.
func (s *SpaceSystem) FindParameter(qualifiedName string) (*Parameter, error) {
	system, name, err := s.findHolder(qualifiedName)
	if err != nil {
		return nil, err
	}
	if param := system.localParameter(name); param != nil {
		return param, nil
	}
	return nil, fmt.Errorf("%w: parameter %q", ErrNotFound, qualifiedName)
}

// FindContainer looks up a container by its qualified name.
func (s *SpaceSystem) FindContainer(qualifiedName string) (*SequenceContainer, error) {
	system, name, err := s.findHolder(qualifiedName)
	if err != nil {
		return nil, err
	}
	if c := system.localContainer(name); c != nil {
		return c, nil
	}
	return nil, fmt.Errorf("%w: container %q", ErrNotFound, qualifiedName)
}

// FindParameterType looks up a parameter type by its qualified name.
func (s *SpaceSystem) FindParameterType(qualifiedName string) (ParameterType, error) {
	system, name, err := s.findHolder(qualifiedName)
	if err != nil {
		return nil, err
	}
	if t := system.localParameterType(name); t != nil {
		return t, nil
	}
	return nil, fmt.Errorf("%w: parameter type %q", ErrNotFound, qualifiedName)
}

// FindSpaceSystem looks up a SpaceSystem by its qualified name.
func (s *SpaceSystem) FindSpaceSystem(qualifiedName string) (*SpaceSystem, error) {
	segments, _, err := splitRef(qualifiedName)
	if err != nil {
		return nil, err
	}

	current := s.Root()
	if segments[0] != current.Name {
		return nil, fmt.Errorf("%w: %q does not start at %q", ErrNotFound, qualifiedName, current.Name)
	}
	for _, segment := range segments[1:] {
		child := current.findSubSystem(segment)
		if child == nil {
			return nil, fmt.Errorf("%w: SpaceSystem %q", ErrNotFound, qualifiedName)
		}
		current = child
	}
	return current, nil
}

// findHolder splits a qualified name into the SpaceSystem holding the thing
// and the thing's own name.
func (s *SpaceSystem) findHolder(qualifiedName string) (*SpaceSystem, string, error) {
	segments, _, err := splitRef(qualifiedName)
	if err != nil {
		return nil, "", err
	}
	if len(segments) < 2 {
		return nil, "", fmt.Errorf("%w: %q is not qualified; it needs a SpaceSystem path",
			ErrInvalidReference, qualifiedName)
	}

	name := segments[len(segments)-1]
	systemPath := "/" + strings.Join(segments[:len(segments)-1], "/")

	system, err := s.FindSpaceSystem(systemPath)
	if err != nil {
		return nil, "", err
	}
	return system, name, nil
}

// Containers returns this SpaceSystem's containers.
func (s *SpaceSystem) Containers() []*SequenceContainer {
	if s.TelemetryMetaData == nil || s.TelemetryMetaData.ContainerSet == nil {
		return nil
	}
	return s.TelemetryMetaData.ContainerSet.SequenceContainers
}

// Parameters returns this SpaceSystem's parameters, telemetry side then
// command side.
func (s *SpaceSystem) Parameters() []*Parameter {
	var all []*Parameter
	for _, set := range s.parameterSets() {
		all = append(all, set.Parameters...)
	}
	return all
}

// ParameterTypes returns this SpaceSystem's parameter types.
func (s *SpaceSystem) ParameterTypes() []ParameterType {
	var all []ParameterType
	for _, set := range s.parameterTypeSets() {
		all = append(all, set.All()...)
	}
	return all
}

// MetaCommands returns this SpaceSystem's commands.
func (s *SpaceSystem) MetaCommands() []*MetaCommand {
	if s.CommandMetaData == nil || s.CommandMetaData.MetaCommandSet == nil {
		return nil
	}
	return s.CommandMetaData.MetaCommandSet.MetaCommands
}
