package testgen

import "fmt"

// DefaultLower is used when no lower bound is specified.
const DefaultLower int64 = -100

// DefaultUpper is used when no upper bound is specified.
const DefaultUpper int64 = 100

// RandomCount is the number of random values generated per parameter.
const RandomCount = 20

// GenerateIntValues produces boundary + random Rust i64 expression strings for an Int parameter.
func GenerateIntValues(c *ParamConstraint) []string {
	lo := DefaultLower
	hi := DefaultUpper
	if c.Lower != nil {
		lo = *c.Lower
	}
	if c.Upper != nil {
		hi = *c.Upper
	}
	// Clamp upper if somehow inverted
	if hi < lo {
		hi = lo
	}

	// Build boundary values (deduplicated)
	seen := make(map[int64]bool)
	var boundary []int64

	addIfInRange := func(v int64) {
		if v >= lo && v <= hi && !seen[v] {
			seen[v] = true
			boundary = append(boundary, v)
		}
	}

	addIfInRange(lo)
	addIfInRange(lo + 1)
	addIfInRange(0)
	addIfInRange(1)
	if hi > 1 {
		addIfInRange(hi - 1)
	}
	addIfInRange(hi)

	// Filter out excluded values
	var filtered []int64
	excludeSet := make(map[int64]bool)
	for _, ne := range c.NotEqual {
		excludeSet[ne] = true
	}
	for _, v := range boundary {
		if !excludeSet[v] {
			filtered = append(filtered, v)
		}
	}

	// Convert to Rust expressions
	var values []string
	for _, v := range filtered {
		values = append(values, fmt.Sprintf("%di64", v))
	}

	// Add random values (using xorshift64 seed)
	rng := uint64(0x517cc1b727220a95)
	for i := 0; i < RandomCount; i++ {
		rng = xorshift64(rng)
		v := randRange(rng, lo, hi)
		if !excludeSet[v] {
			values = append(values, fmt.Sprintf("%di64", v))
		}
	}

	return values
}

// GenerateFloatValues produces boundary + random Rust f64 expression strings for a Float parameter.
func GenerateFloatValues(c *ParamConstraint) []string {
	lo := float64(DefaultLower)
	hi := float64(DefaultUpper)
	if c.Lower != nil {
		lo = float64(*c.Lower)
	}
	if c.Upper != nil {
		hi = float64(*c.Upper)
	}

	values := []string{
		formatFloat(0.0),
		formatFloat(1.0),
		formatFloat(-1.0),
		formatFloat(lo),
		formatFloat(hi),
	}

	// Add random float values
	rng := uint64(0x517cc1b727220a95)
	for i := 0; i < RandomCount; i++ {
		rng = xorshift64(rng)
		// Scale to [lo, hi]
		frac := float64(rng) / float64(^uint64(0))
		v := lo + frac*(hi-lo)
		values = append(values, formatFloat(v))
	}

	return values
}

// GenerateBoolValues returns the two boolean values.
func GenerateBoolValues() []string {
	return []string{"true", "false"}
}

// GenerateStringValues returns a fixed set of test strings.
func GenerateStringValues() []string {
	return []string{
		`"test".to_string()`,
		`"a".to_string()`,
		`"hello world".to_string()`,
	}
}

// GenerateArrayIntValues produces test arrays for Array<Int> parameters.
func GenerateArrayIntValues(c *ParamConstraint) []string {
	minLen := int64(1)
	if c.MinLen != nil {
		minLen = *c.MinLen
	}
	if minLen < 1 {
		minLen = 1
	}

	elemLo := int64(1)
	elemHi := int64(100)
	if c.ElemLower != nil {
		elemLo = *c.ElemLower
	}
	if c.ElemUpper != nil {
		elemHi = *c.ElemUpper
	}
	if elemHi < elemLo {
		elemHi = elemLo
	}

	var values []string

	// Array of exactly minLen elements
	values = append(values, makeArrayLiteral(minLen, elemLo, elemHi, 0x517cc1b727220a95))

	// Array of minLen+1 elements
	if minLen+1 <= 20 {
		values = append(values, makeArrayLiteral(minLen+1, elemLo, elemHi, 0x8a5cd789635d2dff))
	}

	// Array of 5 elements (if >= minLen)
	if int64(5) >= minLen {
		values = append(values, makeArrayLiteral(5, elemLo, elemHi, 0xdeadbeefcafebabe))
	}

	// A few more random-length arrays
	rng := uint64(0x12345678abcdef01)
	for i := 0; i < 3; i++ {
		rng = xorshift64(rng)
		arrLen := minLen + int64(rng%5)
		if arrLen > 20 {
			arrLen = 20
		}
		values = append(values, makeArrayLiteral(arrLen, elemLo, elemHi, rng))
	}

	return values
}

// makeArrayLiteral builds a Rust vec![...] literal of the given length with elements in [elemLo, elemHi].
func makeArrayLiteral(length, elemLo, elemHi int64, seed uint64) string {
	rng := seed
	result := "vec!["
	for i := int64(0); i < length; i++ {
		if i > 0 {
			result += ", "
		}
		rng = xorshift64(rng)
		v := randRange(rng, elemLo, elemHi)
		result += fmt.Sprintf("%di64", v)
	}
	result += "]"
	return result
}

// xorshift64 is a simple deterministic PRNG.
func xorshift64(state uint64) uint64 {
	state ^= state << 13
	state ^= state >> 7
	state ^= state << 17
	return state
}

// randRange maps a PRNG state to a value in [lo, hi].
func randRange(state uint64, lo, hi int64) int64 {
	if lo >= hi {
		return lo
	}
	r := hi - lo + 1
	return lo + int64(state%uint64(r))
}

// formatFloat formats a float64 as a Rust f64 literal.
func formatFloat(v float64) string {
	s := fmt.Sprintf("%g", v)
	// Ensure it has a decimal point for Rust
	hasDecimal := false
	for _, ch := range s {
		if ch == '.' || ch == 'e' || ch == 'E' {
			hasDecimal = true
			break
		}
	}
	if !hasDecimal {
		s += ".0"
	}
	return s + "f64"
}
