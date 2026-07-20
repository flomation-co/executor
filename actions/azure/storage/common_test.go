package storage

import (
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"
)

// testAccount / testKey are Azurite's well-known development credentials —
// fixed, public, and convenient for byte-exact signature vectors.
const (
	testAccount = "devstoreaccount1"
	testKey     = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
)

// fixedNow pins the clock so x-ms-date — and therefore the signature — is
// deterministic. 16 Jul 2026 is a Thursday.
var fixedNow = time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)

const fixedDateHeader = "Thu, 16 Jul 2026 10:00:00 GMT"

func testAuth(t *testing.T, endpoint string) Auth {
	t.Helper()
	a, err := GetAuth([]*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: testAccount},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
	})
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	return a
}

// ---------------------------------------------------------------------------
// Shared Key signing vectors
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// SAS vectors
// ---------------------------------------------------------------------------

func TestValidateSASPermissions(t *testing.T) {
	for _, ok := range []string{"r", "racwd", "rl", "racwdxltmei", "ei", "w"} {
		if err := ValidateSASPermissions(ok); err != nil {
			t.Errorf("ValidateSASPermissions(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "wr", "rr", "z", "rB", "lr"} {
		if err := ValidateSASPermissions(bad); err == nil {
			t.Errorf("ValidateSASPermissions(%q) = nil, want an error", bad)
		}
	}
}

// ---------------------------------------------------------------------------
// Canonicalization pieces
// ---------------------------------------------------------------------------

func TestBlobPathEscaping(t *testing.T) {
	for input, want := range map[string]string{
		"plain.txt":        "/c/plain.txt",
		"dir/sub/file.txt": "/c/dir/sub/file.txt",
		"hello world.txt":  "/c/hello%20world.txt",
		"odd#name?.txt":    "/c/odd%23name%3F.txt",
	} {
		if got := BlobPath("c", input); got != want {
			t.Errorf("BlobPath(%q) = %q, want %q", input, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Auth resolution & validation
// ---------------------------------------------------------------------------

func TestGetAuth(t *testing.T) {
	a := testAuth(t, "")
	if a.Method != AuthSharedKey {
		t.Errorf("empty auth_method must default to shared_key, got %q", a.Method)
	}
	if a.BaseURL != "https://"+testAccount+".blob.core.windows.net" {
		t.Errorf("derived BaseURL = %q", a.BaseURL)
	}
	if a.rawKey == "" {
		t.Error("account key was not retained for the SDK credential")
	}

	for name, inputs := range map[string][]*core.Connection{
		"missing account": {
			{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
		},
		"bad account charset": {
			{Name: "account_name", Type: core.ConnectionTypeString, Value: "bad_account!"},
			{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
		},
		"bad base64 key": {
			{Name: "account_name", Type: core.ConnectionTypeString, Value: testAccount},
			{Name: "account_key", Type: core.ConnectionTypeSecret, Value: "not-base64!!!"},
		},
		"entra missing secret": {
			{Name: "account_name", Type: core.ConnectionTypeString, Value: testAccount},
			{Name: "auth_method", Type: core.ConnectionTypeString, Value: "entra"},
			{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Value: "tenant"},
			{Name: "azure_client_id", Type: core.ConnectionTypeString, Value: "client"},
		},
		"bad endpoint": {
			{Name: "account_name", Type: core.ConnectionTypeString, Value: testAccount},
			{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
			{Name: "endpoint", Type: core.ConnectionTypeString, Value: "ftp://host"},
		},
	} {
		if _, err := GetAuth(inputs); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestValidateContainerName(t *testing.T) {
	for _, ok := range []string{"abc", "my-container", "a1b2", "123", strings.Repeat("a", 63)} {
		if err := ValidateContainerName(ok); err != nil {
			t.Errorf("ValidateContainerName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"ab", "My-Container", "-abc", "abc-", "a--b", "a_b", strings.Repeat("a", 64), "a b"} {
		if err := ValidateContainerName(bad); err == nil {
			t.Errorf("ValidateContainerName(%q) = nil, want an error", bad)
		}
	}
}

func TestClampLimit(t *testing.T) {
	for _, tc := range []struct{ in, want int }{{0, DefaultPageLimit}, {-3, DefaultPageLimit}, {1, 1}, {5000, 5000}, {9999, MaxPageLimit}} {
		if got := ClampLimit(tc.in, true); got != tc.want {
			t.Errorf("ClampLimit(%d, true) = %d, want %d", tc.in, got, tc.want)
		}
	}
	if got := ClampLimit(0, false); got != DefaultPageLimit {
		t.Errorf("ClampLimit unset = %d, want default", got)
	}
}

// ---------------------------------------------------------------------------
// Response body caps
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Error envelope & redaction
// ---------------------------------------------------------------------------

func TestRedact(t *testing.T) {
	a := testAuth(t, "")
	msg := "call failed: key " + testKey + " url https://x/y?sv=1&sig=SECRETSIG&sp=r"
	got := a.redact(msg)
	if strings.Contains(got, testKey) {
		t.Error("account key leaked through redact")
	}
	if strings.Contains(got, "SECRETSIG") {
		t.Error("SAS signature leaked through redact")
	}

	e := Auth{ClientSecret: "s3cr3t-value"}
	if strings.Contains(e.redact("boom s3cr3t-value boom"), "s3cr3t-value") {
		t.Error("client secret leaked through redact")
	}
}

// TestRedactURL — an output-bound URL loses its SAS signature and nothing else:
// the snapshot/version identify WHICH source was read, and se/sp/sv are not
// credentials.
func TestRedactURL(t *testing.T) {
	got := RedactURL("https://other.blob.core.windows.net/c/b.bin?snapshot=2026-07-16T10:00:00Z&sv=2023-11-03&sp=r&sig=abc123%2Fdef%3D")
	if strings.Contains(got, "abc123") {
		t.Errorf("SAS signature survived into output: %q", got)
	}
	if !strings.Contains(got, "sig=REDACTED") {
		t.Errorf("redacted URL = %q, want the sig slot marked", got)
	}
	for _, keep := range []string{"snapshot=2026-07-16T10:00:00Z", "sv=2023-11-03", "sp=r", "/c/b.bin"} {
		if !strings.Contains(got, keep) {
			t.Errorf("redacted URL = %q, want %q kept — it is provenance, not a credential", got, keep)
		}
	}
	if got := RedactURL("https://acct.blob.core.windows.net/c/b.bin"); got != "https://acct.blob.core.windows.net/c/b.bin" {
		t.Errorf("a URL with no SAS must pass through untouched, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// include tokens
// ---------------------------------------------------------------------------

func TestParseIncludeTokens(t *testing.T) {
	for _, tc := range []struct {
		raw, want string
		allowed   []string
	}{
		{"", "", BlobIncludeTokens},
		{"metadata", "metadata", BlobIncludeTokens},
		// The whole point: values COMBINE, so one pass can carry both.
		{"metadata,tags", "metadata,tags", BlobIncludeTokens},
		{" Metadata , TAGS ", "metadata,tags", BlobIncludeTokens},
		{"metadata,,tags,", "metadata,tags", BlobIncludeTokens},
		{"metadata,tags,metadata", "metadata,tags", BlobIncludeTokens},
		{"uncommittedblobs,copy", "uncommittedblobs,copy", BlobIncludeTokens},
		{"metadata,deleted,system", "metadata,deleted,system", ContainerIncludeTokens},
	} {
		got, err := ParseIncludeTokens(tc.raw, tc.allowed)
		if err != nil {
			t.Errorf("ParseIncludeTokens(%q) = %v, want nil", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseIncludeTokens(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}

	// An unknown token is caught here — the service answers one with a flat 400
	// that names nothing.
	for _, tc := range []struct {
		raw     string
		allowed []string
	}{
		{"snapshot", BlobIncludeTokens}, // near-miss for "snapshots"
		{"metadata,everything", BlobIncludeTokens},
		{"tags", ContainerIncludeTokens}, // a blob token, not a container one
	} {
		got, err := ParseIncludeTokens(tc.raw, tc.allowed)
		if err == nil {
			t.Errorf("ParseIncludeTokens(%q) = %q, want an error naming the supported values", tc.raw, got)
			continue
		}
		if !strings.Contains(err.Error(), "metadata") {
			t.Errorf("error %q must list what IS supported", err)
		}
	}
}

// ---------------------------------------------------------------------------
// XML envelopes & input helpers
// ---------------------------------------------------------------------------
