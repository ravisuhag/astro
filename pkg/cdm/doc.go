// Package cdm implements the CCSDS Conjunction Data Message, CCSDS 508.0-B-1.
//
// A CDM is a warning. It says that two objects in orbit are going to pass
// close to each other, when, how close, and how well each object's position is
// known. One message describes one conjunction between exactly two objects.
//
// It is the message an operator acts on. The other navigation messages
// describe a spacecraft; this one describes a risk, and the decision it feeds
// is whether to spend propellant moving out of the way.
//
// # What this package does
//
// It reads and writes the message, in 'keyword = value' notation. It does no
// conjunction analysis: nothing here propagates either object, recomputes a
// miss distance, or calculates a collision probability. Those are the
// originator's work, and clause 1.1 makes the message a report of them rather
// than an input to them.
//
//	message, err := cdm.Decode(data)
//	tca, _ := message.TCA()
//	miss, _ := message.MissDistance()
//	for i, object := range message.Objects {
//	    fmt.Println(i+1, object.Name(), object.Designator())
//	}
//
// # Both objects, or neither
//
// A CDM always describes two objects, and the OBJECT keyword is what switches
// between them: everything after OBJECT = OBJECT1 belongs to the first and
// everything after OBJECT = OBJECT2 to the second. A message with one object
// section is not a conjunction, and is refused.
//
// The two sections are not symmetric in practice. One object is usually the
// operator's spacecraft and the other is usually debris, and the debris
// section will have a much larger covariance. Nothing in the format says which
// is which — MESSAGE_FOR names the spacecraft the message was sent to, and
// even that is optional.
//
// # The covariance is nine by nine, not six
//
// The obligatory covariance is the 21 lower triangular elements of a 6x6 in
// the RTN frame. Three optional rows extend it to 9x9, adding the
// uncertainties in drag, solar radiation pressure and thrust. Covariance
// returns the full 9x9 with the rows that were absent left at zero, and
// CovarianceOrder reports how many were present, because a zero row and an
// absent one mean different things to anyone computing a probability.
package cdm
