package odm

import (
	"strings"
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// covarianceOrder lists the 21 lower triangular positions in the order
// clause 5.2.5.4 puts them on the wire: from [1,1] to [6,6], row by row, left
// to right. Row r carries r+1 values.
var covarianceOrder = func() [][2]int {
	var out [][2]int
	for row := 0; row < 6; row++ {
		for col := 0; col <= row; col++ {
			out = append(out, [2]int{row, col})
		}
	}
	return out
}()

// Encode writes the message in 'keyword = value' notation.
func (m *OEM) Encode() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	var w ndm.Writer
	if err := m.Header.toNDM().Write(&w, oemHeaderSpec); err != nil {
		return nil, err
	}

	for i := range m.Blocks {
		w.Blank()
		if err := writeEphemerisBlock(&w, &m.Blocks[i]); err != nil {
			return nil, err
		}
	}
	return w.Bytes(), nil
}

func writeEphemerisBlock(w *ndm.Writer, b *EphemerisBlock) error {
	md := &b.Metadata

	w.Section(keywordMetaStart)
	w.Comments(md.Comments)
	w.Assign("OBJECT_NAME", md.ObjectName)
	w.Assign("OBJECT_ID", md.ObjectID)
	w.Assign("CENTER_NAME", md.CenterName)
	w.Assign("REF_FRAME", md.RefFrame)
	if err := writeOptionalEpoch(w, "REF_FRAME_EPOCH", md.RefFrameEpoch); err != nil {
		return err
	}
	w.Assign("TIME_SYSTEM", md.TimeSystem)

	if err := writeEpoch(w, "START_TIME", md.StartTime); err != nil {
		return err
	}
	if err := writeOptionalEpoch(w, "USEABLE_START_TIME", md.UseableStartTime); err != nil {
		return err
	}
	if err := writeOptionalEpoch(w, "USEABLE_STOP_TIME", md.UseableStopTime); err != nil {
		return err
	}
	if err := writeEpoch(w, "STOP_TIME", md.StopTime); err != nil {
		return err
	}

	if md.Interpolation != "" {
		w.Assign("INTERPOLATION", md.Interpolation)
		w.Assign("INTERPOLATION_DEGREE", ndm.FormatInt(md.InterpolationDegree))
	}
	w.Section(keywordMetaStop)

	w.Comments(b.Comments)
	for _, line := range b.Lines {
		row, err := formatEphemerisLine(line)
		if err != nil {
			return err
		}
		w.Raw(row)
	}

	if len(b.Covariances) == 0 {
		return nil
	}
	w.Section(keywordCovarianceStart)
	w.Comments(b.CovarianceComments)
	for _, c := range b.Covariances {
		if err := writeEpoch(w, "EPOCH", c.Epoch); err != nil {
			return err
		}
		if c.RefFrame != "" {
			w.Assign("COV_REF_FRAME", c.RefFrame)
		}
		writeCovarianceMatrix(w, c.Matrix)
	}
	w.Section(keywordCovarianceStop)
	return nil
}

// formatEphemerisLine writes one positional data row (clause 5.2.4.1). Fields
// are separated by a single space; clause 5.2.4.3 asks only for at least one.
func formatEphemerisLine(line EphemerisLine) (string, error) {
	epoch, err := ndm.FormatEpoch(line.Epoch, ndm.EpochPrecision(line.Epoch))
	if err != nil {
		return "", err
	}

	fields := []string{
		epoch,
		ndm.FormatValue(line.X), ndm.FormatValue(line.Y), ndm.FormatValue(line.Z),
		ndm.FormatValue(line.XDot), ndm.FormatValue(line.YDot), ndm.FormatValue(line.ZDot),
	}
	if line.HasAcceleration {
		fields = append(fields,
			ndm.FormatValue(line.XDDot), ndm.FormatValue(line.YDDot), ndm.FormatValue(line.ZDDot))
	}
	return strings.Join(fields, " "), nil
}

// writeCovarianceMatrix writes the lower triangle as six lines carrying one to
// six values, which is the shape figure G-13 prints.
func writeCovarianceMatrix(w *ndm.Writer, matrix [6][6]float64) {
	for row := 0; row < 6; row++ {
		fields := make([]string, 0, row+1)
		for col := 0; col <= row; col++ {
			fields = append(fields, ndm.FormatValue(matrix[row][col]))
		}
		w.Raw(strings.Join(fields, " "))
	}
}

func writeEpoch(w *ndm.Writer, keyword string, t time.Time) error {
	value, err := ndm.FormatEpoch(t, ndm.EpochPrecision(t))
	if err != nil {
		return err
	}
	w.Assign(keyword, value)
	return nil
}

func writeOptionalEpoch(w *ndm.Writer, keyword string, t *time.Time) error {
	if t == nil {
		return nil
	}
	return writeEpoch(w, keyword, *t)
}
