package pus

// ST[08] function management, per ECSS-E-ST-70-41C clauses 6.8 and 8.8.
//
// One service, one subservice, one message type: TC[8,1] tells an application
// process to perform one of the functions it declares. What those functions
// are, what arguments they take and what performing one does are all outside
// the standard (clause 6.8.1.1). The standard fixes only the envelope.
//
// The standard also says, in that same clause, that missions should prefer
// their own service types over this one, and that ST[08] "remains in this
// version of the Standard for backward compatibility reasons". It is here
// because packets carrying it still fly, not because it is the good way to do
// this.
const (
	ServiceFunctionManagement uint8 = 8

	SubtypePerformFunction uint8 = 1 // TC[8,1] clause 8.8.2.1
)

// FunctionArguments is the optional argument group of TC[8,1]: the count N and
// the argument octets that follow it.
//
// The group is present only when the function takes arguments (clause 6.8.4c
// item 2), so a request for a function that takes none carries no count field
// at all rather than a count of zero.
//
// Raw is not split into individual arguments because it cannot be. Figure 8-87
// types each argument value as "deduced": its width comes from the function's
// argument declaration (clause 6.8.3.1b), which is mission configuration this
// package does not hold. Both ends of the link already share that declaration,
// so the octets are moved verbatim. Use SplitArguments when you do hold it.
type FunctionArguments struct {
	// Count is N, the number of arguments the instruction supplies. Whether
	// that is every argument of the function or only the ones being updated
	// depends on the function's supplying-arguments policy, clause 6.8.3.1c.
	Count uint64

	// Raw holds the argument ID and argument value fields, N times, exactly as
	// they travel.
	Raw []byte
}

// FunctionArgument is one decoded argument of TC[8,1].
type FunctionArgument struct {
	// ID identifies the argument within its function.
	ID uint64
	// Value is the argument value, still uninterpreted: the declaration gives
	// its width, not its meaning.
	Value []byte
}

// PerformFunctionRequest is TC[8,1], a request to perform a function.
//
// Clause 6.8.4b allows exactly one instruction per request, so there is no
// count of instructions and no list: the function ID and its arguments are the
// whole body.
type PerformFunctionRequest struct {
	Profile MissionProfile

	// FunctionID names the function to perform. Figure 8-87 types it as a
	// fixed character-string, so it travels in exactly FunctionIDSize octets:
	// a shorter name is padded with NUL, and a longer one is refused rather
	// than truncated, because a truncated name can name a different function.
	FunctionID string

	// Arguments is nil when the function takes no arguments.
	Arguments *FunctionArguments
}

// Key returns the message type.
func (PerformFunctionRequest) Key() MessageKey {
	return MessageKey{Service: ServiceFunctionManagement, Subtype: SubtypePerformFunction}
}

// Encode serializes the application data field.
func (r PerformFunctionRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}

	width := r.Profile.FunctionIDSize()
	if len(r.FunctionID) > width {
		return nil, ErrValueTooLarge
	}
	out := make([]byte, width, width+8+len(argumentRaw(r.Arguments)))
	copy(out, r.FunctionID)

	if r.Arguments == nil {
		return out, nil
	}

	out, err := putUint(out, r.Arguments.Count, r.Profile.FunctionArgumentCountSize())
	if err != nil {
		return nil, err
	}
	return append(out, r.Arguments.Raw...), nil
}

// argumentRaw returns the raw argument octets, or nil for an absent group. It
// exists only to size the Encode buffer without repeating the nil check.
func argumentRaw(a *FunctionArguments) []byte {
	if a == nil {
		return nil
	}
	return a.Raw
}

// DecodePerformFunctionRequest parses TC[8,1].
//
// The argument group's presence is what figure 8-87 calls "deduced": nothing
// in the message flags it. Here it is deduced from the message length, which
// is the one thing a decoder without the mission's function declarations can
// read. A body that is exactly the function ID carries no arguments; a longer
// one carries the group.
func DecodePerformFunctionRequest(profile MissionProfile, data []byte) (*PerformFunctionRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	width := profile.FunctionIDSize()
	if len(data) < width {
		return nil, ErrDataTooShort
	}

	request := &PerformFunctionRequest{
		Profile: profile,
		// A fixed character-string is padded to its width, and the padding is
		// not part of the name.
		FunctionID: trimNUL(string(data[:width])),
	}

	rest := data[width:]
	if len(rest) == 0 {
		return request, nil
	}

	countWidth := profile.FunctionArgumentCountSize()
	count, err := readUint(rest, countWidth)
	if err != nil {
		return nil, err
	}
	request.Arguments = &FunctionArguments{
		Count: count,
		Raw:   rest[countWidth:],
	}
	return request, nil
}

// SplitArguments splits the raw argument octets into individual arguments,
// using a caller-supplied width for each argument value.
//
// The width function is the mission's argument declaration, clause 6.8.3.1b:
// given an argument ID it returns how many octets that argument's value
// occupies. This package cannot supply it, and a decoder that guessed would
// mis-split the block silently.
//
// The count in the message is untrusted, so it is not used to size anything.
// The octets are walked until they run out, and a block that does not divide
// evenly into Count arguments is an error rather than a partial answer.
func (a *FunctionArguments) SplitArguments(profile MissionProfile, width func(argumentID uint64) (int, error)) ([]FunctionArgument, error) {
	if a == nil {
		return nil, nil
	}
	if width == nil {
		return nil, ErrInvalidProfile
	}
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	idWidth := profile.FunctionArgumentIDSize()
	out := make([]FunctionArgument, 0, min(a.Count, uint64(len(a.Raw))))

	rest := a.Raw
	for len(rest) > 0 {
		id, err := readUint(rest, idWidth)
		if err != nil {
			return nil, err
		}
		rest = rest[idWidth:]

		valueWidth, err := width(id)
		if err != nil {
			return nil, err
		}
		if valueWidth < 0 {
			return nil, ErrInvalidProfile
		}
		if len(rest) < valueWidth {
			return nil, ErrDataTooShort
		}
		out = append(out, FunctionArgument{ID: id, Value: rest[:valueWidth]})
		rest = rest[valueWidth:]
	}

	// The count is a claim about the block; a block that holds a different
	// number of arguments means the two ends disagree about the declaration,
	// and reporting the arguments anyway would hide that.
	if uint64(len(out)) != a.Count {
		return nil, ErrTrailingBytes
	}
	return out, nil
}

// Humanize returns a human-readable summary.
func (r PerformFunctionRequest) Humanize() string {
	out := "PUS TC[8,1] perform a function" +
		"\n  Function ID .. " + r.FunctionID
	if r.Arguments == nil {
		return out + "\n  Arguments .... none"
	}
	return out +
		"\n  Arguments .... " + itoa(int(r.Arguments.Count)) +
		"\n  Argument data  " + itoa(len(r.Arguments.Raw)) + " octet(s)"
}

// trimNUL drops the NUL padding of a fixed character-string field.
func trimNUL(s string) string {
	for len(s) > 0 && s[len(s)-1] == 0 {
		s = s[:len(s)-1]
	}
	return s
}

// registerST08 adds the ST[08] codecs to a registry.
func registerST08(r *Registry) error {
	return r.RegisterRequest(
		MessageKey{Service: ServiceFunctionManagement, Subtype: SubtypePerformFunction},
		func(p MissionProfile, data []byte) (Request, error) {
			return DecodePerformFunctionRequest(p, data)
		},
	)
}
