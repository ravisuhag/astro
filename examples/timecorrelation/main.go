// Example: Putting a real timestamp on telemetry
//
// A spacecraft does not know what time it is. It counts ticks of its own
// oscillator, and that count drifts against UTC. Telemetry is stamped with the
// count, so before any of it can be compared with anything on the ground, the
// count has to be turned into a time.
//
// That job is time correlation, and it is two separate problems:
//
//	Formats:
//	  1. Write an instant as a CCSDS time code and read it back
//	  2. Choose between CUC, CDS, CCS and ASCII, and see what each costs
//
//	Correlation:
//	  1. Collect pairs of on-board count and known ground time
//	  2. Fit an offset and a rate from them
//	  3. Apply the fit to every other timestamp in the pass
//
// The second part is where the errors live. A perfectly decoded time code
// still reads the wrong instant if the clock model is wrong.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/ravisuhag/astro/pkg/tcf"
)

// missionEpoch is what this spacecraft's clock counts from. Choosing an epoch
// near launch keeps the counter small, and makes the code Level 2: purely
// arithmetic, with no leap seconds applied in either direction.
var missionEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// The on-board clock's real behaviour, which the ground does not know and has
// to work out. A 12 ppm fast oscillator that was set 1.4 seconds late.
const (
	clockRate   = 1.000012 // on-board seconds per real second
	clockOffset = -1.4     // on-board seconds behind at the epoch
)

func main() {
	fmt.Println("--- The four formats, same instant ---")
	fmt.Println()
	formats(time.Date(2026, 4, 17, 8, 30, 15, 250_000_000, time.UTC))

	fmt.Println("--- Correlating the on-board clock ---")
	fmt.Println()
	correlate()
}

// formats writes one instant every way CCSDS 301.0 allows, and reads each
// back. The point is the trade: width against resolution against whether a
// human can read it off a hex dump.
func formats(instant time.Time) {
	fmt.Printf("  The instant .......... %s\n", instant.Format(time.RFC3339Nano))
	fmt.Println()

	// CUC Level 1: a binary counter of TAI seconds since 1958. Two fine
	// octets give about 15 microseconds.
	cuc1, err := tcf.NewCUC(instant,
		tcf.WithCUCCoarseBytes(4), tcf.WithCUCFineBytes(2))
	if err != nil {
		log.Fatalf("encoding CUC level 1: %v", err)
	}
	show("CUC level 1", must(cuc1.Encode()))
	fmt.Printf("      coarse %d s, fine %d, reads back %s\n",
		cuc1.CoarseTime, cuc1.FineTime, cuc1.Time().Format(time.RFC3339Nano))

	// The leap-second table is the reason the coarse count is not simply the
	// UTC seconds since 1958. A Level 1 code counts TAI.
	offset := tcf.TAIUTCOffsetAt(instant)
	fmt.Printf("      TAI is %d s ahead of UTC at this instant\n", offset)
	fmt.Println()

	// CUC Level 2: the same counter against a mission epoch. Smaller numbers,
	// no leap-second arithmetic, and meaningless to anyone without the epoch.
	cuc2, err := tcf.NewCUC(instant,
		tcf.WithCUCEpoch(missionEpoch),
		tcf.WithCUCCoarseBytes(4), tcf.WithCUCFineBytes(2))
	if err != nil {
		log.Fatalf("encoding CUC level 2: %v", err)
	}
	show("CUC level 2", must(cuc2.Encode()))
	fmt.Printf("      coarse %d s since the mission epoch\n", cuc2.CoarseTime)
	fmt.Println()

	// CDS: day count, milliseconds of day, and optional sub-millisecond.
	// Segmented, so a human can read the day off a dump.
	cds, err := tcf.NewCDS(instant,
		tcf.WithCDSDayBytes(2), tcf.WithCDSSubmsBytes(2))
	if err != nil {
		log.Fatalf("encoding CDS: %v", err)
	}
	show("CDS", must(cds.Encode()))
	fmt.Printf("      day %d, %d ms of day, %d us\n",
		cds.Day, cds.Milliseconds, cds.Submilliseconds)
	fmt.Println()

	// CCS: BCD calendar fields. The widest and the easiest to read.
	ccs, err := tcf.NewCCS(instant, tcf.WithCCSMonthDay(), tcf.WithCCSSubSecBytes(2))
	if err != nil {
		log.Fatalf("encoding CCS: %v", err)
	}
	show("CCS", must(ccs.Encode()))
	fmt.Printf("      %04d-%02d-%02d %02d:%02d:%02d\n",
		ccs.Year, ccs.Month, ccs.DayOfMonth, ccs.Hour, ccs.Minute, ccs.Second)
	fmt.Println()

	// ASCII: text. Never on a space link, often in a file or a log.
	ascii, err := tcf.NewASCIITime(tcf.ASCIITypeA, tcf.WithASCIIPrecision(3))
	if err != nil {
		log.Fatalf("building the ASCII codec: %v", err)
	}
	text, err := ascii.Encode(instant)
	if err != nil {
		log.Fatalf("encoding ASCII: %v", err)
	}
	fmt.Printf("  %-20s %2d octets  %s\n", "ASCII type A", len(text), text)
	fmt.Println()
}

// correlate does the real work. The ground collects pairs of "the spacecraft
// said it was T" and "we know it was actually U", fits a straight line through
// them, and uses that line on everything else.
func correlate() {
	// Three correlation points during a pass. Each one is a packet whose
	// on-board time field the ground can pair with an independent measurement
	// of when the event really happened: a ranging solution, a GPS-stamped
	// pulse, or the station's own receive time less the light travel.
	trueTimes := []time.Time{
		time.Date(2026, 4, 17, 8, 30, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 8, 35, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 8, 40, 0, 0, time.UTC),
	}

	fmt.Println("  Correlation points collected during the pass:")
	fmt.Println()
	fmt.Printf("    %-22s %-22s %s\n", "on-board says", "actually was", "error")

	var points []correlationPoint
	for _, actual := range trueTimes {
		// What the spacecraft stamps: its own drifting count, written as a
		// CUC level 2 code, exactly as it would appear in a packet.
		encoded := onboardTimestamp(actual)

		// The ground decodes it. This is a perfect decode of a wrong time.
		code, err := tcf.DecodeCUCTField(encoded, 4, 2, missionEpoch)
		if err != nil {
			log.Fatalf("decoding the on-board time: %v", err)
		}
		claimed := code.Time()

		fmt.Printf("    %-22s %-22s %+.3f s\n",
			claimed.Format("15:04:05.000"),
			actual.Format("15:04:05.000"),
			claimed.Sub(actual).Seconds())

		points = append(points, correlationPoint{
			onboard: claimed.Sub(missionEpoch).Seconds(),
			ground:  actual.Sub(missionEpoch).Seconds(),
		})
	}
	fmt.Println()

	// Two points give an offset and a rate. More points and a least-squares
	// fit is what a real system does, because each measurement has noise.
	rate, offset := fit(points)
	fmt.Printf("  Fitted clock model:\n")
	fmt.Printf("    rate ..... %.9f ground seconds per on-board second\n", rate)
	fmt.Printf("    offset ... %+.6f s at the mission epoch\n", offset)
	fmt.Printf("    drift .... %+.1f ppm\n", (rate-1)*1e6)
	fmt.Println()

	// Now apply it to a timestamp nobody measured independently: a science
	// observation from the middle of the pass.
	observation := time.Date(2026, 4, 17, 8, 37, 30, 0, time.UTC)
	encoded := onboardTimestamp(observation)

	code, err := tcf.DecodeCUCTField(encoded, 4, 2, missionEpoch)
	if err != nil {
		log.Fatalf("decoding the observation time: %v", err)
	}
	claimed := code.Time()
	corrected := apply(claimed, rate, offset)

	fmt.Println("  A science observation, no correlation point of its own:")
	fmt.Println()
	fmt.Printf("    on-board time ..... %s\n", claimed.Format("15:04:05.000000"))
	fmt.Printf("    corrected ......... %s\n", corrected.Format("15:04:05.000000"))
	fmt.Printf("    truth ............. %s\n", observation.Format("15:04:05.000000"))
	fmt.Println()
	fmt.Printf("    error uncorrected . %+.6f s\n", claimed.Sub(observation).Seconds())
	fmt.Printf("    error corrected ... %+.6f s\n", corrected.Sub(observation).Seconds())
	fmt.Println()
	fmt.Println("  The residual is the time code's own resolution, not the clock.")
	fmt.Printf("  Two fine octets quantise to 2^-16 s, about %.1f us.\n", 1e6/65536)
}

// correlationPoint is one pair: what the clock said, and when it really was.
// Both are seconds since the mission epoch, which keeps the arithmetic in a
// range where float64 has plenty of precision left.
type correlationPoint struct {
	onboard float64
	ground  float64
}

// fit returns the rate and offset of the straight line through the points,
// by ordinary least squares. Two points give an exact answer; more points
// average out the measurement noise.
func fit(points []correlationPoint) (rate, offset float64) {
	n := float64(len(points))
	var sumX, sumY, sumXY, sumXX float64
	for _, p := range points {
		sumX += p.onboard
		sumY += p.ground
		sumXY += p.onboard * p.ground
		sumXX += p.onboard * p.onboard
	}

	denominator := n*sumXX - sumX*sumX
	if denominator == 0 {
		// Every point at the same on-board time: no rate is observable, so
		// take the mean offset and assume the clock runs true.
		return 1, (sumY - sumX) / n
	}

	rate = (n*sumXY - sumX*sumY) / denominator
	offset = (sumY - rate*sumX) / n
	return rate, offset
}

// apply converts an on-board instant to a ground one using the fitted model.
func apply(claimed time.Time, rate, offset float64) time.Time {
	onboard := claimed.Sub(missionEpoch).Seconds()
	ground := rate*onboard + offset
	return missionEpoch.Add(time.Duration(ground * float64(time.Second)))
}

// onboardTimestamp is the spacecraft's side: take the real instant, apply the
// clock's real error, and write the result as a CUC T-field. A packet carries
// the T-field alone, because the format is agreed in the mission database
// rather than sent with every timestamp.
func onboardTimestamp(actual time.Time) []byte {
	elapsed := actual.Sub(missionEpoch).Seconds()
	onboard := (elapsed - clockOffset) / clockRate

	code, err := tcf.NewCUC(missionEpoch.Add(time.Duration(onboard*float64(time.Second))),
		tcf.WithCUCEpoch(missionEpoch),
		tcf.WithCUCCoarseBytes(4), tcf.WithCUCFineBytes(2))
	if err != nil {
		log.Fatalf("stamping the packet: %v", err)
	}
	encoded, err := code.EncodeTField()
	if err != nil {
		log.Fatalf("encoding the T-field: %v", err)
	}
	return encoded
}

func show(name string, encoded []byte) {
	fmt.Printf("  %-20s %2d octets  % x\n", name, len(encoded), encoded)
}

func must(encoded []byte, err error) []byte {
	if err != nil {
		log.Fatalf("encoding a time code: %v", err)
	}
	return encoded
}
