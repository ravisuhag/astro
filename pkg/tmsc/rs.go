package tmsc

// Reed-Solomon codec for CCSDS TM Synchronization and Channel Coding
// per CCSDS 131.0-B-4.
//
// Supports (255,223) with 32 parity symbols (corrects up to 16 errors)
// and (255,239) with 16 parity symbols (corrects up to 8 errors).
//
// Field: GF(2^8) with primitive polynomial 0x187.
// First consecutive root (FCR): 112.
// Generator: g(x) = ∏(x - α^(112+i)) for i = 0..nroots-1.

const (
	rsNN  = 255 // codeword length
	rsFCR = 112 // first consecutive root
)

// RSCodec holds precomputed state for a CCSDS Reed-Solomon code.
type RSCodec struct {
	nroots int    // number of parity symbols
	gen    []byte // generator polynomial coefficients (nroots+1 entries, monic)
}

// NewRS255_223 returns an RSCodec for CCSDS (255,223) with 32 parity symbols.
// This code can correct up to 16 symbol errors per codeword.
func NewRS255_223() *RSCodec {
	return newRSCodec(32)
}

// NewRS255_239 returns an RSCodec for CCSDS (255,239) with 16 parity symbols.
// This code can correct up to 8 symbol errors per codeword.
func NewRS255_239() *RSCodec {
	return newRSCodec(16)
}

func newRSCodec(nroots int) *RSCodec {
	// Build generator polynomial:
	// g(x) = (x - α^FCR)(x - α^(FCR+1))...(x - α^(FCR+nroots-1))
	gen := make([]byte, nroots+1)
	gen[0] = 1

	for i := range nroots {
		root := gfPow(rsFCR + i)
		// Multiply gen by (x - root) = (x + root) in GF(2^8)
		for j := i + 1; j > 0; j-- {
			gen[j] = gen[j-1] ^ gfMul(gen[j], root)
		}
		gen[0] = gfMul(gen[0], root)
	}

	return &RSCodec{nroots: nroots, gen: gen}
}

// NRoots returns the number of parity symbols.
func (rs *RSCodec) NRoots() int { return rs.nroots }

// DataLen returns the data length per codeword (255 - nroots).
func (rs *RSCodec) DataLen() int { return rsNN - rs.nroots }

// Encode appends nroots parity symbols to data and returns a 255-byte codeword.
// The input must be exactly DataLen() bytes. The input slice is not modified.
func (rs *RSCodec) Encode(data []byte) ([]byte, error) {
	k := rs.DataLen()
	if len(data) != k {
		return nil, ErrInvalidDataLength
	}

	codeword := make([]byte, rsNN)
	copy(codeword, data)

	// Systematic encoding: compute remainder of data * x^nroots / g(x)
	parity := make([]byte, rs.nroots)
	for i := range k {
		feedback := data[i] ^ parity[0]
		if feedback != 0 {
			for j := range rs.nroots - 1 {
				parity[j] = parity[j+1] ^ gfMul(feedback, rs.gen[rs.nroots-1-j])
			}
			parity[rs.nroots-1] = gfMul(feedback, rs.gen[0])
		} else {
			copy(parity, parity[1:])
			parity[rs.nroots-1] = 0
		}
	}

	copy(codeword[k:], parity)
	return codeword, nil
}

// Decode corrects errors in a 255-byte codeword and returns the corrected
// data (first DataLen() bytes), the number of corrected symbol errors,
// and any error. Returns ErrUncorrectable if errors exceed correction capability.
// The input slice is not modified.
func (rs *RSCodec) Decode(codeword []byte) ([]byte, int, error) {
	if len(codeword) != rsNN {
		return nil, 0, ErrInvalidDataLength
	}

	work := make([]byte, rsNN)
	copy(work, codeword)

	// Step 1: Compute syndromes S_i = R(α^(FCR+i))
	syndromes := make([]byte, rs.nroots)
	allZero := true
	for i := range rs.nroots {
		s := byte(0)
		for j := range rsNN {
			s = gfMul(s, gfPow(rsFCR+i)) ^ work[j]
		}
		syndromes[i] = s
		if s != 0 {
			allZero = false
		}
	}

	if allZero {
		return work[:rs.DataLen()], 0, nil
	}

	// Step 2: Berlekamp-Massey → error-locator polynomial σ(x)
	sigma, nerrs, err := rs.berlekampMassey(syndromes)
	if err != nil {
		return nil, 0, err
	}

	// Step 3: Chien search → error positions
	errPos := rs.chienSearch(sigma, nerrs)
	if errPos == nil {
		return nil, 0, ErrUncorrectable
	}

	// Step 4: Forney algorithm → error magnitudes, correct in-place
	rs.forney(work, syndromes, sigma, errPos)

	return work[:rs.DataLen()], len(errPos), nil
}

// berlekampMassey computes the error-locator polynomial using the
// Berlekamp-Massey algorithm. Returns the polynomial, degree (number
// of errors), and any error.
func (rs *RSCodec) berlekampMassey(syndromes []byte) ([]byte, int, error) {
	n := rs.nroots
	// σ(x): error-locator polynomial
	sigma := make([]byte, n+1)
	sigma[0] = 1
	// B(x): auxiliary polynomial
	B := make([]byte, n+1)
	B[0] = 1

	L := 0 // current number of assumed errors

	for k := range n {
		// Compute discrepancy Δ
		delta := syndromes[k]
		for j := 1; j <= L; j++ {
			delta ^= gfMul(sigma[j], syndromes[k-j])
		}

		// Shift B: B(x) = x * B(x)
		copy(B[1:], B)
		B[0] = 0

		if delta != 0 {
			T := make([]byte, n+1)
			copy(T, sigma)

			// σ(x) = σ(x) - Δ * B(x)
			for j := range n + 1 {
				sigma[j] ^= gfMul(delta, B[j])
			}

			if 2*L <= k {
				L = k + 1 - L
				// B(x) = Δ^{-1} * T(x)
				inv := gfInv(delta)
				for j := range n + 1 {
					B[j] = gfMul(T[j], inv)
				}
			}
		}
	}

	if L > rs.nroots/2 {
		return nil, 0, ErrUncorrectable
	}

	return sigma, L, nil
}

// chienSearch finds the roots of the error-locator polynomial σ(x)
// by exhaustive evaluation. If σ(α^{-i}) == 0, the error is at codeword
// byte index (254-i), since byte j corresponds to the coefficient of x^(254-j).
// Returns the codeword byte indices or nil if the count doesn't match.
func (rs *RSCodec) chienSearch(sigma []byte, nerrs int) []int {
	var positions []int

	for i := range rsNN {
		xiInv := gfPow(255 - i) // α^{-i}
		if evalPoly(sigma, xiInv) == 0 {
			pos := (rsNN - 1) - i // map to codeword byte index
			if pos >= 0 && pos < rsNN {
				positions = append(positions, pos)
			}
		}
	}

	if len(positions) != nerrs {
		return nil
	}
	return positions
}

// evalPoly evaluates polynomial p at point x using Horner's method.
// p[0] is the constant term.
func evalPoly(p []byte, x byte) byte {
	val := byte(0)
	xPow := byte(1)
	for _, coeff := range p {
		val ^= gfMul(coeff, xPow)
		xPow = gfMul(xPow, x)
	}
	return val
}

// forney computes error magnitudes using the Forney algorithm and
// corrects the codeword in-place.
func (rs *RSCodec) forney(codeword []byte, syndromes []byte, sigma []byte, errPos []int) {
	n := rs.nroots

	// Compute error-evaluator polynomial:
	// Ω(x) = S(x) · σ(x) mod x^nroots
	omega := make([]byte, n)
	for i := range n {
		val := byte(0)
		for j := range i + 1 {
			if j < len(sigma) {
				val ^= gfMul(syndromes[i-j], sigma[j])
			}
		}
		omega[i] = val
	}

	// Formal derivative of σ(x) in characteristic 2:
	// σ'(x) = σ_1 + σ_3·x^2 + σ_5·x^4 + ... (only odd-indexed coefficients)
	sigmaD := make([]byte, len(sigma))
	for j := 1; j < len(sigma); j += 2 {
		sigmaD[j-1] = sigma[j]
	}

	for _, pos := range errPos {
		// Byte at codeword[pos] corresponds to x^(254-pos)
		power := (rsNN - 1) - pos
		xiInv := gfPow(255 - power) // X_i^{-1}

		omegaVal := evalPoly(omega, xiInv)
		sigmaDVal := evalPoly(sigmaD, xiInv)

		if sigmaDVal == 0 {
			continue
		}

		// Forney: e_i = X_i^{1-FCR} · Ω(X_i^{-1}) / σ'(X_i^{-1})
		magnitude := gfMul(gfMul(gfPow(power*(1-rsFCR)), omegaVal), gfInv(sigmaDVal))
		codeword[pos] ^= magnitude
	}
}

// EncodeInterleaved encodes data using symbol interleaving at the given depth.
// Input length must be exactly depth * DataLen() bytes.
// Returns a slice of length depth * 255.
func (rs *RSCodec) EncodeInterleaved(data []byte, depth int) ([]byte, error) {
	if !validInterleaveDepth(depth) {
		return nil, ErrInvalidInterleaveDepth
	}
	k := rs.DataLen()
	if len(data) != depth*k {
		return nil, ErrInvalidDataLength
	}

	codewords := make([][]byte, depth)
	for d := range depth {
		block := make([]byte, k)
		for i := range k {
			block[i] = data[i*depth+d]
		}
		cw, err := rs.Encode(block)
		if err != nil {
			return nil, err
		}
		codewords[d] = cw
	}

	out := make([]byte, depth*rsNN)
	for i := range rsNN {
		for d := range depth {
			out[i*depth+d] = codewords[d][i]
		}
	}
	return out, nil
}

// DecodeInterleaved decodes interleaved data, correcting errors.
// Input length must be exactly depth * 255 bytes.
// Returns corrected data of length depth * DataLen(), total corrections, and error.
func (rs *RSCodec) DecodeInterleaved(data []byte, depth int) ([]byte, int, error) {
	if !validInterleaveDepth(depth) {
		return nil, 0, ErrInvalidInterleaveDepth
	}
	if len(data) != depth*rsNN {
		return nil, 0, ErrInvalidDataLength
	}

	k := rs.DataLen()
	totalCorr := 0

	codewords := make([][]byte, depth)
	for d := range depth {
		cw := make([]byte, rsNN)
		for i := range rsNN {
			cw[i] = data[i*depth+d]
		}
		codewords[d] = cw
	}

	decoded := make([][]byte, depth)
	for d := range depth {
		corrected, corr, err := rs.Decode(codewords[d])
		if err != nil {
			return nil, 0, err
		}
		decoded[d] = corrected
		totalCorr += corr
	}

	out := make([]byte, depth*k)
	for i := range k {
		for d := range depth {
			out[i*depth+d] = decoded[d][i]
		}
	}
	return out, totalCorr, nil
}

func validInterleaveDepth(depth int) bool {
	switch depth {
	case 1, 2, 3, 4, 5, 8:
		return true
	}
	return false
}
