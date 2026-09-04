package csts

import (
	"fmt"
	"strings"
)

// Humanize returns a human-readable summary of the PDU.
func (p *PDU) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "CCSDS CSTS framework PDU: %s\n", p.Type)

	if header, ok := p.Header(); ok {
		fmt.Fprintf(&sb, "  Invoke ID ....... %d\n", header.InvokeID)
		sb.WriteString(procedureLine(header.Procedure))
		fmt.Fprintf(&sb, "  Credentials ..... %s\n", credentialsLine(header.InvokerCredentials))
	}

	if header, ok := p.ReturnHeader(); ok {
		fmt.Fprintf(&sb, "  Invoke ID ....... %d\n", header.InvokeID)
		if header.Positive {
			sb.WriteString("  Result .......... positive\n")
		} else {
			fmt.Fprintf(&sb, "  Result .......... negative, %s\n", header.Diagnostic)
		}
		fmt.Fprintf(&sb, "  Credentials ..... %s\n", credentialsLine(header.PerformerCredentials))
	}

	switch {
	case p.Bind != nil:
		fmt.Fprintf(&sb, "  Initiator ....... %s\n", p.Bind.InitiatorIdentifier)
		fmt.Fprintf(&sb, "  Responder port .. %s\n", p.Bind.ResponderPortIdentifier)
		fmt.Fprintf(&sb, "  Service type .... %s\n", p.Bind.ServiceType)
		fmt.Fprintf(&sb, "  Version ......... %d\n", p.Bind.VersionNumber)
		fmt.Fprintf(&sb, "  Service instance  %s / %s, instance %d\n",
			p.Bind.ServiceInstance.SpacecraftID, p.Bind.ServiceInstance.FacilityID,
			p.Bind.ServiceInstance.InstanceNumber)

	case p.BindReturn != nil:
		fmt.Fprintf(&sb, "  Responder ....... %s\n", p.BindReturn.ResponderIdentifier)

	case p.PeerAbort != nil:
		// A peer abort has no header at all, which clause 3.3.1.1 states.
		fmt.Fprintf(&sb, "  Diagnostic ...... %s (%d)\n",
			p.PeerAbort.Diagnostic, uint8(p.PeerAbort.Diagnostic))
		fmt.Fprintf(&sb, "  Allocated by .... %s\n", p.PeerAbort.Diagnostic.Origin())

	case p.TransferData != nil:
		fmt.Fprintf(&sb, "  Sequence counter  %d\n", p.TransferData.SequenceCounter)
		fmt.Fprintf(&sb, "  Data ............ %d octet(s), encoded\n", len(p.TransferData.Data))

	case p.ProcessData != nil:
		fmt.Fprintf(&sb, "  Data unit ID .... %d\n", p.ProcessData.DataUnitID)
		fmt.Fprintf(&sb, "  Data ............ %d octet(s), encoded\n", len(p.ProcessData.Data))

	case p.Notify != nil:
		fmt.Fprintf(&sb, "  Event ........... %d octet(s), encoded name\n", len(p.Notify.EventName))

	case p.Get != nil:
		fmt.Fprintf(&sb, "  Parameter list .. %d octet(s), encoded\n", len(p.Get.ListOfParameters))
	}

	return sb.String()
}

// procedureLine renders the procedure a message belongs to, naming the
// framework's own procedure types and reporting anything else by its OID.
func procedureLine(p ProcedureName) string {
	name := ProcedureTypeName(p.Type)
	if name == "" {
		// A service specification may define its own procedure types, and
		// this package has no registry to look one up in.
		name = p.Type.String()
	}

	if p.Role == RoleSecondary {
		return fmt.Sprintf("  Procedure ....... %s, secondary instance %d\n", name, p.Instance)
	}
	return fmt.Sprintf("  Procedure ....... %s, %s\n", name, p.Role)
}

func credentialsLine(c Credentials) string {
	if !c.Used {
		return "unused"
	}
	return fmt.Sprintf("%d octet(s)", len(c.Value))
}
