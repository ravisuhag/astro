package adm_test

import (
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/adm"
)

// The APM's risk is its six delimited blocks: a stop must match its start, a
// start must not nest, and each block allows a different keyword set.
func FuzzDecodeAPM(f *testing.F) {
	f.Add(figureG1)
	f.Add(figureG3)
	f.Add("")
	f.Add("CCSDS_APM_VERS = 2.0\n")

	f.Add("CCSDS_APM_VERS = 2.0\nCREATION_DATE = 2003-09-30T19:23:57\nORIGINATOR = X\nQUAT_START\n")
	f.Add("CCSDS_APM_VERS = 2.0\nCREATION_DATE = 2003-09-30T19:23:57\nORIGINATOR = X\nQUAT_STOP\n")
	f.Add("CCSDS_APM_VERS = 2.0\nCREATION_DATE = 2003-09-30T19:23:57\nORIGINATOR = X\nQUAT_START\nEULER_STOP\n")

	f.Fuzz(func(t *testing.T, data string) {
		m, err := adm.DecodeAPM([]byte(data))
		if err != nil {
			return
		}

		if m.Humanize() == "" {
			t.Fatal("Humanize returned nothing for a message that decoded")
		}
		encoded, err := m.Encode()
		if err != nil {
			t.Fatalf("a message that decoded failed to encode: %v", err)
		}

		again, err := adm.DecodeAPM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}
		if (again.Quaternion == nil) != (m.Quaternion == nil) ||
			(again.Euler == nil) != (m.Euler == nil) ||
			(again.Spin == nil) != (m.Spin == nil) ||
			(again.Inertia == nil) != (m.Inertia == nil) {
			t.Fatal("a block appeared or vanished on round trip")
		}
		if len(again.Maneuvers) != len(m.Maneuvers) {
			t.Fatal("the maneuver count changed on round trip")
		}
		if m.Quaternion != nil && again.Quaternion.Quaternion != m.Quaternion.Quaternion {
			t.Fatal("the quaternion changed on round trip")
		}
	})
}

// The AEM's risk is that a data line's width comes from the metadata. A file
// whose ATTITUDE_TYPE and line width disagree must be refused rather than
// read as something else.
func FuzzDecodeAEM(f *testing.F) {
	f.Add(figureG4)
	f.Add("")
	f.Add("CCSDS_AEM_VERS = 2.0\n")

	f.Add("CCSDS_AEM_VERS = 2.0\nCREATION_DATE = 2002-11-04T17:22:31\nORIGINATOR = X\nMETA_START\n")
	f.Add("CCSDS_AEM_VERS = 2.0\nCREATION_DATE = 2002-11-04T17:22:31\nORIGINATOR = X\nDATA_START\nDATA_STOP\n")

	f.Fuzz(func(t *testing.T, data string) {
		m, err := adm.DecodeAEM([]byte(data))
		if err != nil {
			return
		}

		if m.Humanize() == "" {
			t.Fatal("Humanize returned nothing for a message that decoded")
		}
		encoded, err := m.Encode()
		if err != nil {
			t.Fatalf("a message that decoded failed to encode: %v", err)
		}

		again, err := adm.DecodeAEM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}
		if again.Records() != m.Records() || len(again.Blocks) != len(m.Blocks) {
			t.Fatal("the shape changed on round trip")
		}
		for i := range m.Blocks {
			// The type decides how the values are read, so it must survive.
			if again.Blocks[i].Metadata.Type != m.Blocks[i].Metadata.Type {
				t.Fatalf("block %d attitude type changed on round trip", i)
			}
			for j := range m.Blocks[i].Lines {
				if len(again.Blocks[i].Lines[j].Values) != len(m.Blocks[i].Lines[j].Values) {
					t.Fatalf("block %d line %d width changed on round trip", i, j)
				}
			}
		}
	})
}

// The XML form of an AEM is where the attitude type stops being a width and
// becomes an element name, so a file whose type and inner element disagree has
// to be refused rather than read as another type.
func FuzzDecodeXMLAEM(f *testing.F) {
	f.Add("")
	f.Add("<aem/>")
	f.Add(`<aem id="CCSDS_AEM_VERS" version="2.0"><header/><body><segment><metadata/><data/></segment></body></aem>`)

	if m, err := adm.DecodeAEM([]byte(figureG4)); err == nil {
		if encoded, err := m.EncodeXML(); err == nil {
			f.Add(string(encoded))
		}
	}
	if m, err := adm.DecodeAPM([]byte(figureG3)); err == nil {
		if encoded, err := m.EncodeXML(); err == nil {
			f.Add(string(encoded))
		}
	}

	f.Fuzz(func(t *testing.T, data string) {
		m, err := adm.DecodeXMLAEM([]byte(data))
		if err != nil {
			return
		}

		encoded, err := m.EncodeXML()
		if err != nil {
			t.Fatalf("a message that decoded failed to encode: %v", err)
		}
		again, err := adm.DecodeXMLAEM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own XML failed: %v\n%s", err, encoded)
		}
		if again.Records() != m.Records() {
			t.Fatal("the record count changed on an XML round trip")
		}
		for i := range m.Blocks {
			if again.Blocks[i].Metadata.Type != m.Blocks[i].Metadata.Type {
				t.Fatalf("block %d attitude type changed on an XML round trip", i)
			}
		}

		// And crossing to the key-value form keeps the values.
		kvn, err := m.Encode()
		if err != nil {
			t.Fatalf("a message that decoded from XML failed to encode as KVN: %v", err)
		}
		crossed, err := adm.DecodeAEM(kvn)
		if err != nil {
			t.Fatalf("the key-value form written from XML does not read back: %v", err)
		}
		if crossed.Records() != m.Records() {
			t.Fatal("the record count changed crossing the forms")
		}
	})
}

// The ACM is the largest surface in the package and the one with the least
// typing behind it. Its risks are its own: six kinds of delimited section that
// can repeat or never close, a sensor sub-block nested inside one of them,
// time tags that may be a date or a signed number, and row widths that come
// from two keywords which must agree with each other.
func FuzzDecodeACM(f *testing.F) {
	f.Add(acmSimple)
	f.Add(acmManeuver)
	f.Add(acmPhysical)
	f.Add(acmCovariance)
	f.Add("")
	f.Add("CCSDS_ACM_VERS = 2.0\n")

	const header = "CCSDS_ACM_VERS = 2.0\nCREATION_DATE = 1998-11-06T09:23:57\nORIGINATOR = X\n"
	const meta = "META_START\nOBJECT_NAME = A\nTIME_SYSTEM = UTC\nEPOCH_TZERO = 1998-12-18T14:28:15.1172\nMETA_STOP\n"

	// Delimiters out of place: a section that never closes, one closed by
	// another's delimiter, a sensor block outside the section that may hold
	// one, and a nested sensor block that never closes.
	f.Add(header + meta + "ATT_START\n")
	f.Add(header + meta + "ATT_START\nCOV_STOP\n")
	f.Add(header + meta + "ATT_START\nSENSOR_START\nSENSOR_STOP\nATT_STOP\n")
	f.Add(header + meta + "AD_START\nATTITUDE_STATES = QUATERNION\nSENSOR_START\nAD_STOP\n")

	// Row widths that disagree with the keywords declaring them, and a
	// relative time tag with no EPOCH_TZERO to resolve it against.
	f.Add(header + meta + "ATT_START\nREF_FRAME_A = A\nREF_FRAME_B = B\nNUMBER_STATES = 4\nATT_TYPE = QUATERNION\n0.0 1 2 3\nATT_STOP\n")
	f.Add(header + "META_START\nOBJECT_NAME = A\nTIME_SYSTEM = UTC\nMETA_STOP\nATT_START\n0.0 1.0\nATT_STOP\n")

	f.Fuzz(func(t *testing.T, data string) {
		m, err := adm.DecodeACM([]byte(data))
		if err != nil {
			return
		}

		if m.Humanize() == "" {
			t.Fatal("Humanize returned nothing for a message that decoded")
		}
		encoded, err := m.Encode()
		if err != nil {
			t.Fatalf("a message that decoded failed to encode: %v", err)
		}

		again, err := adm.DecodeACM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}

		// The section shape must survive, since it is what says how to read
		// every number in the file.
		if len(again.Attitudes) != len(m.Attitudes) ||
			len(again.Covariances) != len(m.Covariances) ||
			len(again.Maneuvers) != len(m.Maneuvers) {
			t.Fatal("the section counts changed on round trip")
		}
		if (again.Physical == nil) != (m.Physical == nil) ||
			(again.AttitudeDetermination == nil) != (m.AttitudeDetermination == nil) {
			t.Fatal("a section appeared or vanished on round trip")
		}
		if ad := m.AttitudeDetermination; ad != nil {
			if len(again.AttitudeDetermination.Sensors) != len(ad.Sensors) {
				t.Fatal("the sensor block count changed on round trip")
			}
		}
		for i := range m.Attitudes {
			for j, row := range m.Attitudes[i].Rows {
				if strings.Join(again.Attitudes[i].Rows[j].Fields, " ") !=
					strings.Join(row.Fields, " ") {
					t.Fatalf("attitude block %d row %d changed on round trip", i, j)
				}
			}
		}
	})
}

// The ACM's XML form has the same six sections behind a different parser, a
// nested sensor element the standard shows only by example, and a units
// attribute that is put back onto the value on the way out.
func FuzzDecodeXMLACM(f *testing.F) {
	f.Add(acmXML)
	f.Add("")
	f.Add("<acm/>")
	f.Add(`<acm id="CCSDS_ACM_VERS" version="2.0"><header/><body/></acm>`)
	f.Add(`<acm id="CCSDS_ACM_VERS" version="2.0"><header/><body><segment><metadata/><data/></segment></body></acm>`)

	f.Add(strings.Replace(acmXML, "<att>", "<phys/><att>", 1))
	f.Add(strings.Replace(acmXML, "<attLine>", "<covLine>", 1))
	f.Add(strings.Replace(acmXML, "</att>", "<sensorData><SENSOR_NUMBER>1</SENSOR_NUMBER></sensorData></att>", 1))

	f.Fuzz(func(t *testing.T, data string) {
		m, err := adm.DecodeXMLACM([]byte(data))
		if err != nil {
			return
		}

		encoded, err := m.EncodeXML()
		if err != nil {
			t.Fatalf("a message that decoded failed to encode: %v", err)
		}
		again, err := adm.DecodeXMLACM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own XML failed: %v\n%s", err, encoded)
		}
		if len(again.Attitudes) != len(m.Attitudes) {
			t.Fatal("the attitude block count changed on an XML round trip")
		}

		// And the two forms must still agree after a crossing.
		kvn, err := m.Encode()
		if err != nil {
			t.Fatalf("a message that decoded from XML failed to encode as KVN: %v", err)
		}
		crossed, err := adm.DecodeACM(kvn)
		if err != nil {
			t.Fatalf("the key-value form written from XML does not read back: %v\n%s", err, kvn)
		}
		if crossed.TimeSystem() != m.TimeSystem() {
			t.Fatal("the time system changed crossing the forms")
		}
	})
}
