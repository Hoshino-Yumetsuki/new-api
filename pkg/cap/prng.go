package cap

// ponytail: port of capjs-core prng.js — must match widget PoW salts

func fnv1a(str string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(str); i++ {
		hash ^= uint32(str[i])
		hash += (hash << 1) + (hash << 4) + (hash << 7) + (hash << 8) + (hash << 24)
	}
	return hash
}

func fnv1aResume(state uint32, str string) uint32 {
	h := state
	for i := 0; i < len(str); i++ {
		h ^= uint32(str[i])
		h += (h << 1) + (h << 4) + (h << 7) + (h << 8) + (h << 24)
	}
	return h
}

func prngFromHash(initialHash uint32, length int) string {
	state := initialHash
	var result []byte
	for len(result) < length {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		hex := fmtHex8(state)
		result = append(result, hex...)
	}
	return string(result[:length])
}

func fmtHex8(v uint32) string {
	const hexdigits = "0123456789abcdef"
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = hexdigits[v&0xf]
		v >>= 4
	}
	return string(b[:])
}