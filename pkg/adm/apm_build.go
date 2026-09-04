package adm

// buildAPMBlock turns one block's collected keywords into its typed form.
//
// Table 3-3 heads every block with "all mandatory elements are to be provided
// if the block is present", so a block that opened must be complete. That is
// what the required flags below encode.
func buildAPMBlock(m *APM, name string, fields map[string]string, comments []string) error {
	f := newFieldSet(fields)

	switch name {
	case blockQuaternion:
		block := &QuaternionBlock{
			Comments: comments,
			frames:   frames{FrameA: f.require("REF_FRAME_A"), FrameB: f.require("REF_FRAME_B")},
			Quaternion: Quaternion{
				Q1: f.num("Q1", true), Q2: f.num("Q2", true),
				Q3: f.num("Q3", true), QC: f.num("QC", true),
			},
		}
		// The derivatives are optional as a group; any one of them means the
		// block carried them.
		if f.has("Q1_DOT") || f.has("Q2_DOT") || f.has("Q3_DOT") || f.has("QC_DOT") {
			block.HasDerivative = true
			block.Derivative = Quaternion{
				Q1: f.num("Q1_DOT", false), Q2: f.num("Q2_DOT", false),
				Q3: f.num("Q3_DOT", false), QC: f.num("QC_DOT", false),
			}
		}
		m.Quaternion = block

	case blockEuler:
		block := &EulerBlock{
			Comments: comments,
			frames:   frames{FrameA: f.require("REF_FRAME_A"), FrameB: f.require("REF_FRAME_B")},
			EulerAngles: EulerAngles{
				RotSeq: f.require("EULER_ROT_SEQ"),
				Angle1: f.num("ANGLE_1", true),
				Angle2: f.num("ANGLE_2", true),
				Angle3: f.num("ANGLE_3", true),
			},
		}
		if f.has("ANGLE_1_DOT") || f.has("ANGLE_2_DOT") || f.has("ANGLE_3_DOT") {
			block.HasRates = true
			block.Rate1 = f.num("ANGLE_1_DOT", false)
			block.Rate2 = f.num("ANGLE_2_DOT", false)
			block.Rate3 = f.num("ANGLE_3_DOT", false)
		}
		m.Euler = block

	case blockAngVel:
		m.AngVel = &AngVelBlock{
			Comments: comments,
			frames:   frames{FrameA: f.require("REF_FRAME_A"), FrameB: f.require("REF_FRAME_B")},
			AngularVelocity: AngularVelocity{
				Frame: f.require("ANGVEL_FRAME"),
				X:     f.num("ANGVEL_X", true),
				Y:     f.num("ANGVEL_Y", true),
				Z:     f.num("ANGVEL_Z", true),
			},
		}

	case blockSpin:
		spin := Spin{
			Alpha:    f.num("SPIN_ALPHA", true),
			Delta:    f.num("SPIN_DELTA", true),
			Angle:    f.num("SPIN_ANGLE", true),
			AngleVel: f.num("SPIN_ANGLE_VEL", true),
		}
		// The nutation and momentum groups are each conditional as a whole.
		if f.has("NUTATION") || f.has("NUTATION_PER") || f.has("NUTATION_PHASE") {
			spin.HasNutation = true
			spin.Nutation = f.num("NUTATION", false)
			spin.NutationPeriod = f.num("NUTATION_PER", false)
			spin.NutationPhase = f.num("NUTATION_PHASE", false)
		}
		if f.has("MOMENTUM_ALPHA") || f.has("MOMENTUM_DELTA") || f.has("NUTATION_VEL") {
			spin.HasMomentum = true
			spin.MomentumAlpha = f.num("MOMENTUM_ALPHA", false)
			spin.MomentumDelta = f.num("MOMENTUM_DELTA", false)
			spin.NutationVel = f.num("NUTATION_VEL", false)
		}
		m.Spin = &SpinBlock{
			Comments: comments,
			frames:   frames{FrameA: f.require("REF_FRAME_A"), FrameB: f.require("REF_FRAME_B")},
			Spin:     spin,
		}

	case blockInertia:
		m.Inertia = &InertiaBlock{
			Comments: comments,
			Inertia: Inertia{
				Frame: f.require("INERTIA_REF_FRAME"),
				IXX:   f.num("IXX", true), IYY: f.num("IYY", true), IZZ: f.num("IZZ", true),
				IXY: f.num("IXY", true), IXZ: f.num("IXZ", true), IYZ: f.num("IYZ", true),
			},
		}

	case blockManeuver:
		// Clause 3.2.4 repeats the whole block for each manoeuvre, so a second
		// MAN_START appends rather than replacing.
		m.Maneuvers = append(m.Maneuvers, Maneuver{
			Comments:   comments,
			EpochStart: f.epoch("MAN_EPOCH_START", true),
			Duration:   f.num("MAN_DURATION", true),
			RefFrame:   f.require("MAN_REF_FRAME"),
			TorqueX:    f.num("MAN_TOR_X", true),
			TorqueY:    f.num("MAN_TOR_Y", true),
			TorqueZ:    f.num("MAN_TOR_Z", true),
		})
	}
	return f.err
}
