package csts

import (
	"fmt"
	"strings"
)

// The object identifier tree of CCSDS 921.1-B-2 annex F3.1.
//
// CSTS identifies things by OID where SLE used a wire tag and a context. A
// procedure type, a service type, a parameter, an event, a directive: each is
// an OID rooted under the CCSDS cross support arc, and the allocation is under
// CCSDS control rather than fixed by the document.
//
// That is the deeper change between the two families. An SLE PDU's meaning
// depends on which service instance carried it, which is why pkg/sle needs a
// --service argument it cannot infer. A CSTS PDU names its procedure in the
// message, so the same octets mean the same thing wherever they arrive.

// OID is an object identifier, the arc numbers in order.
type OID []uint32

// String renders an OID in dotted form.
func (o OID) String() string {
	parts := make([]string, len(o))
	for i, arc := range o {
		parts[i] = fmt.Sprint(arc)
	}
	return strings.Join(parts, ".")
}

// Equal reports whether two identifiers are the same.
func (o OID) Equal(other OID) bool {
	if len(o) != len(other) {
		return false
	}
	for i := range o {
		if o[i] != other[i] {
			return false
		}
	}
	return true
}

// child returns an OID one arc below this one.
func (o OID) child(arc uint32) OID {
	out := make(OID, len(o)+1)
	copy(out, o)
	out[len(o)] = arc
	return out
}

// The roots of annex F3.1. The css arc is written out in full because it is
// the one place the tree is not defined relative to something above it.
var (
	// OIDCSS is {1 3 112 4 4}: iso, identified-organization,
	// standards-producing-organization, ccsds, css.
	OIDCSS = OID{1, 3, 112, 4, 4}

	// OIDCSTS is the cross support transfer service arc, {css 1}.
	OIDCSTS = OIDCSS.child(1)
	// OIDCrossSupportResources is {css 2}.
	OIDCrossSupportResources = OIDCSS.child(2)

	// OIDFramework is {csts 1}, everything this package implements.
	OIDFramework = OIDCSTS.child(1)
	// OIDServices is {csts 2}, where a service type such as the Monitored
	// Data CSTS is registered.
	OIDServices = OIDCSTS.child(2)

	// OIDModules is {framework 1}, OIDOperations {framework 2},
	// OIDProcedures {framework 3}.
	OIDModules    = OIDFramework.child(1)
	OIDOperations = OIDFramework.child(2)
	OIDProcedures = OIDFramework.child(3)
	// OIDFwProceduresFunctionalities is {framework 4}.
	OIDFwProceduresFunctionalities = OIDFramework.child(4)
)

// The framework operation identifiers, {operations n}.
//
// These name an operation message rather than tagging it: the wire tag on a
// framework PDU is the CHOICE tag of annex F3.15, and these OIDs are what a
// service specification refers to when it extends an operation.
var (
	OIDBindInvocation              = OIDOperations.child(1)
	OIDBindReturn                  = OIDOperations.child(2)
	OIDUnbindInvocation            = OIDOperations.child(3)
	OIDUnbindReturn                = OIDOperations.child(4)
	OIDPeerAbortInvocation         = OIDOperations.child(5)
	OIDStartInvocation             = OIDOperations.child(6)
	OIDStartReturn                 = OIDOperations.child(7)
	OIDStopInvocation              = OIDOperations.child(8)
	OIDStopReturn                  = OIDOperations.child(9)
	OIDExecuteDirectiveInvocation  = OIDOperations.child(10)
	OIDExecuteDirectiveAcknowledge = OIDOperations.child(11)
	OIDExecuteDirectiveReturn      = OIDOperations.child(12)
	OIDGetInvocation               = OIDOperations.child(13)
	OIDGetReturn                   = OIDOperations.child(14)
	OIDNotifyInvocation            = OIDOperations.child(15)
	OIDTransferDataInvocation      = OIDOperations.child(16)
	OIDProcessDataInvocation       = OIDOperations.child(17)
	OIDProcessDataReturn           = OIDOperations.child(18)
)

// The framework procedure types, {procedures n}, and the two derived
// procedures the framework itself defines.
//
// A ProcedureName carries one of these, which is how a PDU says what it is
// part of. Clause 4.1 lists the twelve procedures; seven have their own arc
// here and the rest are derived from one of those, so Buffered Data Processing
// sits under Data Processing rather than beside it.
var (
	OIDAssociationControl     = OIDProcedures.child(1)
	OIDUnbufferedDataDelivery = OIDProcedures.child(2)
	OIDBufferedDataDelivery   = OIDProcedures.child(3)
	OIDDataProcessing         = OIDProcedures.child(4)
	OIDInformationQuery       = OIDProcedures.child(5)
	OIDNotification           = OIDProcedures.child(6)
	OIDThrowEvent             = OIDProcedures.child(7)

	// OIDCyclicReport is derived from Unbuffered Data Delivery.
	OIDCyclicReport = OIDUnbufferedDataDelivery.child(1).child(1)
	// OIDBufferedDataProcessing and OIDSequenceControlledDataProcessing are
	// derived from Data Processing.
	OIDBufferedDataProcessing           = OIDDataProcessing.child(1).child(1)
	OIDSequenceControlledDataProcessing = OIDDataProcessing.child(1).child(2)
)

// procedureNames maps a procedure type to the name the document gives it, for
// Humanize. An OID that is not one of the framework's own comes back empty:
// a service specification may define its own, and this package has no registry
// to look it up in.
var procedureNames = []struct {
	oid  OID
	name string
}{
	{OIDAssociationControl, "Association Control"},
	{OIDUnbufferedDataDelivery, "Unbuffered Data Delivery"},
	{OIDBufferedDataDelivery, "Buffered Data Delivery"},
	{OIDDataProcessing, "Data Processing"},
	{OIDInformationQuery, "Information Query"},
	{OIDNotification, "Notification"},
	{OIDThrowEvent, "Throw Event"},
	{OIDCyclicReport, "Cyclic Report"},
	{OIDBufferedDataProcessing, "Buffered Data Processing"},
	{OIDSequenceControlledDataProcessing, "Sequence-Controlled Data Processing"},
}

// ProcedureTypeName returns the framework's name for a procedure type, or an
// empty string for one it does not define.
func ProcedureTypeName(oid OID) string {
	for _, p := range procedureNames {
		if p.oid.Equal(oid) {
			return p.name
		}
	}
	return ""
}
