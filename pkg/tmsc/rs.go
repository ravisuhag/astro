package tmsc

// Reed-Solomon codec for CCSDS TM Synchronization and Channel Coding
// per CCSDS 131.0-B-5 section 4.
//
// Supports (255,223) with 32 parity symbols (corrects up to 16 errors)
// and (255,239) with 16 parity symbols (corrects up to 8 errors).
//
// Field: GF(2^8) with primitive polynomial 0x187 (131.0-B-5 4.3.3).
// Generator roots are consecutive powers of β = α^11 starting at β^112:
// g(x) = ∏(x - β^(112+j)) for j = 0..nroots-1 (131.0-B-5 4.3.4).
// Symbols cross the Encode/Decode boundary in the dual (Berlekamp)
// basis as the standard requires (131.0-B-5 4.3.9, annex F); see tal.go.
// Shortened codeblocks via virtual fill (131.0-B-5 4.3.7, 4.3.8) are
// provided by EncodeShortened and DecodeShortened.

const (
	rsNN   = 255 // codeword length
	rsFCR  = 112 // first consecutive root (exponent of β)
	rsPrim = 11  // β = α^rsPrim; root spacing in powers of α
)

// gfPowB returns β^n where β = α^rsPrim.
func gfPowB(n int) byte {
	return gfPow(rsPrim * n)
}

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
	// g(x) = (x - β^FCR)(x - β^(FCR+1))...(x - β^(FCR+nroots-1))
	gen := make([]byte, nroots+1)
	gen[0] = 1

	for i := range nroots {
		root := gfPowB(rsFCR + i)
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

	// Wire bytes are dual-basis symbols; encoder arithmetic runs in the
	// conventional basis (CCSDS 131.0-B-5 4.3.9).
	cdata := make([]byte, k)
	copy(cdata, data)
	toConventional(cdata)

	parity := rs.parity(cdata)
	toDual(parity)
	copy(codeword[k:], parity)
	return codeword, nil
}

// parity computes the systematic check symbols for conventional-basis data:
// the remainder of data(x) * x^nroots divided by g(x). The data may be
// shorter than DataLen(); missing leading symbols are treated as zeros,
// which is exactly the virtual fill of CCSDS 131.0-B-5 4.3.7.3 (an encoder
// initially cleared at the start of a block).
func (rs *RSCodec) parity(cdata []byte) []byte {
	parity := make([]byte, rs.nroots)
	for i := range cdata {
		feedback := cdata[i] ^ parity[0]
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
	return parity
}

// Decode corrects errors in a 255-byte codeword and returns the corrected
// data (first DataLen() bytes), the number of corrected symbol errors,
// and any error. Returns ErrUncorrectable if errors exceed correction capability.
// The input slice is not modified.
func (rs *RSCodec) Decode(codeword []byte) ([]byte, int, error) {
	if len(codeword) != rsNN {
		return nil, 0, ErrInvalidDataLength
	}

	// Wire bytes are dual-basis symbols; decoder arithmetic runs in the
	// conventional basis. Corrected data is converted back at the end.
	work := make([]byte, rsNN)
	copy(work, codeword)
	toConventional(work)

	// Step 1: Compute syndromes S_i = R(β^(FCR+i))
	syndromes, allZero := rs.syndromes(work)
	if allZero {
		data := make([]byte, rs.DataLen())
		copy(data, codeword[:rs.DataLen()])
		return data, 0, nil
	}

	// Step 2: Berlekamp-Massey -> error-locator polynomial σ(x)
	sigma, nerrs, err := rs.berlekampMassey(syndromes)
	if err != nil {
		return nil, 0, err
	}

	// Step 3: Chien search -> error positions
	errPos := rs.chienSearch(sigma, nerrs)
	if errPos == nil {
		return nil, 0, ErrUncorrectable
	}

	// Step 4: Forney algorithm -> error magnitudes, correct in-place
	if err := rs.forney(work, syndromes, sigma, errPos); err != nil {
		return nil, 0, err
	}

	// Step 5: Recompute the syndromes over the corrected codeword. A decoder
	// that has been pushed past its correction capability can emerge from the
	// steps above with corrections that do not form a valid codeword; any
	// nonzero syndrome here means the result cannot be trusted.
	if _, allZero := rs.syndromes(work); !allZero {
		return nil, 0, ErrUncorrectable
	}

	data := work[:rs.DataLen()]
	toDual(data)
	return data, len(errPos), nil
}

// syndromes evaluates the received polynomial (conventional basis) at the
// generator roots β^(FCR+i) and reports whether every syndrome is zero,
// which is the definition of a valid codeword.
func (rs *RSCodec) syndromes(work []byte) ([]byte, bool) {
	// The logarithm of each root, taken once. A root is a power of alpha and
	// so never zero, which is what lets the multiply below drop to a single
	// table lookup: gfMul's zero test is only needed for the accumulator.
	var logRoots [rsNN]int
	for i := range rs.nroots {
		logRoots[i] = int(gfLog[gfPowB(rsFCR+i)])
	}

	// Horner's method, evaluated at every root in one pass over the codeword.
	//
	// The obvious shape is a pass per root, but each pass is a serial chain:
	// every octet's accumulator depends on the one before, so the processor
	// waits on two dependent table lookups per octet with nothing else to do.
	// The roots are independent of one another, so interleaving them gives it
	// nroots chains to overlap instead. The accumulators live in a stack
	// array for the same reason. A slice would add a bounds check and a
	// store to memory on every step.
	var acc [rsNN]byte

	for _, octet := range work {
		for i := range rs.nroots {
			s := acc[i]
			if s != 0 {
				s = gfExp[int(gfLog[s])+logRoots[i]]
			}
			acc[i] = s ^ octet
		}
	}

	syndromes := make([]byte, rs.nroots)
	allZero := true
	for i := range rs.nroots {
		syndromes[i] = acc[i]
		if acc[i] != 0 {
			allZero = false
		}
	}
	return syndromes, allZero
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
// by exhaustive evaluation. If σ(β^{-i}) == 0, the error is at codeword
// byte index (254-i), since byte j corresponds to the coefficient of x^(254-j).
// Returns the codeword byte indices or nil if the count doesn't match.
func (rs *RSCodec) chienSearch(sigma []byte, nerrs int) []int {
	var positions []int

	for i := range rsNN {
		xiInv := gfPowB(255 - i) // β^{-i}
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

// evalPoly evaluates polynomial p at point x using direct power accumulation.
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
// corrects the codeword in-place. It returns ErrUncorrectable if the formal
// derivative σ'(X^-1) evaluates to zero at a claimed error position: a root
// of σ with multiplicity, which no valid error pattern produces, so the
// decode as a whole cannot be trusted.
func (rs *RSCodec) forney(codeword []byte, syndromes []byte, sigma []byte, errPos []int) error {
	n := rs.nroots

	// Compute error-evaluator polynomial:
	// Ω(x) = S(x) | σ(x) mod x^nroots
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
	// σ'(x) = σ_1 + σ_3|x^2 + σ_5|x^4 + ... (only odd-indexed coefficients)
	sigmaD := make([]byte, len(sigma))
	for j := 1; j < len(sigma); j += 2 {
		sigmaD[j-1] = sigma[j]
	}

	for _, pos := range errPos {
		// Byte at codeword[pos] corresponds to x^(254-pos)
		power := (rsNN - 1) - pos
		xiInv := gfPowB(255 - power) // X_i^{-1}, X_i = β^power

		omegaVal := evalPoly(omega, xiInv)
		sigmaDVal := evalPoly(sigmaD, xiInv)

		if sigmaDVal == 0 {
			return ErrUncorrectable
		}

		// Forney: e_i = X_i^{1-FCR} | Ω(X_i^{-1}) / σ'(X_i^{-1})
		magnitude := gfMul(gfMul(gfPowB(power*(1-rsFCR)), omegaVal), gfInv(sigmaDVal))
		codeword[pos] ^= magnitude
	}
	return nil
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

// EncodeShortened encodes a shortened codeblock using virtual fill per
// CCSDS 131.0-B-5 4.3.7 and 4.3.8. The codeblock is logically completed by
// virtualFill zero symbols that precede the data; they are neither passed in
// nor transmitted, only fed to the encoder. A zero byte is zero in both the
// dual and the conventional basis, so the fill needs no basis transform.
//
// virtualFill must be a non-negative multiple of depth and smaller than
// depth * DataLen() (4.3.7.3, 4.3.8.2 e)). The input must be exactly
// depth*DataLen() - virtualFill bytes; the result is the transmitted
// codeblock of depth*255 - virtualFill bytes. With virtualFill = 0 this is
// EncodeInterleaved; a single shortened codeword is depth = 1.
func (rs *RSCodec) EncodeShortened(data []byte, depth, virtualFill int) ([]byte, error) {
	if !validInterleaveDepth(depth) {
		return nil, ErrInvalidInterleaveDepth
	}
	if !validVirtualFill(rs, depth, virtualFill) {
		return nil, ErrInvalidVirtualFill
	}
	if len(data) != depth*rs.DataLen()-virtualFill {
		return nil, ErrInvalidDataLength
	}

	full := make([]byte, depth*rs.DataLen())
	copy(full[virtualFill:], data)
	out, err := rs.EncodeInterleaved(full, depth)
	if err != nil {
		return nil, err
	}
	return out[virtualFill:], nil
}

// DecodeShortened decodes a shortened codeblock encoded with virtualFill
// symbols of virtual fill, correcting errors. It logically restores the
// leading zero symbols before decoding and strips them again afterwards.
// The input must be exactly depth*255 - virtualFill bytes; the result is the
// corrected data of depth*DataLen() - virtualFill bytes, with the total
// number of corrected symbol errors. Both ends must agree on virtualFill:
// the fill is not transmitted, so its length is a managed parameter
// (CCSDS 131.0-B-5 4.3.7.2 note 2).
func (rs *RSCodec) DecodeShortened(data []byte, depth, virtualFill int) ([]byte, int, error) {
	if !validInterleaveDepth(depth) {
		return nil, 0, ErrInvalidInterleaveDepth
	}
	if !validVirtualFill(rs, depth, virtualFill) {
		return nil, 0, ErrInvalidVirtualFill
	}
	if len(data) != depth*rsNN-virtualFill {
		return nil, 0, ErrInvalidDataLength
	}

	full := make([]byte, depth*rsNN)
	copy(full[virtualFill:], data)
	decoded, corr, err := rs.DecodeInterleaved(full, depth)
	if err != nil {
		return nil, 0, err
	}

	// The fill positions are known to be zero. A "correction" landing there
	// means the decoder settled on a codeword the transmitter cannot have
	// sent, which only happens past the correction capability.
	for _, b := range decoded[:virtualFill] {
		if b != 0 {
			return nil, 0, ErrUncorrectable
		}
	}
	return decoded[virtualFill:], corr, nil
}

// validVirtualFill checks the constraints of CCSDS 131.0-B-5 4.3.7.3 and
// 4.3.8.2 e): the fill is a non-negative multiple of the interleaving depth
// (Q symbols is 8|I|(Q/I) bits, an integer multiple of 8I) and leaves at
// least one information symbol per codeword.
func validVirtualFill(rs *RSCodec, depth, virtualFill int) bool {
	return virtualFill >= 0 &&
		virtualFill%depth == 0 &&
		virtualFill < depth*rs.DataLen()
}

func validInterleaveDepth(depth int) bool {
	switch depth {
	case 1, 2, 3, 4, 5, 8:
		return true
	}
	return false
}
