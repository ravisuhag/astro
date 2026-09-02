// Example: Decoding telemetry from a mission database
//
// The other examples hand-write a struct for every packet, which means the
// packet layout lives in Go code. A real mission keeps it in a database
// instead, and the ground system learns the layout by reading the file.
//
// This example shows both halves of that:
//
//	Loading:
//	  1. Parse mission.xml
//	  2. Check that its references resolve
//	  3. Look at what it declares
//
//	Decoding:
//	  1. Extract a packet whose type you already know
//	  2. Match a packet whose type you do not
//
// The database describes the CCSDS primary header once, as an abstract
// container, then two packet types that extend it. Each says which APID
// selects it, which is what makes matching possible.
package main

import (
	"bytes"
	"embed"
	"encoding/binary"
	"fmt"
	"log"
	"strconv"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/xtce"
)

//go:embed mission.xml
var mission embed.FS

const (
	apidPower   = 100 // the container PowerReport restricts on
	apidThermal = 101 // the container ThermalReport restricts on
)

func main() {
	file, err := mission.Open("mission.xml")
	if err != nil {
		log.Fatalf("opening the database: %v", err)
	}
	defer func() { _ = file.Close() }()

	// Load parses. It says the file is well-formed XTCE and nothing more.
	db, err := xtce.Load(file)
	if err != nil {
		log.Fatalf("loading the database: %v", err)
	}

	// Validate is a separate step because a database being edited often has
	// references that do not resolve yet. Load it during authoring, validate
	// it before flight.
	if err := db.Validate(); err != nil {
		log.Fatalf("validating the database: %v", err)
	}

	fmt.Println("--- The database ---")
	fmt.Println()
	fmt.Printf("  Space system ... %s\n", db.QualifiedName())
	fmt.Printf("  Parameters ..... %d\n", len(db.Parameters()))
	fmt.Printf("  Containers ..... %d\n", len(db.Containers()))
	fmt.Println()
	for _, container := range db.Containers() {
		kind := "packet"
		if container.Abstract {
			kind = "abstract"
		}
		fmt.Printf("  %-14s %-8s %s\n", container.Name, kind, container.ShortDescription)
	}
	fmt.Println()

	// A layout is the container flattened: inheritance worked through, and a
	// bit offset and width settled for every field. It depends only on the
	// database, so build it once per packet type and keep it.
	fmt.Println("--- The layout of PowerReport ---")
	fmt.Println()

	layout, err := db.LayoutOf("/Demosat/PowerReport")
	if err != nil {
		log.Fatalf("laying out the power report: %v", err)
	}

	fmt.Printf("  %d fields, %d bits\n", len(layout.Fields), layout.BitSize)
	fmt.Println()
	for _, field := range layout.Fields {
		fmt.Printf("  bit %3d  %2d wide  %s\n", field.BitOffset, field.BitSize, field.Name)
	}
	fmt.Println()

	// Note that the primary header fields are in there. They came from the
	// base container, which is how a mission describes them once and extends
	// them twenty times.

	fmt.Println("--- Extracting a packet you know the type of ---")
	fmt.Println()

	power := powerPacket(28.140, 4.2, 2)
	fmt.Printf("  %d octets: % x\n", len(power), power)
	fmt.Println()

	packet, err := layout.Extract(power)
	if err != nil {
		log.Fatalf("extracting the power report: %v", err)
	}
	// Err reports the first field that failed. Extract keeps going past a bad
	// field, so one unsupported encoding does not hide everything after it.
	if err := packet.Err(); err != nil {
		fmt.Printf("  warning: %v\n", err)
	}
	printValues(packet)

	// The engineering value is the point of the exercise. The wire carried
	// 56280; the database knows that means 28.14 volts.
	if voltage, ok := packet.Get("BusVoltage"); ok {
		volts, _ := voltage.Float()
		fmt.Printf("  BusVoltage raw %v becomes %.3f V\n", voltage.Raw, volts)
	}
	fmt.Println()

	fmt.Println("--- Matching a packet you do not ---")
	fmt.Println()

	// A ground station taking a stream off an antenna does not know what each
	// packet is. Match starts at a container and follows every child whose
	// restriction criteria the packet satisfies, taking the deepest one that
	// fits.
	root, err := db.FindContainer("/Demosat/PrimaryHeader")
	if err != nil {
		log.Fatalf("finding the root container: %v", err)
	}

	for _, unknown := range [][]byte{
		power,
		thermalPacket(-12.75, 21.30, true),
	} {
		matched, err := db.Match(root, unknown)
		if err != nil {
			log.Fatalf("matching a packet: %v", err)
		}

		fmt.Printf("  %d octets matched %s\n", len(unknown), matched.Layout.Container.Name)
		printValues(matched)
		fmt.Println()
	}
}

// powerPacket builds a real Space Packet for APID 100. The payload is raw
// counts, which is what a spacecraft actually sends: the calibration lives in
// the database, not on the wire.
func powerPacket(volts, amps float64, mode uint8) []byte {
	body := new(bytes.Buffer)
	_ = binary.Write(body, binary.BigEndian, uint16(volts/0.0005)) // 0.5 mV per count
	_ = binary.Write(body, binary.BigEndian, uint16(amps/5.0*32768))
	body.WriteByte(mode)

	return encode(spp.NewTMPacket(apidPower, body.Bytes(),
		spp.WithSequenceCount(7)))
}

// thermalPacket builds one for APID 101. Same header, different payload, and
// the APID is the only thing that tells them apart.
func thermalPacket(radiator, battery float64, heater bool) []byte {
	body := new(bytes.Buffer)
	_ = binary.Write(body, binary.BigEndian, int16(radiator*100)) // centidegrees
	_ = binary.Write(body, binary.BigEndian, int16(battery*100))
	if heater {
		body.WriteByte(1)
	} else {
		body.WriteByte(0)
	}

	return encode(spp.NewTMPacket(apidThermal, body.Bytes(),
		spp.WithSequenceCount(8)))
}

// printValues shows the payload fields. The primary header ones are correct
// too, they are just not what you came for.
func printValues(packet *xtce.Packet) {
	for _, value := range packet.Values {
		switch value.Field.Parameter.Name {
		case "Version", "PacketType", "SecondaryHeaderFlag",
			"SequenceFlags", "PacketLength":
			continue
		}
		if value.Err != nil {
			fmt.Printf("    %-14s failed: %v\n", value.Name(), value.Err)
			continue
		}
		fmt.Printf("    %-14s %s\n", value.Field.Parameter.Name, format(value.Engineering))
	}
}

// format trims the noise off a calibrated float. A count of 27524 through a
// spline does not land on a round number of amperes, and printing every digit
// of that is not useful.
func format(engineering any) string {
	if number, ok := engineering.(float64); ok {
		return strconv.FormatFloat(number, 'f', -1, 32)
	}
	return fmt.Sprintf("%v", engineering)
}

func encode(packet *spp.SpacePacket, err error) []byte {
	if err != nil {
		log.Fatalf("building the packet: %v", err)
	}
	encoded, err := packet.Encode()
	if err != nil {
		log.Fatalf("encoding the packet: %v", err)
	}
	return encoded
}
