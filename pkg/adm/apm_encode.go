package adm

import "github.com/ravisuhag/astro/internal/ndm"

// Encode writes the message in 'keyword = value' notation.
//
// Blocks go out in the order table 3-3 lists them: quaternion, Euler angles,
// angular velocity, spin, inertia, manoeuvres.
func (m *APM) Encode() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	var w ndm.Writer
	if err := m.Header.toNDM().Write(&w, headerSpec("CCSDS_APM_VERS")); err != nil {
		return nil, err
	}

	md := m.Metadata
	w.Comments(md.Comments)
	w.Assign("OBJECT_NAME", md.ObjectName)
	w.Assign("OBJECT_ID", md.ObjectID)
	if md.CenterName != "" {
		w.Assign("CENTER_NAME", md.CenterName)
	}
	w.Assign("TIME_SYSTEM", md.TimeSystem)

	w.Blank()
	w.Comments(m.Comments)
	epoch, err := ndm.FormatEpoch(m.Epoch, epochPrecision(m.Epoch))
	if err != nil {
		return nil, err
	}
	w.Assign("EPOCH", epoch)

	if b := m.Quaternion; b != nil {
		w.Blank()
		openBlock(&w, blockQuaternion, b.Comments)
		writeFrames(&w, b.frames)
		w.Assign("Q1", formatValue(b.Q1))
		w.Assign("Q2", formatValue(b.Q2))
		w.Assign("Q3", formatValue(b.Q3))
		w.Assign("QC", formatValue(b.QC))
		if b.HasDerivative {
			w.AssignUnits("Q1_DOT", formatValue(b.Derivative.Q1), "1/s")
			w.AssignUnits("Q2_DOT", formatValue(b.Derivative.Q2), "1/s")
			w.AssignUnits("Q3_DOT", formatValue(b.Derivative.Q3), "1/s")
			w.AssignUnits("QC_DOT", formatValue(b.Derivative.QC), "1/s")
		}
		closeBlock(&w, blockQuaternion)
	}

	if b := m.Euler; b != nil {
		w.Blank()
		openBlock(&w, blockEuler, b.Comments)
		writeFrames(&w, b.frames)
		w.Assign("EULER_ROT_SEQ", b.RotSeq)
		w.AssignUnits("ANGLE_1", formatValue(b.Angle1), "deg")
		w.AssignUnits("ANGLE_2", formatValue(b.Angle2), "deg")
		w.AssignUnits("ANGLE_3", formatValue(b.Angle3), "deg")
		if b.HasRates {
			w.AssignUnits("ANGLE_1_DOT", formatValue(b.Rate1), "deg/s")
			w.AssignUnits("ANGLE_2_DOT", formatValue(b.Rate2), "deg/s")
			w.AssignUnits("ANGLE_3_DOT", formatValue(b.Rate3), "deg/s")
		}
		closeBlock(&w, blockEuler)
	}

	if b := m.AngVel; b != nil {
		w.Blank()
		openBlock(&w, blockAngVel, b.Comments)
		writeFrames(&w, b.frames)
		w.Assign("ANGVEL_FRAME", b.Frame)
		w.AssignUnits("ANGVEL_X", formatValue(b.X), "deg/s")
		w.AssignUnits("ANGVEL_Y", formatValue(b.Y), "deg/s")
		w.AssignUnits("ANGVEL_Z", formatValue(b.Z), "deg/s")
		closeBlock(&w, blockAngVel)
	}

	if b := m.Spin; b != nil {
		w.Blank()
		openBlock(&w, blockSpin, b.Comments)
		writeFrames(&w, b.frames)
		w.AssignUnits("SPIN_ALPHA", formatValue(b.Alpha), "deg")
		w.AssignUnits("SPIN_DELTA", formatValue(b.Delta), "deg")
		w.AssignUnits("SPIN_ANGLE", formatValue(b.Angle), "deg")
		w.AssignUnits("SPIN_ANGLE_VEL", formatValue(b.AngleVel), "deg/s")
		if b.HasNutation {
			w.AssignUnits("NUTATION", formatValue(b.Nutation), "deg")
			w.AssignUnits("NUTATION_PER", formatValue(b.NutationPeriod), "s")
			w.AssignUnits("NUTATION_PHASE", formatValue(b.NutationPhase), "deg")
		}
		if b.HasMomentum {
			w.AssignUnits("MOMENTUM_ALPHA", formatValue(b.MomentumAlpha), "deg")
			w.AssignUnits("MOMENTUM_DELTA", formatValue(b.MomentumDelta), "deg")
			w.AssignUnits("NUTATION_VEL", formatValue(b.NutationVel), "deg/s")
		}
		closeBlock(&w, blockSpin)
	}

	if b := m.Inertia; b != nil {
		w.Blank()
		openBlock(&w, blockInertia, b.Comments)
		w.Assign("INERTIA_REF_FRAME", b.Frame)
		for _, e := range []struct {
			keyword string
			value   float64
		}{
			{"IXX", b.IXX}, {"IYY", b.IYY}, {"IZZ", b.IZZ},
			{"IXY", b.IXY}, {"IXZ", b.IXZ}, {"IYZ", b.IYZ},
		} {
			w.AssignUnits(e.keyword, formatValue(e.value), "kg*m**2")
		}
		closeBlock(&w, blockInertia)
	}

	for _, man := range m.Maneuvers {
		w.Blank()
		openBlock(&w, blockManeuver, man.Comments)
		start, err := ndm.FormatEpoch(man.EpochStart, epochPrecision(man.EpochStart))
		if err != nil {
			return nil, err
		}
		w.Assign("MAN_EPOCH_START", start)
		w.AssignUnits("MAN_DURATION", formatValue(man.Duration), "s")
		w.Assign("MAN_REF_FRAME", man.RefFrame)
		w.AssignUnits("MAN_TOR_X", formatValue(man.TorqueX), "N*m")
		w.AssignUnits("MAN_TOR_Y", formatValue(man.TorqueY), "N*m")
		w.AssignUnits("MAN_TOR_Z", formatValue(man.TorqueZ), "N*m")
		closeBlock(&w, blockManeuver)
	}
	return w.Bytes(), nil
}

func openBlock(w *ndm.Writer, name string, comments []string) {
	w.Section(name + "_START")
	w.Comments(comments)
}

func closeBlock(w *ndm.Writer, name string) {
	w.Section(name + "_STOP")
}

// writeFrames writes the pair of frames a transformation block carries.
func writeFrames(w *ndm.Writer, f frames) {
	w.Assign("REF_FRAME_A", f.FrameA)
	w.Assign("REF_FRAME_B", f.FrameB)
}
