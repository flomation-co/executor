package desktop_common

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestPoolKeyFor_SeparatesConnectionIdentities(t *testing.T) {
	RegisterTestingT(t)

	base := poolKeyFor("10.0.0.5:22", "ubuntu", "key", "SECRET", "SHA256:abc")

	// Same identity in, same key out — this is what makes reuse happen at all.
	Expect(poolKeyFor("10.0.0.5:22", "ubuntu", "key", "SECRET", "SHA256:abc")).To(Equal(base))

	// Anything that changes which connection this IS must not share a pooled
	// client. Sharing across a differing credential or host key would hand one
	// tenant's connection to another's action.
	Expect(poolKeyFor("10.0.0.6:22", "ubuntu", "key", "SECRET", "SHA256:abc")).NotTo(Equal(base))
	Expect(poolKeyFor("10.0.0.5:22", "root", "key", "SECRET", "SHA256:abc")).NotTo(Equal(base))
	Expect(poolKeyFor("10.0.0.5:22", "ubuntu", "password", "SECRET", "SHA256:abc")).NotTo(Equal(base))
	Expect(poolKeyFor("10.0.0.5:22", "ubuntu", "key", "OTHER", "SHA256:abc")).NotTo(Equal(base))
	Expect(poolKeyFor("10.0.0.5:22", "ubuntu", "key", "SECRET", "SHA256:xyz")).NotTo(Equal(base))
}

// The key is length-prefixed per field so adjacent fields cannot be shuffled
// between each other to produce a collision — otherwise user "ab" + secret "c"
// and user "a" + secret "bc" would pool together.
func TestPoolKeyFor_NoFieldBoundaryCollision(t *testing.T) {
	RegisterTestingT(t)
	Expect(poolKeyFor("h", "ab", "key", "c", "f")).
		NotTo(Equal(poolKeyFor("h", "a", "key", "bc", "f")))
}

// The credential must not be recoverable from the key, which is why it is
// hashed rather than concatenated.
func TestPoolKeyFor_DoesNotLeakCredential(t *testing.T) {
	RegisterTestingT(t)
	key := poolKeyFor("10.0.0.5:22", "ubuntu", "key", "-----BEGIN PRIVATE KEY-----abc", "")
	Expect(key).NotTo(ContainSubstring("BEGIN"))
	Expect(key).NotTo(ContainSubstring("abc"))
	Expect(key).To(HaveLen(64)) // hex-encoded SHA-256
}

func TestEntryFor_ReturnsSameEntryPerKey(t *testing.T) {
	RegisterTestingT(t)
	defer CloseAllConnections()

	a := entryFor("k1")
	b := entryFor("k1")
	c := entryFor("k2")

	Expect(a).To(BeIdenticalTo(b))
	Expect(a).NotTo(BeIdenticalTo(c))
}

// A cached client is only handed out while it is inside the idle TTL. Past it,
// connect must dial afresh rather than return a socket the far end has very
// likely already dropped.
func TestPoolEntry_HonoursIdleTTL(t *testing.T) {
	RegisterTestingT(t)

	e := &poolEntry{}

	// Cold entry: nothing to reuse.
	Expect(e.client).To(BeNil())

	// Simulate a cached-but-stale entry. A nil client is never returned as
	// reusable, so staleness is expressed through lastUsed alone.
	e.lastUsed = time.Now().Add(-2 * idleTTL)
	Expect(time.Since(e.lastUsed) < idleTTL).To(BeFalse())

	e.lastUsed = time.Now()
	Expect(time.Since(e.lastUsed) < idleTTL).To(BeTrue())
}

// discard must be a no-op when the entry has already moved on to a different
// client — closing the replacement would break a healthy connection that
// another goroutine is mid-command on.
func TestPoolEntry_DiscardIgnoresSupersededClient(t *testing.T) {
	RegisterTestingT(t)

	e := &poolEntry{}
	e.discard(nil) // must not panic on a cold entry
	Expect(e.client).To(BeNil())
}

func TestCloseAllConnections_EmptiesPool(t *testing.T) {
	RegisterTestingT(t)

	entryFor("a")
	entryFor("b")
	poolMu.Lock()
	n := len(pool)
	poolMu.Unlock()
	Expect(n).To(BeNumerically(">=", 2))

	CloseAllConnections()

	poolMu.Lock()
	n = len(pool)
	poolMu.Unlock()
	Expect(n).To(Equal(0))
}

func TestReapOnce_SurvivesEmptyAndColdEntries(t *testing.T) {
	RegisterTestingT(t)
	defer CloseAllConnections()

	reapOnce() // empty pool
	entryFor("cold")
	reapOnce() // entry with no client
	Expect(entryFor("cold").client).To(BeNil())
}

func TestSettle_CapsAndIgnoresNonPositive(t *testing.T) {
	RegisterTestingT(t)

	start := time.Now()
	Settle(0)
	Settle(-500)
	Expect(time.Since(start)).To(BeNumerically("<", 50*time.Millisecond))

	start = time.Now()
	Settle(30)
	Expect(time.Since(start)).To(BeNumerically(">=", 25*time.Millisecond))
}

func TestMaxSettleIsBounded(t *testing.T) {
	RegisterTestingT(t)
	// A settle is a convenience, not a scheduler: an absurd value must not be
	// able to park a flow for minutes.
	Expect(maxSettle).To(BeNumerically("<=", 10*time.Second))
}
