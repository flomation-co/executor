package desktop_common

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSH connection reuse.
//
// Every Desktop action used to dial a brand-new SSH connection: TCP handshake,
// key exchange, authentication, then a single short command. An agent driving a
// desktop makes dozens of those in one execution (screenshot, click, screenshot,
// type, …), so the handshake cost dominated the actual work by a wide margin.
//
// SSH multiplexes many independent sessions over one connection, so the fix is
// to keep the client and open a fresh session per command. The pool is keyed on
// the connection identity (host, user, credential, host-key policy), so two
// actions pointing at the same VM with the same credentials share a connection
// while two different targets stay isolated.
//
// Lifetime: the executor is a process per execution, so the pool naturally dies
// with the flow — there is no cross-tenant reuse to reason about. Within that
// process a connection can still go stale (the VM reboots, sshd restarts, a NAT
// idles the socket out), so pooled connections are keepalive-pinged, evicted
// once idle, and every command retries exactly once on a fresh dial if the
// reused connection turns out to be dead.

const (
	// idleTTL evicts a pooled connection that has gone unused for this long.
	// Long enough to span an agent thinking between steps; short enough that a
	// suspended flow does not sit on an open socket indefinitely.
	idleTTL = 5 * time.Minute

	// keepAliveEvery pings live pooled connections so an idle NAT or an sshd
	// ClientAliveInterval does not silently drop them mid-flow.
	keepAliveEvery = 30 * time.Second
)

// poolEntry owns at most one live client for a given connection identity. The
// per-entry mutex (rather than one global lock) means a slow dial to one VM
// never blocks commands to another, and two goroutines racing for the same cold
// entry produce one dial rather than two.
type poolEntry struct {
	mu       sync.Mutex
	client   *ssh.Client
	lastUsed time.Time
}

var (
	poolMu     sync.Mutex
	pool       = map[string]*poolEntry{}
	reaperOnce sync.Once
)

// poolKeyFor hashes the connection identity. The credential is hashed rather
// than stored so no secret material sits in a map key.
func poolKeyFor(addr, user, authMethod, secret, fingerprint string) string {
	h := sha256.New()
	for _, part := range []string{addr, user, authMethod, secret, fingerprint} {
		// Length-prefix each part so ("ab","c") cannot collide with ("a","bc").
		_, _ = fmt.Fprintf(h, "%d:%s", len(part), part)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// entryFor returns the (possibly cold) pool entry for a key, creating it once.
func entryFor(key string) *poolEntry {
	poolMu.Lock()
	defer poolMu.Unlock()
	e, ok := pool[key]
	if !ok {
		e = &poolEntry{}
		pool[key] = e
	}
	return e
}

// connect returns a usable client, dialling only if the entry is cold or its
// cached connection has gone idle past the TTL. reused reports whether the
// returned client came from the cache, which tells the caller whether a
// transport failure is worth retrying on a fresh connection.
func (e *poolEntry) connect(addr string, config *ssh.ClientConfig) (client *ssh.Client, reused bool, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.client != nil {
		if time.Since(e.lastUsed) < idleTTL {
			e.lastUsed = time.Now()
			return e.client, true, nil
		}
		// Idle too long to trust; close it and dial fresh below.
		_ = e.client.Close()
		e.client = nil
	}

	fresh, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, false, err
	}
	e.client = fresh
	e.lastUsed = time.Now()
	startReaper()
	return fresh, false, nil
}

// discard drops a client from the cache if it is still the cached one. The
// identity check matters: another goroutine may already have replaced it, and
// closing the replacement would break a healthy connection.
func (e *poolEntry) discard(client *ssh.Client) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == client {
		e.client = nil
	}
	if client != nil {
		_ = client.Close()
	}
}

// startReaper launches the single background janitor: it keepalive-pings live
// connections and closes ones that have gone idle past the TTL.
func startReaper() {
	reaperOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(keepAliveEvery)
			defer ticker.Stop()
			for range ticker.C {
				reapOnce()
			}
		}()
	})
}

// reapOnce is one janitor pass, split out so tests can drive it directly.
func reapOnce() {
	poolMu.Lock()
	entries := make([]*poolEntry, 0, len(pool))
	for _, e := range pool {
		entries = append(entries, e)
	}
	poolMu.Unlock()

	for _, e := range entries {
		e.mu.Lock()
		client := e.client
		idle := time.Since(e.lastUsed)
		if client == nil {
			e.mu.Unlock()
			continue
		}
		if idle >= idleTTL {
			e.client = nil
			e.mu.Unlock()
			_ = client.Close()
			continue
		}
		e.mu.Unlock()

		// A failed keepalive means the connection is gone; drop it now rather
		// than letting the next command discover it and pay a retry.
		if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
			e.discard(client)
		}
	}
}

// CloseAllConnections closes every pooled connection and empties the pool.
// Not needed for normal operation (the process exits per execution) but keeps
// tests hermetic and gives a long-lived host an explicit shutdown hook.
func CloseAllConnections() {
	poolMu.Lock()
	entries := make([]*poolEntry, 0, len(pool))
	for _, e := range pool {
		entries = append(entries, e)
	}
	pool = map[string]*poolEntry{}
	poolMu.Unlock()

	for _, e := range entries {
		e.mu.Lock()
		client := e.client
		e.client = nil
		e.mu.Unlock()
		if client != nil {
			_ = client.Close()
		}
	}
}
