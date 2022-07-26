package jaws

import (
	crand "crypto/rand"
	"math/rand"
	"sync"
	"time"
)

// Keygen is a source of pseudo-random data seeded from true random data.
type Keygen struct {
	mu   sync.Mutex // protects following
	rnd  *rand.Rand // underlying PRNG
	used bool       // true if used since last reseed
}

// NewKeygen creates a new key generator.
// Calls Reseed on the newly created Keygen before returning it.
func NewKeygen() (kg *Keygen) {
	kg = &Keygen{rnd: rand.New(rand.NewSource(time.Now().Unix()))}
	kg.Reseed()
	return
}

// IsUsed returns true if the Keygen has been used since last Reseed()
func (kg *Keygen) IsUsed() (b bool) {
	kg.mu.Lock()
	b = kg.used
	kg.mu.Unlock()
	return
}

// Int63 returns 63 bits of pseudo-random data. It is safe to use by concurrent goroutines.
func (kg *Keygen) Int63() (v int64) {
	kg.mu.Lock()
	v = kg.rnd.Int63()
	kg.used = true
	kg.mu.Unlock()
	return
}

// Reseed seeds the PRNG with cryptographically sound random data. It is safe to use by concurrent goroutines.
// Returns the number of bytes of cryptographically sound random data used to reseed the PRNG.
func (kg *Keygen) Reseed() (n int) {
	buf := make([]byte, 8)
	if n, _ = crand.Reader.Read(buf); n > 0 {
		seed := kg.Int63()
		for i := 0; i < n; i++ {
			seed = (seed << 8) ^ int64(buf[i])
		}
		if seed < 0 {
			seed = -seed
		}
		kg.mu.Lock()
		kg.rnd.Seed(seed)
		kg.used = false
		kg.mu.Unlock()
	}
	return
}
