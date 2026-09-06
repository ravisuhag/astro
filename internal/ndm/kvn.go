package ndm

import (
	"strconv"
	"strings"
)

// MaxLineLength is the ceiling CCSDS 502.0-B-3 clause 7.3.2 puts on a line,
// excluding the terminator. Clause 7.3.3 exempts the OCM, so Scanner only
// enforces it when asked.
const MaxLineLength = 254

// Kind says what a line is. Clause 7.3.1 lists five kinds; header, metadata
// and data lines are not distinguishable by their syntax alone, so what is
// left is whether a line is blank, a comment, an assignment, or something
// else — a data line whose fields are positional rather than named.
type Kind int

const (
	// Blank is a line of nothing but whitespace. Clause 7.3.5 gives it no
	// meaning and allows it anywhere.
	Blank Kind = iota
	// Comment is a line starting with the COMMENT keyword (clause 7.8.5).
	Comment
	// Assignment is a 'keyword = value' line (clause 7.4).
	Assignment
	// Free is any other non-blank line: an ephemeris row, a covariance row, a
	// manoeuvre row. Clause 7.4.1.2 onward carve these out of the KVN rule.
	Free
)

// Line is one line of a navigation data message.
type Line struct {
	Kind Kind
	// Keyword is the assignment's keyword, or "COMMENT" for a comment. It is
	// empty for blank and free lines.
	Keyword string
	// Value is what was assigned, or the comment text. Leading and trailing
	// whitespace is stripped for an assignment (clauses 7.4.6, 7.4.7) and kept
	// for a comment (clause 7.8.5).
	Value string
	// Text is the line as it stood, without its terminator.
	Text string
	// Number is the 1-based line number, for error messages.
	Number int
}

// sectionKeyword reports whether a keyword is one of the section delimiters
// that clause 7.4.2 exempts from the KVN rule: anything ending in _START or
// _STOP. They appear on a line by themselves with no equals sign.
func sectionKeyword(s string) bool {
	return strings.HasSuffix(s, "_START") || strings.HasSuffix(s, "_STOP")
}

// Scanner walks the lines of a navigation data message.
//
// It handles all four line terminators clause 7.3.7 allows: a bare carriage
// return, a bare line feed, a CR/LF pair, and an LF/CR pair. That last one is
// the reason this is not bufio.Scanner with ScanLines — an LF followed by a CR
// is one terminator, not a line feed followed by a blank line, and reading it
// the ordinary way shifts every subsequent line number by one.
type Scanner struct {
	data []byte
	pos  int

	line       Line
	lineNumber int

	strictLength bool
	pushedBack   bool
	err          error
}

// NewScanner returns a Scanner over data. When strictLength is set, a line
// past MaxLineLength is an error; pass false for message types clause 7.3.3
// exempts.
func NewScanner(data []byte, strictLength bool) *Scanner {
	return &Scanner{data: data, strictLength: strictLength}
}

// Scan advances to the next line and reports whether there was one. Blank
// lines are returned like any other; a caller that does not care about them
// skips on Kind.
func (s *Scanner) Scan() bool {
	// A caller that has read one line too far puts it back rather than
	// buffering the whole file. Reading a header means stopping at the first
	// keyword that is not one, and that keyword belongs to whoever reads next.
	if s.pushedBack {
		s.pushedBack = false
		return true
	}
	if s.err != nil || s.pos >= len(s.data) {
		return false
	}

	start := s.pos
	end := -1
	for i := s.pos; i < len(s.data); i++ {
		if s.data[i] == '\r' || s.data[i] == '\n' {
			end = i
			break
		}
	}
	if end < 0 {
		// A final line with no terminator. Clause 7.3.7 requires one, but
		// refusing the file over a missing final newline would reject
		// something every editor produces and every reader accepts.
		end = len(s.data)
		s.pos = len(s.data)
	} else {
		s.pos = end + 1
		// Consume the partner of a two-character terminator: CR/LF or LF/CR.
		if s.pos < len(s.data) {
			if pair := s.data[end]; (pair == '\r' && s.data[s.pos] == '\n') ||
				(pair == '\n' && s.data[s.pos] == '\r') {
				s.pos++
			}
		}
	}

	s.lineNumber++
	text := string(s.data[start:end])

	if err := checkCharacters(text); err != nil {
		s.err = &LineError{Line: s.lineNumber, Err: err}
		return false
	}
	if s.strictLength && len(text) > MaxLineLength {
		s.err = &LineError{Line: s.lineNumber, Err: ErrLineTooLong}
		return false
	}

	s.line = classify(text, s.lineNumber)
	return true
}

// Line returns the line Scan just read.
func (s *Scanner) Line() Line { return s.line }

// Unread puts the current line back, so the next Scan returns it again. Only
// one line can be held at a time.
func (s *Scanner) Unread() { s.pushedBack = true }

// Err returns the first error Scan hit, if any.
func (s *Scanner) Err() error { return s.err }

// checkCharacters enforces clause 7.3.4: printable ASCII and blanks only.
func checkCharacters(text string) error {
	for i := 0; i < len(text); i++ {
		if c := text[i]; c < 0x20 || c > 0x7E {
			return ErrControlCharacter
		}
	}
	return nil
}

// classify decides what kind of line this is and pulls out its parts.
func classify(text string, number int) Line {
	line := Line{Text: text, Number: number}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		line.Kind = Blank
		return line
	}

	// Clause 7.8.5: a comment line begins with COMMENT followed by at least
	// one space, and the keyword repeats on every line of a multi-line
	// comment. A bare COMMENT with nothing after it is an empty comment.
	if trimmed == "COMMENT" {
		line.Kind, line.Keyword = Comment, "COMMENT"
		return line
	}
	if rest, ok := strings.CutPrefix(trimmed, "COMMENT "); ok {
		line.Kind, line.Keyword = Comment, "COMMENT"
		// Clause 7.8.5 keeps white space significant in a comment value, so
		// only the single separating space is removed.
		line.Value = rest
		return line
	}

	// Clause 7.4.2: a section delimiter stands alone with no equals sign.
	if sectionKeyword(trimmed) && !strings.Contains(trimmed, "=") {
		line.Kind, line.Keyword = Assignment, trimmed
		return line
	}

	keyword, value, found := strings.Cut(trimmed, "=")
	if !found {
		line.Kind = Free
		return line
	}

	// Clauses 7.4.5 to 7.4.7: whitespace around the keyword, around the equals
	// sign and before the end of the line is not significant.
	line.Kind = Assignment
	line.Keyword = strings.TrimSpace(keyword)
	line.Value = strings.TrimSpace(value)
	return line
}

// Assignment reads the line as a 'keyword = value' pair, checking the keyword
// rules of clause 7.4.4.
func (l Line) Assignment() (keyword, value string, err error) {
	if l.Kind != Assignment {
		return "", "", ErrNotAnAssignment
	}
	if l.Keyword == "" {
		return "", "", ErrEmptyKeyword
	}
	if l.Keyword != strings.ToUpper(l.Keyword) || strings.ContainsAny(l.Keyword, " \t") {
		return "", "", ErrKeywordNotUppercase
	}
	return l.Keyword, l.Value, nil
}

// LineError attaches a line number to a syntax error, so a caller reading a
// 10,000-line ephemeris is told where the problem is rather than only what it
// was.
type LineError struct {
	Line int
	Err  error
}

func (e *LineError) Error() string {
	return "ndm: line " + strconv.Itoa(e.Line) + ": " + e.Err.Error()
}

func (e *LineError) Unwrap() error { return e.Err }

// At wraps err with a line number.
func At(line int, err error) error {
	if err == nil {
		return nil
	}
	return &LineError{Line: line, Err: err}
}

// Writer builds a navigation data message.
//
// Every line it writes is terminated with a bare line feed. Clause 7.3.7
// allows four terminators and this picks one; a reader that follows the clause
// accepts it, and choosing per-platform line endings would make the same
// message hash differently on Windows.
type Writer struct {
	buf strings.Builder
}

// Assign writes a 'keyword = value' line.
func (w *Writer) Assign(keyword, value string) {
	w.buf.WriteString(keyword)
	w.buf.WriteString(" = ")
	w.buf.WriteString(value)
	w.buf.WriteByte('\n')
}

// AssignUnits writes a 'keyword = value [units]' line. Clause 7.7.1.1 requires
// at least one blank before the bracket, and forbids '[n/a]' — a dimensionless
// item simply carries no suffix.
func (w *Writer) AssignUnits(keyword, value, units string) {
	if units == "" || units == "n/a" {
		w.Assign(keyword, value)
		return
	}
	w.Assign(keyword, value+" ["+units+"]")
}

// Comment writes one COMMENT line per line of text, as clause 7.8.5 requires.
func (w *Writer) Comment(text string) {
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			w.buf.WriteString("COMMENT\n")
			continue
		}
		w.buf.WriteString("COMMENT ")
		w.buf.WriteString(line)
		w.buf.WriteByte('\n')
	}
}

// Comments writes each entry as its own COMMENT line.
func (w *Writer) Comments(texts []string) {
	for _, text := range texts {
		w.Comment(text)
	}
}

// Section writes a bare section delimiter such as META_START (clause 7.4.2).
func (w *Writer) Section(keyword string) {
	w.buf.WriteString(keyword)
	w.buf.WriteByte('\n')
}

// Raw writes a line as given, for the data rows clause 7.4.1.2 onward exempt
// from KVN.
func (w *Writer) Raw(line string) {
	w.buf.WriteString(line)
	w.buf.WriteByte('\n')
}

// Blank writes an empty line. Clause 7.3.5 allows one anywhere and gives it no
// meaning, so this is for readability only.
func (w *Writer) Blank() { w.buf.WriteByte('\n') }

// Bytes returns the message.
func (w *Writer) Bytes() []byte { return []byte(w.buf.String()) }

// String returns the message as a string.
func (w *Writer) String() string { return w.buf.String() }
