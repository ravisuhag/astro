package sle

import "fmt"

// Delivery modes.
//
// A service instance's delivery mode is agreed before the session, in the
// service agreement, and reported by GET-PARAMETER. It is not something the
// protocol negotiates. What it changes is how the provider behaves when data
// arrives faster than the user takes it.
//
// The three return modes, from CCSDS 911.1-B-5 §1.2.2 and the state table
// rows 10 to 14 of §4.2.2:
//
//	timely online     The provider may throw data away to stay current. When
//	                  its buffer fills and the connection is congested, it
//	                  discards and says so with a sync notification. A user
//	                  watching a pass live wants this: late frames are worth
//	                  less than current ones.
//	complete online   The provider delivers everything, in order, and lets
//	                  the connection push back when the user is slow. Nothing
//	                  is discarded. A user recording a pass wants this.
//	offline           The provider reads from a store rather than from a live
//	                  channel, so START's time range names a past window.
//
// The forward modes are simpler: FCLTU is online or offline, and offline
// forward service is not something this library models beyond the enum value.
//
// **What this package implements.** Delivery mode here is state, not an
// engine. The machines in service.go hold one PDU at a time and never queue,
// so the buffering that distinguishes the modes lives in the caller. What the
// library does is:
//
//   - refuse operations the mode forbids, as the state tables require;
//   - tell the caller which behavior the mode asks of it, through the
//     predicate methods below;
//   - carry the discard notification when a timely-online provider drops
//     data, so a user can see the gap.
//
// What the library does not do: size a buffer, decide when it is full, run a
// release timer, or discard anything itself. Those are the caller's, and the
// predicates are how it knows which to do.

// DeliveryMode says how a service instance delivers data.
type DeliveryMode int

const (
	DeliveryReturnTimelyOnline   DeliveryMode = 0
	DeliveryReturnCompleteOnline DeliveryMode = 1
	DeliveryReturnOffline        DeliveryMode = 2
	DeliveryForwardOnline        DeliveryMode = 3
	DeliveryForwardOffline       DeliveryMode = 4
)

// String names the delivery mode.
func (d DeliveryMode) String() string {
	switch d {
	case DeliveryReturnTimelyOnline:
		return "return timely online"
	case DeliveryReturnCompleteOnline:
		return "return complete online"
	case DeliveryReturnOffline:
		return "return offline"
	case DeliveryForwardOnline:
		return "forward online"
	case DeliveryForwardOffline:
		return "forward offline"
	default:
		return fmt.Sprintf("mode(%d)", int(d))
	}
}

// Valid reports whether the mode is one of the five the standard defines.
func (d DeliveryMode) Valid() bool {
	return d >= DeliveryReturnTimelyOnline && d <= DeliveryForwardOffline
}

// IsReturn reports whether the mode belongs to a return service.
func (d DeliveryMode) IsReturn() bool {
	switch d {
	case DeliveryReturnTimelyOnline, DeliveryReturnCompleteOnline, DeliveryReturnOffline:
		return true
	default:
		return false
	}
}

// IsForward reports whether the mode belongs to a forward service.
func (d DeliveryMode) IsForward() bool {
	switch d {
	case DeliveryForwardOnline, DeliveryForwardOffline:
		return true
	default:
		return false
	}
}

// IsOnline reports whether the mode reads from a live channel rather than a
// store.
func (d DeliveryMode) IsOnline() bool {
	switch d {
	case DeliveryReturnTimelyOnline, DeliveryReturnCompleteOnline, DeliveryForwardOnline:
		return true
	default:
		return false
	}
}

// AllowsDiscard reports whether the provider may throw data away to keep up.
// Only timely online may: state table row 14 of CCSDS 911.1-B-5 §4.2.2 is the
// only cell that sends a 'data discarded' sync notification.
func (d DeliveryMode) AllowsDiscard() bool {
	return d == DeliveryReturnTimelyOnline
}

// RequiresBackpressure reports whether the caller must slow down rather than
// let data be dropped. Complete online and offline both promise every frame,
// so a slow caller has to become the brake.
func (d DeliveryMode) RequiresBackpressure() bool {
	switch d {
	case DeliveryReturnCompleteOnline, DeliveryReturnOffline:
		return true
	default:
		return false
	}
}

// AllowsPastStartTime reports whether START may name a time range that has
// already happened. Only offline delivery reads from a store, so only offline
// can be asked for yesterday.
func (d DeliveryMode) AllowsPastStartTime() bool {
	switch d {
	case DeliveryReturnOffline, DeliveryForwardOffline:
		return true
	default:
		return false
	}
}

// AllowsPeriodicStatusReport reports whether SCHEDULE-STATUS-REPORT may ask
// for periodic reports. Offline delivery has no live channel to report on, so
// the answer there is the 'not supported in this delivery mode' diagnostic.
func (d DeliveryMode) AllowsPeriodicStatusReport() bool {
	return d.IsOnline()
}
