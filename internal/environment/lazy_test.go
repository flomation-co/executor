package environment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	. "github.com/onsi/gomega"
)

// NewEnvironment must NOT hit the network — the env id is resolved lazily only
// when a secret/property/credential is first referenced. This is the perf win:
// a flow that never touches the environment pays zero startup latency.
func TestNewEnvironment_LazyIdentifierResolution(t *testing.T) {
	RegisterTestingT(t)

	var envHits, secretHits int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/execution/exec-1/environment/myenv", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&envHits, 1)
		_ = json.NewEncoder(w).Encode(Summary{ID: "env-9", Name: "myenv"})
	})
	mux.HandleFunc("/api/v1/execution/exec-1/environment/env-9/secret/API_KEY", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&secretHits, 1)
		val := "s3cr3t"
		_ = json.NewEncoder(w).Encode(Secret{ID: "sec-1", Name: "API_KEY", Value: &val})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	url := srv.URL

	e, err := NewEnvironment("myenv", &url, "exec-1", nil)
	Expect(err).To(BeNil())
	// Construction did NOT resolve the environment (no network call).
	Expect(atomic.LoadInt32(&envHits)).To(Equal(int32(0)))

	// First secret access resolves the id (once) + fetches the secret.
	sec, err := e.GetSecret("API_KEY")
	Expect(err).To(BeNil())
	Expect(sec).ToNot(BeNil())
	Expect(*sec.Value).To(Equal("s3cr3t"))
	Expect(atomic.LoadInt32(&envHits)).To(Equal(int32(1)))
	Expect(atomic.LoadInt32(&secretHits)).To(Equal(int32(1)))

	// A second access reuses the resolved id (no extra env fetch); the secret
	// itself is cached for 30s so no second secret fetch either.
	_, err = e.GetSecret("API_KEY")
	Expect(err).To(BeNil())
	Expect(atomic.LoadInt32(&envHits)).To(Equal(int32(1)))
	Expect(atomic.LoadInt32(&secretHits)).To(Equal(int32(1)))
}
