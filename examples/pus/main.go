// Example: A PUS service model
//
// This example demonstrates the ECSS Packet Utilization Standard riding inside
// CCSDS Space Packets:
//
//	Ground Side (requests):
//	  1. TC[3,5]  enable periodic housekeeping reporting
//	  2. TC[12,5] add a limit check on the battery voltage
//	  3. TC[11,4] schedule a command for later release
//
//	Spacecraft Side (reports):
//	  1. TM[1,1]  telecommand accepted
//	  2. TM[1,7]  execution completed
//	  3. TM[3,25] housekeeping parameter report
//	  4. TM[12,12] a check transitioned out of limits
//	  5. TM[5,2]  low severity anomaly event
//
// Every message is a real Space Packet with a PUS secondary header, encoded
// and decoded through a mission profile that pins the tailorable widths.
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ravisuhag/astro/pkg/pus"
	"github.com/ravisuhag/astro/pkg/spp"
)

const (
	apidCommanding = 100 // ground -> spacecraft, the commanding application
	apidPower      = 200 // spacecraft -> ground, the power application

	groundSourceID = 0x0001 // the entity issuing the telecommands
	groundDestID   = 0x0001 // the entity the reports are addressed to

	hkStructureID = 1      // the housekeeping report structure we enable
	paramBattery  = 0x0101 // the on-board parameter ID for battery voltage
	pmonBattery   = 0x0010 // the monitoring definition watching it
	eventLowBatt  = 0x2001 // the event definition raised when it goes low
)

// missionProfile pins every width ECSS-E-ST-70-41C leaves to the mission.
//
// DefaultProfile covers the common choices. The five capability flags are not
// widths: they decide whether a field is present at all, so both ends have to
// agree on them or every field after one shifts.
func missionProfile() pus.MissionProfile {
	profile := pus.DefaultProfile()
	profile.TCSpareBytes = 0
	profile.TMSpareBytes = 0
	profile.SupportsConditionalChecking = false
	profile.PerDefinitionMonitoringInterval = true
	profile.ExpectedValueSpare = false
	return profile
}

// parameters is the mission's on-board parameter table. ST[12] needs the width
// of a parameter's value to read a limit or a sampled value, and nothing in the
// message carries it, so the codec asks this.
func parameters(id uint64) (pus.ParameterLayout, error) {
	switch id {
	case paramBattery:
		return pus.ParameterLayout{ValueBytes: 2, MaskBytes: 2}, nil
	default:
		return pus.ParameterLayout{}, fmt.Errorf("unknown parameter %#x", id)
	}
}

// millivolts encodes a voltage the way the battery parameter is defined:
// an unsigned 16-bit count of millivolts.
func millivolts(v float64) []byte {
	raw := make([]byte, 2)
	binary.BigEndian.PutUint16(raw, uint16(v*1000))
	return raw
}

func main() {
	profile := missionProfile()
	if err := profile.Validate(); err != nil {
		log.Fatalf("mission profile: %v", err)
	}

	// One registry per mission. It maps a message type to the codec that
	// handles it, so the receiving end decodes by looking up what arrived
	// rather than by guessing.
	registry, err := pus.NewDefaultRegistry(profile,
		pus.WithParameterResolver(parameters))
	if err != nil {
		log.Fatalf("building the registry: %v", err)
	}

	fmt.Println("--- Ground Side: sending requests ---")
	fmt.Println()

	requests := groundRequests(profile)
	for _, encoded := range requests {
		fmt.Printf("  %d octets on the wire\n", len(encoded))
	}
	fmt.Println()

	// The spacecraft decodes each request and remembers what it accepted.
	fmt.Println("--- Spacecraft Side: decoding requests ---")
	fmt.Println()

	var accepted []spp.PrimaryHeader
	for _, encoded := range requests {
		header := &pus.TCHeader{Profile: profile}
		packet, err := spp.Decode(encoded,
			spp.WithDecodeSecondaryHeader(header),
			spp.WithDecodeErrorControl(),
		)
		if err != nil {
			log.Fatalf("decoding the telecommand: %v", err)
		}

		request, err := registry.DecodeRequest(header.Key(), packet.UserData)
		if err != nil {
			log.Fatalf("decoding %s: %v", header.Key(), err)
		}

		fmt.Printf("  %s accepted from APID %d\n", header.Key(), packet.PrimaryHeader.APID)
		fmt.Print(indent(describe(request)))
		accepted = append(accepted, packet.PrimaryHeader)
	}
	fmt.Println()

	// The spacecraft reports back. The clock is fixed so the output is stable.
	now := time.Date(2026, 3, 14, 9, 26, 53, 0, time.UTC)

	fmt.Println("--- Spacecraft Side: sending reports ---")
	fmt.Println()

	reports := spacecraftReports(profile, accepted[0], now)
	for _, encoded := range reports {
		fmt.Printf("  %d octets on the wire\n", len(encoded))
	}
	fmt.Println()

	fmt.Println("--- Ground Side: decoding reports ---")
	fmt.Println()

	for _, encoded := range reports {
		header := &pus.TMHeader{Profile: profile}
		packet, err := spp.Decode(encoded,
			spp.WithDecodeSecondaryHeader(header),
			spp.WithDecodeErrorControl(),
		)
		if err != nil {
			log.Fatalf("decoding the report: %v", err)
		}

		report, err := registry.DecodeReport(header.Key(), packet.UserData)
		if err != nil {
			log.Fatalf("decoding %s: %v", header.Key(), err)
		}

		fmt.Printf("  %s from APID %d at %s\n",
			header.Key(), packet.PrimaryHeader.APID,
			header.Time.Format("2006-01-02T15:04:05Z"))
		fmt.Print(indent(describe(report)))
	}
}

// groundRequests builds the three telecommands, each a Space Packet with a PUS
// telecommand secondary header.
func groundRequests(profile pus.MissionProfile) [][]byte {
	var out [][]byte

	// TC[3,5]: start generating the housekeeping report periodically.
	enable := &pus.HousekeepingControlRequest{
		Profile:      profile,
		Subtype:      pus.SubtypeEnableHKGeneration,
		StructureIDs: []uint64{hkStructureID},
	}
	out = append(out, telecommand(profile, 0, pus.ServiceHousekeeping,
		pus.SubtypeEnableHKGeneration, mustEncode(enable.Encode())))

	// TC[12,5]: watch the battery voltage and tell us when it leaves its band.
	// A limit check needs both limits at the parameter's own width, which is
	// why the resolver exists.
	monitor := pus.AddPMONDefinitionsRequest{
		Profile: profile,
		Resolve: parameters,
		Definitions: []pus.PMONDefinition{{
			ID:                   pmonBattery,
			MonitoredParameterID: paramBattery,
			MonitoringInterval:   10,
			RepetitionNumber:     3,
			Criteria: pus.CheckCriteria{
				Type: pus.CheckLimit,
				Limit: &pus.LimitCheck{
					LowLimit:              millivolts(26.0),
					LowEventDefinitionID:  eventLowBatt,
					HighLimit:             millivolts(29.5),
					HighEventDefinitionID: 0, // no event on the high side
				},
			},
		}},
	}
	out = append(out, telecommand(profile, 1, pus.ServiceOnBoardMonitoring,
		pus.SubtypeAddPMONDefinitions, mustEncode(monitor.Encode())))

	// TC[11,4]: hold a command on board and release it later. The scheduled
	// activity carries a whole telecommand packet, primary header included,
	// so this is a command wrapped in a command.
	later := telecommand(profile, 2, pus.ServiceHousekeeping,
		pus.SubtypeDisableHKGeneration, mustEncode((&pus.HousekeepingControlRequest{
			Profile:      profile,
			Subtype:      pus.SubtypeDisableHKGeneration,
			StructureIDs: []uint64{hkStructureID},
		}).Encode()))

	schedule := pus.InsertActivitiesRequest{
		Profile:       profile,
		SubScheduleID: 1,
		Activities: []pus.ScheduledActivity{{
			GroupID:     1,
			ReleaseTime: time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC),
			Request:     later,
		}},
	}
	out = append(out, telecommand(profile, 3, pus.ServiceTimeBasedScheduling,
		pus.SubtypeInsertActivities, mustEncode(schedule.Encode())))

	return out
}

// spacecraftReports builds the five reports. The first two verify the
// telecommand named by request; the rest are unprompted.
func spacecraftReports(profile pus.MissionProfile, request spp.PrimaryHeader, now time.Time) [][]byte {
	id := requestID(request)
	var out [][]byte

	// TM[1,1] and TM[1,7]: the two halves of verifying one telecommand. The
	// acceptance report says the packet was well formed and the request was
	// understood; the completion report says it actually ran.
	for _, subtype := range []uint8{pus.SubtypeAcceptSuccess, pus.SubtypeCompleteSuccess} {
		report := &pus.VerificationReport{
			Profile:   profile,
			Subtype:   subtype,
			RequestID: id,
		}
		out = append(out, report_(profile, pus.ServiceRequestVerification, subtype,
			now, mustEncode(report.Encode())))
		now = now.Add(2 * time.Second)
	}

	// TM[3,25]: the periodic report the enable request asked for. The values
	// are laid out exactly as the report structure declares, and this package
	// moves them verbatim because both ends already share that structure.
	housekeeping := &pus.HousekeepingReport{
		Profile:         profile,
		StructureID:     hkStructureID,
		ParameterValues: millivolts(28.1),
	}
	out = append(out, report_(profile, pus.ServiceHousekeeping,
		pus.SubtypeHKParameterReport, now, mustEncode(housekeeping.Encode())))
	now = now.Add(10 * time.Second)

	// TM[12,12]: the battery has drifted below the low limit and stayed there
	// for three consecutive checks, so the checking status changed.
	transition := pus.CheckTransitionReport{
		Profile: profile,
		Resolve: parameters,
		Subtype: pus.SubtypeCheckTransitionReport,
		Transitions: []pus.CheckTransition{{
			PMONID:                 pmonBattery,
			MonitoredParameterID:   paramBattery,
			CheckType:              pus.CheckLimit,
			ParameterValue:         millivolts(25.4),
			LimitCrossed:           millivolts(26.0),
			PreviousCheckingStatus: pus.PMONNominal,
			CurrentCheckingStatus:  pus.PMONBelowLowLimit,
			TransitionTime:         now,
		}},
	}
	out = append(out, report_(profile, pus.ServiceOnBoardMonitoring,
		pus.SubtypeCheckTransitionReport, now, mustEncode(transition.Encode())))

	// TM[5,2]: the event the limit check named. ST[12] detects, ST[05]
	// announces, and the definition ID is the link between them.
	event := &pus.EventReport{
		Profile:           profile,
		Severity:          pus.Severity(pus.SubtypeLowSeverity),
		EventDefinitionID: eventLowBatt,
		AuxiliaryData:     millivolts(25.4),
	}
	out = append(out, report_(profile, pus.ServiceEventReporting,
		pus.SubtypeLowSeverity, now, mustEncode(event.Encode())))

	return out
}

// telecommand wraps a PUS request body in a Space Packet with a TC secondary
// header. AckAcceptance and AckCompletion are what ask for TM[1,1] and TM[1,7];
// a telecommand that sets neither is executed silently.
func telecommand(profile pus.MissionProfile, seq uint16, service, subtype uint8, body []byte) []byte {
	header := profile.NewTCHeader(service, subtype, groundSourceID,
		pus.AckAcceptance|pus.AckCompletion)

	packet, err := spp.NewTCPacket(apidCommanding, body,
		spp.WithSecondaryHeader(header),
		spp.WithSequenceCount(seq),
		spp.WithErrorControl(),
	)
	if err != nil {
		log.Fatalf("building the telecommand: %v", err)
	}
	return mustEncode(packet.Encode())
}

// report_ wraps a PUS report body in a Space Packet with a TM secondary
// header. The time tag lives in that header, not in the report body.
func report_(profile pus.MissionProfile, service, subtype uint8, t time.Time, body []byte) []byte {
	header := profile.NewTMHeader(service, subtype, groundDestID, t)

	packet, err := spp.NewTMPacket(apidPower, body,
		spp.WithSecondaryHeader(header),
		spp.WithErrorControl(),
	)
	if err != nil {
		log.Fatalf("building the report: %v", err)
	}
	return mustEncode(packet.Encode())
}

// requestID names the telecommand a verification report concerns. It is the
// first two words of that telecommand's own primary header, which is why the
// spacecraft has to keep the header around until it has finished reporting.
func requestID(header spp.PrimaryHeader) pus.RequestID {
	return pus.RequestID{
		PacketVersion:       header.Version,
		PacketType:          header.Type,
		SecondaryHeaderFlag: header.SecondaryHeaderFlag,
		APID:                header.APID,
		SequenceFlags:       header.SequenceFlags,
		SequenceCount:       header.SequenceCount,
	}
}

func mustEncode(data []byte, err error) []byte {
	if err != nil {
		log.Fatalf("encoding: %v", err)
	}
	return data
}

// describable is every PUS message that can print itself. Request and Report
// promise only Key and Encode, so the human-readable form is asked for rather
// than assumed.
type describable interface {
	Humanize() string
}

// describe returns the human-readable form of a decoded message, which is
// what you want in front of you when a capture does not look right.
func describe(message any) string {
	printer, ok := message.(describable)
	if !ok {
		return "(no human-readable form)"
	}
	return printer.Humanize()
}

// indent pushes a multi-line description under the line that introduced it.
func indent(text string) string {
	var out strings.Builder
	for line := range strings.SplitSeq(strings.TrimRight(text, "\n"), "\n") {
		out.WriteString("    ")
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String()
}
