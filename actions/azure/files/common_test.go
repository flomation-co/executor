package files

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"
)

// testAccount / testKey are Azurite's well-known development credentials.
// Azurite implements no File service, so they are used here purely as a fixed,
// public key pair for byte-exact signature vectors — never as an emulator
// target.
const (
	testAccount = "devstoreaccount1"
	testKey     = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
)

// fixedNow pins the clock so x-ms-date — and therefore the signature — is
// deterministic. 17 Jul 2026 is a Friday.
var fixedNow = time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)

const fixedDateHeader = "Fri, 17 Jul 2026 10:00:00 GMT"

func pinClock(t *testing.T) {
	t.Helper()
	prev := nowFunc
	nowFunc = func() time.Time { return fixedNow }
	t.Cleanup(func() { nowFunc = prev })
}

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

// expectSharedKeyAuth turns a LITERAL string-to-sign into the full expected
// Authorization header. The literal is hand-built from the official spec — only
// the final HMAC step is computed here.
func expectSharedKeyAuth(t *testing.T, a Auth, literalStringToSign string) string {
	t.Helper()
	return "SharedKey " + testAccount + ":" + hmacSHA256B64(a.AccountKey, literalStringToSign)
}

// ---------------------------------------------------------------------------
// Shared Key signing vectors
// ---------------------------------------------------------------------------

// TestSharedKeyAuthorizationHeaderVector drives a canned request through Do()
// against a capture server and compares the FULL Authorization header with one
// derived from a hand-written string-to-sign.
//
// This is the vector that backs the whole "reuse the Blob signer" claim:
// Microsoft documents ONE Shared Key scheme for Blob, Queue and File, and the
// literal below is the File service's own documented layout, written from the
// spec rather than copied from the Blob test.
func TestSharedKeyAuthorizationHeaderVector(t *testing.T) {
	pinClock(t)

	// The string-to-sign for:
	//   PUT /myshare/reports/hello world.txt?comp=metadata
	//   x-ms-meta-owner: ops (plus the always-on x-ms-date / x-ms-version, and
	//   x-ms-allow-trailing-dot, which every below-share path carries)
	//   empty body
	want := "PUT\n" + // VERB
		"\n" + // Content-Encoding
		"\n" + // Content-Language
		"\n" + // Content-Length — empty when 0
		"\n" + // Content-MD5
		"\n" + // Content-Type
		"\n" + // Date — empty; x-ms-date takes precedence
		"\n" + // If-Modified-Since
		"\n" + // If-Match
		"\n" + // If-None-Match
		"\n" + // If-Unmodified-Since
		"\n" + // Range
		"x-ms-allow-trailing-dot:true\n" +
		"x-ms-date:" + fixedDateHeader + "\n" +
		"x-ms-meta-owner:ops\n" +
		"x-ms-version:" + APIVersion + "\n" +
		"/" + testAccount + "/myshare/reports/hello%20world.txt" + "\n" +
		"comp:metadata"

	var gotAuth, gotPath, gotDate, gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.EscapedPath()
		gotDate = r.Header.Get("x-ms-date")
		gotVersion = r.Header.Get("x-ms-version")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := testAuth(t, srv.URL)
	resp, err := Do(&core.Flow{}, a, Request{
		Method:  http.MethodPut,
		Path:    FilePath("myshare", "reports", "hello world.txt"),
		Query:   url.Values{"comp": []string{"metadata"}},
		Headers: map[string]string{"x-ms-meta-owner": "ops"},
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if want := expectSharedKeyAuth(t, a, want); gotAuth != want {
		t.Errorf("Authorization header:\n got: %s\nwant: %s", gotAuth, want)
	}
	if gotPath != "/myshare/reports/hello%20world.txt" {
		t.Errorf("request path = %q, want the segment-escaped file path", gotPath)
	}
	if gotDate != fixedDateHeader {
		t.Errorf("x-ms-date = %q, want %q", gotDate, fixedDateHeader)
	}
	if gotVersion != APIVersion {
		t.Errorf("x-ms-version = %q, want %q", gotVersion, APIVersion)
	}
}

// TestSharedKeyOfficialSlotOrderVector pins the slot order the Blob node's
// comment defends: Content-Encoding comes BEFORE Content-Language, and a
// present body signs Content-Length as its decimal length. The Range slot is
// exercised too, because Put Range is the one File operation that always sets it
// — a signer that dropped the slot would work everywhere except uploads.
func TestSharedKeyOfficialSlotOrderVector(t *testing.T) {
	pinClock(t)

	want := "PUT\n" +
		"gzip\n" + // Content-Encoding FIRST
		"en-GB\n" + // Content-Language second
		"5\n" + // Content-Length of "hello"
		"\n" + // Content-MD5
		"text/plain\n" + // Content-Type
		"\n" + // Date
		"\n\n\n\n" + // If-Modified-Since, If-Match, If-None-Match, If-Unmodified-Since
		"bytes=0-4\n" + // Range
		"x-ms-allow-trailing-dot:true\n" +
		"x-ms-date:" + fixedDateHeader + "\n" +
		"x-ms-version:" + APIVersion + "\n" +
		"x-ms-write:update\n" +
		"/" + testAccount + "/myshare/blob.txt\n" +
		"comp:range"

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	a := testAuth(t, srv.URL)
	if _, err := Do(&core.Flow{}, a, Request{
		Method: http.MethodPut,
		Path:   FilePath("myshare", "", "blob.txt"),
		Query:  url.Values{"comp": []string{"range"}},
		Headers: map[string]string{
			"Content-Encoding": "gzip",
			"Content-Language": "en-GB",
			"Range":            "bytes=0-4",
			"x-ms-write":       "update",
		},
		Body:        []byte("hello"),
		ContentType: "text/plain",
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if want := expectSharedKeyAuth(t, a, want); gotAuth != want {
		t.Errorf("Authorization header:\n got: %s\nwant: %s", gotAuth, want)
	}
}

// TestSharedKeyPathStyleHost proves the path-style endpoint signs the SAME
// canonicalized resource as the host-style default: the account appears once,
// prefixed to the path actually sent, which for this host style already carries
// it — so it legitimately appears TWICE. Inherited from the Blob node, where
// signing the logical path instead cost a flat 403 from a live service; no
// httptest server can catch that, because a mock validates no signature.
func TestSharedKeyPathStyleHost(t *testing.T) {
	pinClock(t)

	want := "GET\n\n\n\n\n\n\n\n\n\n\n\n" +
		"x-ms-date:" + fixedDateHeader + "\n" +
		"x-ms-version:" + APIVersion + "\n" +
		"/" + testAccount + "/" + testAccount + "/myshare\n" +
		"restype:share"

	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.EscapedPath()
	}))
	defer srv.Close()

	a := testAuth(t, srv.URL+"/"+testAccount)
	if _, err := Do(&core.Flow{}, a, Request{
		Method: http.MethodGet,
		Path:   SharePath("myshare"),
		Query:  url.Values{"restype": []string{"share"}},
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if want := expectSharedKeyAuth(t, a, want); gotAuth != want {
		t.Errorf("Authorization header:\n got: %s\nwant: %s", gotAuth, want)
	}
	if gotPath != "/"+testAccount+"/myshare" {
		t.Errorf("request path = %q, want the account-prefixed path", gotPath)
	}
}

// TestDoDerivesTheFileHost — the single line that makes this the File node.
// Deriving the Blob host here would point all 20 actions at the wrong service
// with a signature that is otherwise perfectly valid.
func TestDoDerivesTheFileHost(t *testing.T) {
	a, err := GetAuth([]*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "mystorageaccount"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
	})
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if a.BaseURL != "https://mystorageaccount.file.core.windows.net" {
		t.Errorf("derived BaseURL = %q, want the .file. host", a.BaseURL)
	}
}

// ---------------------------------------------------------------------------
// Entra: the backup-intent header
// ---------------------------------------------------------------------------

// TestEntraSendsFileRequestIntent — every OAuth request to the File service must
// carry x-ms-file-request-intent: backup or the service answers a bare 400. It
// is the one header with no Blob counterpart, so nothing but this test stops the
// copied Do() from shipping without it and failing 100% of Entra calls.
func TestEntraSendsFileRequestIntent(t *testing.T) {
	defer SetTokenForTest("tok-123")()

	var gotIntent, gotAuthz string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIntent = r.Header.Get("x-ms-file-request-intent")
		gotAuthz = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	a, err := GetAuth([]*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: testAccount},
		{Name: "auth_method", Type: core.ConnectionTypeString, Value: AuthEntra},
		{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Value: "tenant"},
		{Name: "azure_client_id", Type: core.ConnectionTypeString, Value: "client"},
		{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Value: "s3cret"},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: srv.URL},
	})
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if _, err := Do(&core.Flow{}, a, Request{Method: http.MethodGet, Path: SharePath("s"), Query: url.Values{}}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotIntent != "backup" {
		t.Errorf("x-ms-file-request-intent = %q, want %q — without it every OAuth call 400s", gotIntent, "backup")
	}
	if gotAuthz != "Bearer tok-123" {
		t.Errorf("Authorization = %q, want the bearer token", gotAuthz)
	}
}

// TestSharedKeyDoesNotSendFileRequestIntent — the header requests backup
// semantics, which bypass the share's ACLs. Shared Key does not need it and must
// not silently ask for it.
func TestSharedKeyDoesNotSendFileRequestIntent(t *testing.T) {
	var gotIntent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIntent = r.Header.Get("x-ms-file-request-intent")
	}))
	defer srv.Close()

	if _, err := Do(&core.Flow{}, testAuth(t, srv.URL), Request{Method: http.MethodGet, Path: SharePath("s"), Query: url.Values{}}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotIntent != "" {
		t.Errorf("x-ms-file-request-intent = %q on a Shared Key call — it bypasses the share's permissions and has no business here", gotIntent)
	}
}

// ---------------------------------------------------------------------------
// Trailing dots
// ---------------------------------------------------------------------------

// TestAllowTrailingDot — without the header the service SILENTLY TRIMS a
// trailing dot from a file or directory name, so "report." is created as
// "report" and every later call naming "report." 404s. It applies below the
// share only.
func TestAllowTrailingDot(t *testing.T) {
	for _, tc := range []struct {
		name       string
		path       string
		wantHeader bool
	}{
		{"account root", "/", false},
		{"share", SharePath("myshare"), false},
		{"file in the share root", FilePath("myshare", "", "report."), true},
		{"file in a directory", FilePath("myshare", "reports", "q1."), true},
	} {
		var got string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Get("x-ms-allow-trailing-dot")
		}))
		if _, err := Do(&core.Flow{}, testAuth(t, srv.URL), Request{Method: http.MethodGet, Path: tc.path, Query: url.Values{}}); err != nil {
			t.Fatalf("%s: Do: %v", tc.name, err)
		}
		srv.Close()
		if want := ""; !tc.wantHeader && got != want {
			t.Errorf("%s: x-ms-allow-trailing-dot = %q, want none — a share name cannot end in a dot", tc.name, got)
		}
		if tc.wantHeader && got != "true" {
			t.Errorf("%s: x-ms-allow-trailing-dot = %q, want \"true\" — without it the service trims the dot and the name stops matching", tc.name, got)
		}
	}
}

// TestSourceAllowTrailingDot — a copy source is trimmed by the same rule, so the
// source twin rides along whenever the request names one.
func TestSourceAllowTrailingDot(t *testing.T) {
	var got, gotDest string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("x-ms-source-allow-trailing-dot")
		gotDest = r.Header.Get("x-ms-allow-trailing-dot")
	}))
	defer srv.Close()

	if _, err := Do(&core.Flow{}, testAuth(t, srv.URL), Request{
		Method:  http.MethodPut,
		Path:    FilePath("myshare", "", "dest."),
		Query:   url.Values{},
		Headers: map[string]string{"x-ms-copy-source": "https://acct.file.core.windows.net/s/src."},
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got != "true" || gotDest != "true" {
		t.Errorf("copy trailing-dot headers = source %q dest %q, want both \"true\"", got, gotDest)
	}
}

// ---------------------------------------------------------------------------
// SAS vectors
// ---------------------------------------------------------------------------

// TestServiceSASFileVector compares BuildServiceSAS against a hand-written
// FILE service-SAS string-to-sign (2020-12-06+ layout: THIRTEEN fields, rsct
// last, no trailing newline). Only the HMAC is computed here.
//
// The field COUNT is the point, and it is what this vector originally got
// wrong. It asserted Blob's 16-field layout — signedResource, signedSnapshotTime
// and signedEncryptionScope after signedVersion — because it was written from
// the same "Blob and File are one scheme" premise as the code it checks, so the
// two agreed with each other and both disagreed with Azure. Every generated link
// 403'd in production while this test was green: a vector restating the code's
// own assumption proves only that the assumption is held consistently.
//
// The layout below is the one the LIVE service echoes back in
// AuthenticationErrorDetail when a signature does not match, which is the only
// authority that settles it. Shared Key REQUEST signing really is shared across
// Blob/Queue/File — the service SAS is not.
func TestServiceSASFileVector(t *testing.T) {
	want := "r\n" + // signedPermissions
		"\n" + // signedStart
		"2026-07-18T10:00:00Z\n" + // signedExpiry
		"/file/" + testAccount + "/myshare/reports/hello.txt\n" + // canonicalizedResource (DECODED names)
		"\n" + // signedIdentifier
		"\n" + // signedIP
		"\n" + // signedProtocol
		APIVersion + "\n" + // signedVersion — Blob has signedResource/SnapshotTime/EncryptionScope next; File does NOT
		"\n" + // rscc
		"\n" + // rscd
		"\n" + // rsce
		"\n" + // rscl
		"" // rsct — final field, no trailing newline

	a := testAuth(t, "")
	token, err := BuildServiceSAS(a, SASParams{
		Resource:    SASResourceFile,
		Share:       "myshare",
		Path:        "reports/hello.txt",
		Permissions: "r",
		Expiry:      time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildServiceSAS: %v", err)
	}
	q, err := url.ParseQuery(token)
	if err != nil {
		t.Fatalf("token does not parse as a query string: %v", err)
	}
	if got, want := q.Get("sig"), hmacSHA256B64(a.AccountKey, want); got != want {
		t.Errorf("sig = %q, want %q", got, want)
	}
	for param, want := range map[string]string{
		"sv": APIVersion, "sr": "f", "sp": "r", "se": "2026-07-18T10:00:00Z",
	} {
		if got := q.Get(param); got != want {
			t.Errorf("%s = %q, want %q", param, got, want)
		}
	}
	if q.Get("st") != "" || q.Get("sip") != "" || q.Get("rscd") != "" {
		t.Errorf("unset optional fields must be omitted from the token, got %q", token)
	}
}

// TestServiceSASShareVector covers the share-resource shape (sr=s, no path
// segment) plus the optional start/IP/disposition fields.
//
// rscd is the slot that moves. On Blob's layout it is the 13th field; on File's
// it is the 10th, immediately after rscc. Signing the Blob layout therefore put
// the operator's Content-Disposition three slots from where the service reads
// it — so this vector pins the position, not just the value.
func TestServiceSASShareVector(t *testing.T) {
	want := "rcl\n" + // signedPermissions
		"2026-07-17T09:00:00Z\n" + // signedStart
		"2026-07-18T10:00:00Z\n" + // signedExpiry
		"/file/" + testAccount + "/myshare\n" + // canonicalizedResource
		"\n" + // signedIdentifier
		"10.0.0.1-10.0.0.9\n" + // signedIP
		"\n" + // signedProtocol
		APIVersion + "\n" + // signedVersion — sr is NOT signed on the File service
		"\n" + // rscc
		"attachment\n" + // rscd
		"\n" + // rsce
		"\n" + // rscl
		"" // rsct

	a := testAuth(t, "")
	token, err := BuildServiceSAS(a, SASParams{
		Resource:           SASResourceShare,
		Share:              "myshare",
		Permissions:        "rcl",
		Start:              time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC),
		Expiry:             time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC),
		IP:                 "10.0.0.1-10.0.0.9",
		ContentDisposition: "attachment",
	})
	if err != nil {
		t.Fatalf("BuildServiceSAS: %v", err)
	}
	q, _ := url.ParseQuery(token)
	if got, want := q.Get("sig"), hmacSHA256B64(a.AccountKey, want); got != want {
		t.Errorf("sig = %q, want %q", got, want)
	}
	for param, want := range map[string]string{
		"sr": "s", "sp": "rcl", "st": "2026-07-17T09:00:00Z",
		"sip": "10.0.0.1-10.0.0.9", "rscd": "attachment",
	} {
		if got := q.Get(param); got != want {
			t.Errorf("%s = %q, want %q", param, got, want)
		}
	}
}

func TestBuildServiceSASRequiresSharedKey(t *testing.T) {
	a := Auth{AccountName: testAccount, Method: AuthEntra}
	if _, err := BuildServiceSAS(a, SASParams{Resource: SASResourceFile, Share: "s", Path: "f", Permissions: "r", Expiry: fixedNow}); err == nil {
		t.Fatal("expected an error for Entra auth — a service SAS needs the account key")
	}
}

// TestValidateSASPermissions — the Files alphabet is "rcwdl", NOT blob's
// "racwdxltmei". Accepting a blob permission here would sign a token the service
// rejects, and "l" is share-only.
func TestValidateSASPermissions(t *testing.T) {
	for _, ok := range []struct{ perms, resource string }{
		{"r", SASResourceFile},
		{"rcwd", SASResourceFile},
		{"w", SASResourceFile},
		{"rcwdl", SASResourceShare},
		{"rl", SASResourceShare},
	} {
		if err := ValidateSASPermissions(ok.perms, ok.resource); err != nil {
			t.Errorf("ValidateSASPermissions(%q, %q) = %v, want nil", ok.perms, ok.resource, err)
		}
	}
	for _, bad := range []struct {
		perms, resource, why string
	}{
		{"", SASResourceFile, "empty"},
		{"cr", SASResourceFile, "out of order"},
		{"rr", SASResourceFile, "duplicated"},
		{"a", SASResourceFile, "append is a blob permission, not a file one"},
		{"x", SASResourceFile, "delete-version is a blob permission"},
		{"t", SASResourceFile, "tags are a blob concept"},
		{"l", SASResourceFile, "list applies to a share, not a single file"},
		{"rcwdl", SASResourceFile, "list applies to a share, not a single file"},
	} {
		if err := ValidateSASPermissions(bad.perms, bad.resource); err == nil {
			t.Errorf("ValidateSASPermissions(%q, %q) = nil, want an error (%s)", bad.perms, bad.resource, bad.why)
		}
	}
}

// ---------------------------------------------------------------------------
// Canonicalization pieces
// ---------------------------------------------------------------------------

func TestCanonicalizedHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("x-ms-version", APIVersion)
	h.Set("x-ms-date", "d")
	h.Set("x-ms-meta-b", " padded ")
	h.Set("x-ms-type", "file")
	h.Set("Content-Type", "text/plain") // not x-ms-*, excluded
	got := canonicalizedHeaders(h)
	want := "x-ms-date:d\n" +
		"x-ms-meta-b:padded\n" +
		"x-ms-type:file\n" +
		"x-ms-version:" + APIVersion + "\n"
	if got != want {
		t.Errorf("canonicalizedHeaders:\n got: %q\nwant: %q", got, want)
	}
}

func TestCanonicalizedResource(t *testing.T) {
	q := url.Values{}
	q.Set("comp", "list")
	q.Set("Prefix", "a b") // decoded value; key lowercased
	got := canonicalizedResource(testAccount, "/", q)
	want := "/" + testAccount + "/\ncomp:list\nprefix:a b"
	if got != want {
		t.Errorf("canonicalizedResource:\n got: %q\nwant: %q", got, want)
	}
	if got := canonicalizedResource(testAccount, "", nil); got != "/"+testAccount+"/" {
		t.Errorf("empty path canonicalizes to %q, want account root", got)
	}
}

func TestPathHelpers(t *testing.T) {
	for name, tc := range map[string]struct{ got, want string }{
		"share":                   {SharePath("my-share"), "/my-share"},
		"root directory":          {DirectoryPath("s", ""), "/s"},
		"nested directory":        {DirectoryPath("s", "reports/2026"), "/s/reports/2026"},
		"directory with slashes":  {DirectoryPath("s", "/reports/"), "/s/reports"},
		"file in the root":        {FilePath("s", "", "a.txt"), "/s/a.txt"},
		"file in a directory":     {FilePath("s", "reports", "a.txt"), "/s/reports/a.txt"},
		"file with a space":       {FilePath("s", "", "hello world.txt"), "/s/hello%20world.txt"},
		"file with a hash":        {FilePath("s", "", "odd#name.txt"), "/s/odd%23name.txt"},
		"directory with a space":  {FilePath("s", "my reports", "a.txt"), "/s/my%20reports/a.txt"},
		"file name with a period": {FilePath("s", "", "report."), "/s/report."},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", name, tc.got, tc.want)
		}
	}

	for name, tc := range map[string]struct{ got, want string }{
		"root":   {JoinPath("", "a.txt"), "a.txt"},
		"nested": {JoinPath("reports/2026", "a.txt"), "reports/2026/a.txt"},
		"padded": {JoinPath("/reports/", "a.txt"), "reports/a.txt"},
	} {
		if tc.got != tc.want {
			t.Errorf("JoinPath %s = %q, want %q", name, tc.got, tc.want)
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
	if len(a.AccountKey) == 0 {
		t.Error("account key did not decode")
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
		"unknown auth method": {
			{Name: "account_name", Type: core.ConnectionTypeString, Value: testAccount},
			{Name: "auth_method", Type: core.ConnectionTypeString, Value: "sas"},
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

func TestValidateShareName(t *testing.T) {
	for _, ok := range []string{"abc", "my-share", "a1b2", "123", strings.Repeat("a", 63)} {
		if err := ValidateShareName(ok); err != nil {
			t.Errorf("ValidateShareName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"ab", "My-Share", "-abc", "abc-", "a--b", "a_b", strings.Repeat("a", 64), "a b"} {
		if err := ValidateShareName(bad); err == nil {
			t.Errorf("ValidateShareName(%q) = nil, want an error", bad)
		}
	}
}

// TestValidateFilePath — file and directory names are NOT share names. They are
// case-preserving and take a wide charset, so reusing the share rule on them
// would reject perfectly legal paths; only the SMB-reserved characters are out.
func TestValidateFilePath(t *testing.T) {
	for _, ok := range []string{
		"a.txt",
		"Reports 2026/Q1 Summary.pdf", // capitals AND spaces — the share rule rejects both
		"UPPER/MiXeD/case.TXT",
		"under_score-and.dots",
		"trailing.dot.",   // kept, not trimmed — see applyTrailingDot
		"unicode/café.md", // non-ASCII is legal
		strings.Repeat("a", 255),
	} {
		if err := ValidateFilePath("file_name", ok); err != nil {
			t.Errorf("ValidateFilePath(%q) = %v, want nil", ok, err)
		}
	}
	for name, bad := range map[string]string{
		"empty":              "",
		"leading slash":      "/a.txt",
		"trailing slash":     "a/",
		"double slash":       "a//b.txt",
		"colon":              "a:b.txt",
		"pipe":               "a|b.txt",
		"asterisk":           "a*.txt",
		"question mark":      "a?.txt",
		"quote":              `a".txt`,
		"backslash":          `a\b.txt`,
		"angle brackets":     "a<b>.txt",
		"segment too long":   strings.Repeat("a", 256),
		"whole path too big": strings.Repeat("a/", 600),
	} {
		if err := ValidateFilePath("file_name", bad); err == nil {
			t.Errorf("ValidateFilePath(%s: %q) = nil, want an error", name, bad)
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

// TestDoRejectsABodyOverTheCap — the cap must FAIL, not clip. A LimitReader at
// the cap returns io.EOF with no error, so an oversized file would otherwise
// come back as a truncated prefix that every caller reports as a success.
func TestDoRejectsABodyOverTheCap(t *testing.T) {
	const bodyCap = 1 << 20

	serve := func(n int) *httptest.Server {
		body := bytes.Repeat([]byte("x"), n)
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(body)
		}))
	}

	// Exactly at the cap is a legitimate response, not an error.
	atCap := serve(bodyCap)
	defer atCap.Close()
	resp, err := Do(&core.Flow{}, testAuth(t, atCap.URL), Request{
		Method: http.MethodGet, Path: FilePath("s", "", "at-cap.bin"), Query: url.Values{}, MaxBody: bodyCap,
	})
	if err != nil {
		t.Fatalf("a body of exactly the cap must succeed, got: %v", err)
	}
	if len(resp.Body) != bodyCap {
		t.Errorf("body = %d bytes, want the full %d", len(resp.Body), bodyCap)
	}

	// One byte past it is a truncation, and must be reported as one.
	overCap := serve(bodyCap + 1)
	defer overCap.Close()
	_, err = Do(&core.Flow{}, testAuth(t, overCap.URL), Request{
		Method: http.MethodGet, Path: FilePath("s", "", "over-cap.bin"), Query: url.Values{}, MaxBody: bodyCap,
	})
	if err == nil {
		t.Fatal("a body past the cap must error — silently truncating it reports success on corrupt data")
	}
	for _, want := range []string{"1 MB", "Byte Range", "Copy File"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q must name the cap and the way out (%q)", err, want)
		}
	}
}

// TestOverCapErrorRemedyMatchesTheCap — the envelope cap and the download cap
// have different remedies; an operator told to use the Byte Range input on an
// 8 MB list response has been sent nowhere.
func TestOverCapErrorRemedyMatchesTheCap(t *testing.T) {
	download := overCapError(true, MaxDownloadBody).Error()
	if !strings.Contains(download, "256 MB") || !strings.Contains(download, "Byte Range") {
		t.Errorf("download cap error = %q", download)
	}
	envelope := overCapError(false, maxResponseBody).Error()
	if !strings.Contains(envelope, "8 MB") || !strings.Contains(envelope, "Limit") {
		t.Errorf("envelope cap error = %q", envelope)
	}
	if strings.Contains(envelope, "Byte Range") {
		t.Errorf("a list response over the cap is not fixed by a byte range: %q", envelope)
	}
}

// ---------------------------------------------------------------------------
// Error envelope & redaction
// ---------------------------------------------------------------------------

func TestCheckResponse(t *testing.T) {
	if err := CheckResponse(&APIResponse{StatusCode: 201}); err != nil {
		t.Errorf("2xx must pass, got %v", err)
	}

	xmlBody := []byte(`<?xml version="1.0" encoding="utf-8"?><Error><Code>ShareNotFound</Code><Message>The specified share does not exist.
RequestId:xyz</Message></Error>`)
	err := CheckResponse(&APIResponse{StatusCode: 404, Body: xmlBody, Headers: http.Header{}})
	if err == nil || !strings.Contains(err.Error(), "ShareNotFound: The specified share does not exist.") {
		t.Errorf("XML error not surfaced: %v", err)
	}
	if strings.Contains(err.Error(), "RequestId") {
		t.Errorf("RequestId noise must be trimmed: %v", err)
	}

	// HEAD failures have no body — the x-ms-error-code header is the fallback.
	h := http.Header{}
	h.Set("x-ms-error-code", "ResourceNotFound")
	err = CheckResponse(&APIResponse{StatusCode: 404, Headers: h})
	if err == nil || !strings.Contains(err.Error(), "ResourceNotFound") {
		t.Errorf("header fallback not surfaced: %v", err)
	}

	if code := ErrorCode(&APIResponse{StatusCode: 404, Body: xmlBody, Headers: http.Header{}}); code != "ShareNotFound" {
		t.Errorf("ErrorCode = %q", code)
	}
	if code := ErrorCode(&APIResponse{StatusCode: 200, Headers: http.Header{}}); code != "" {
		t.Errorf("ErrorCode on a 2xx = %q, want empty", code)
	}
}

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
// se/sp/sv are not credentials, they are provenance.
func TestRedactURL(t *testing.T) {
	got := RedactURL("https://other.file.core.windows.net/s/f.bin?sv=2023-11-03&sp=r&sig=abc123%2Fdef%3D")
	if strings.Contains(got, "abc123") {
		t.Errorf("SAS signature survived into output: %q", got)
	}
	if !strings.Contains(got, "sig=REDACTED") {
		t.Errorf("redacted URL = %q, want the sig slot marked", got)
	}
	for _, keep := range []string{"sv=2023-11-03", "sp=r", "/s/f.bin"} {
		if !strings.Contains(got, keep) {
			t.Errorf("redacted URL = %q, want %q kept — it is provenance, not a credential", got, keep)
		}
	}
	if got := RedactURL("https://acct.file.core.windows.net/s/f.bin"); got != "https://acct.file.core.windows.net/s/f.bin" {
		t.Errorf("a URL with no SAS must pass through untouched, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// include tokens
// ---------------------------------------------------------------------------

func TestParseIncludeTokens(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"", ""},
		{"metadata", "metadata"},
		// The whole point: values COMBINE, so one pass can carry both.
		{"metadata,snapshots", "metadata,snapshots"},
		{" Metadata , SNAPSHOTS ", "metadata,snapshots"},
		{"metadata,,snapshots,", "metadata,snapshots"},
		{"metadata,snapshots,metadata", "metadata,snapshots"},
		{"deleted", "deleted"},
	} {
		got, err := ParseIncludeTokens(tc.raw, ShareIncludeTokens)
		if err != nil {
			t.Errorf("ParseIncludeTokens(%q) = %v, want nil", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseIncludeTokens(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}

	// An unknown token is caught here — the service answers one with a flat 400
	// that names nothing. "tags" is the near-miss that matters: it is a Blob
	// token, and this node is not Blob.
	for _, raw := range []string{"snapshot", "tags", "metadata,everything"} {
		got, err := ParseIncludeTokens(raw, ShareIncludeTokens)
		if err == nil {
			t.Errorf("ParseIncludeTokens(%q) = %q, want an error naming the supported values", raw, got)
			continue
		}
		if !strings.Contains(err.Error(), "metadata") {
			t.Errorf("error %q must list what IS supported", err)
		}
	}
}

// ---------------------------------------------------------------------------
// XML envelopes
// ---------------------------------------------------------------------------

func TestListSharesParsing(t *testing.T) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<EnumerationResults ServiceEndpoint="https://x">
  <Shares>
    <Share>
      <Name>my-share</Name>
      <Properties>
        <Last-Modified>Fri, 17 Jul 2026 10:00:00 GMT</Last-Modified>
        <Etag>0x8D</Etag>
        <Quota>5120</Quota>
        <AccessTier>Hot</AccessTier>
        <EnabledProtocols>SMB</EnabledProtocols>
      </Properties>
      <Metadata><team>ops</team></Metadata>
    </Share>
  </Shares>
  <NextMarker>cursor-1</NextMarker>
</EnumerationResults>`
	var env EnumerationResults
	if err := xml.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.NextMarker != "cursor-1" || len(env.Shares) != 1 {
		t.Fatalf("envelope = %+v", env)
	}
	m := ShareMap(env.Shares[0])
	props := m["properties"].(map[string]interface{})
	if props["quota"] != int64(5120) {
		t.Errorf("quota = %#v, want int64(5120)", props["quota"])
	}
	if props["accessTier"] != "Hot" || props["lastModified"] == nil {
		t.Errorf("camelCased properties missing: %#v", props)
	}
	if m["metadata"].(map[string]interface{})["team"] != "ops" {
		t.Errorf("metadata = %#v", m["metadata"])
	}
}

// TestListDirectoryParsing pins the envelope that most distinguishes Files from
// Blob: ONE <Entries> element holding both <File> and <Directory> children.
// A blob listing has no such thing — it fakes hierarchy with name prefixes — so
// this envelope has no Blob-side test to have been copied from.
func TestListDirectoryParsing(t *testing.T) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<EnumerationResults ServiceEndpoint="https://x" ShareName="my-share" DirectoryPath="reports">
  <Entries>
    <Directory>
      <Name>2026</Name>
      <Properties><Last-Modified>Fri, 17 Jul 2026 10:00:00 GMT</Last-Modified></Properties>
    </Directory>
    <File>
      <Name>summary.pdf</Name>
      <Properties><Content-Length>1024</Content-Length></Properties>
    </File>
    <File>
      <Name>notes.txt</Name>
      <Properties><Content-Length>7</Content-Length></Properties>
    </File>
  </Entries>
  <NextMarker />
</EnumerationResults>`
	var env EnumerationResults
	if err := xml.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(env.Directories) != 1 || len(env.Files) != 2 {
		t.Fatalf("entries = %d directories, %d files; want 1 and 2", len(env.Directories), len(env.Files))
	}

	// The `type` key is what a flow branches on: unlike a blob listing there is
	// no name convention to infer a directory from.
	d := EntryMap(env.Directories[0], "directory")
	if d["name"] != "2026" || d["type"] != "directory" {
		t.Errorf("directory entry = %#v", d)
	}
	f := EntryMap(env.Files[0], "file")
	if f["name"] != "summary.pdf" || f["type"] != "file" {
		t.Errorf("file entry = %#v", f)
	}
	if props := f["properties"].(map[string]interface{}); props["contentLength"] != int64(1024) {
		t.Errorf("contentLength = %#v, want int64(1024)", props["contentLength"])
	}
}

// TestListMetadataMatchesTheHeaderPath is the shape contract between the two
// ways a flow can read the same metadata: the list envelope (<Metadata> XML) and
// the properties GET (x-ms-meta-* headers). They must agree, or a filter written
// against one silently matches nothing on the other.
//
// The two transforms <Properties> needs are both corrupting here: camelKey
// mangles the operator's name (ORDER_ID → oRDER_ID) and coerceScalar eats a
// zero-padded reference ("0012" → 12).
func TestListMetadataMatchesTheHeaderPath(t *testing.T) {
	const body = `<?xml version="1.0" encoding="utf-8"?>
<EnumerationResults>
  <Shares>
    <Share>
      <Name>s</Name>
      <Metadata><Project_Code>0012</Project_Code><ORDER_ID>00123</ORDER_ID><Archived>true</Archived></Metadata>
    </Share>
  </Shares>
</EnumerationResults>`

	var env EnumerationResults
	if err := xml.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The same metadata as the service returns it on a properties GET.
	h := http.Header{}
	h.Set("x-ms-meta-Project_Code", "0012")
	h.Set("x-ms-meta-ORDER_ID", "00123")
	h.Set("x-ms-meta-Archived", "true")
	want := HeadersResult("s", h)["metadata"]

	if got := want.(map[string]interface{}); got["project_code"] != "0012" {
		t.Fatalf("the header path itself changed shape: %#v", got)
	}
	if got := ShareMap(env.Shares[0])["metadata"]; !reflect.DeepEqual(got, want) {
		t.Errorf("share_get_all metadata disagrees with share_get_properties:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestShareStatsParsing(t *testing.T) {
	var stats ShareStats
	if err := xml.Unmarshal([]byte(`<?xml version="1.0" encoding="utf-8"?><ShareStats><ShareUsageBytes>1234567</ShareUsageBytes></ShareStats>`), &stats); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if stats.ShareUsageBytes != 1234567 {
		t.Errorf("ShareUsageBytes = %d, want 1234567", stats.ShareUsageBytes)
	}
}

func TestRangeListParsing(t *testing.T) {
	var list RangeList
	body := `<?xml version="1.0" encoding="utf-8"?>
<Ranges>
  <Range><Start>0</Start><End>511</End></Range>
  <Range><Start>1024</Start><End>1535</End></Range>
  <ClearRange><Start>512</Start><End>1023</End></ClearRange>
</Ranges>`
	if err := xml.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.Ranges) != 2 || len(list.ClearRanges) != 1 {
		t.Fatalf("ranges = %+v", list)
	}
	if list.Ranges[1].Start != 1024 || list.Ranges[1].End != 1535 {
		t.Errorf("second range = %+v", list.Ranges[1])
	}
	if list.ClearRanges[0].Start != 512 {
		t.Errorf("clear range = %+v", list.ClearRanges[0])
	}
}

func TestHeadersResult(t *testing.T) {
	h := http.Header{}
	h.Set("ETag", `"0x8D"`)
	h.Set("Last-Modified", "Fri, 17 Jul 2026 10:00:00 GMT")
	h.Set("Content-Length", "42")
	h.Set("Content-Type", "text/plain")
	h.Set("x-ms-type", "File")
	h.Set("x-ms-lease-status", "unlocked")
	h.Set("x-ms-server-encrypted", "true")
	h.Set("x-ms-meta-owner", "ops")
	h.Set("x-ms-request-id", "noise")
	h.Set("Date", "noise")
	h.Set("Server", "noise")

	out := HeadersResult("a.txt", h)
	props := out["properties"].(map[string]interface{})
	meta := out["metadata"].(map[string]interface{})
	if out["name"] != "a.txt" {
		t.Errorf("name = %v", out["name"])
	}
	if props["type"] != "File" || props["leaseStatus"] != "unlocked" {
		t.Errorf("x-ms props = %#v", props)
	}
	if props["serverEncrypted"] != true {
		t.Errorf("serverEncrypted = %#v, want bool", props["serverEncrypted"])
	}
	if props["contentLength"] != int64(42) {
		t.Errorf("contentLength = %#v", props["contentLength"])
	}
	if meta["owner"] != "ops" {
		t.Errorf("metadata = %#v", meta)
	}
	if _, leaked := props["requestId"]; leaked {
		t.Error("x-ms-request-id must be dropped as transport noise")
	}
}

// TestListEnumerationPagination walks two marker pages and checks the marker
// round-trip plus aggregation; a non-return_all call must stop after one page.
func TestListEnumerationPagination(t *testing.T) {
	var markers []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		markers = append(markers, r.URL.Query().Get("marker"))
		w.Header().Set("Content-Type", "application/xml")
		if r.URL.Query().Get("marker") == "" {
			_, _ = w.Write([]byte(`<EnumerationResults><Shares><Share><Name>one</Name></Share></Shares><NextMarker>m2</NextMarker></EnumerationResults>`))
			return
		}
		_, _ = w.Write([]byte(`<EnumerationResults><Shares><Share><Name>two</Name></Share></Shares><NextMarker></NextMarker></EnumerationResults>`))
	}))
	defer srv.Close()

	a := testAuth(t, srv.URL)
	shares, _, _, truncated, err := ListEnumeration(&core.Flow{}, a, "/", url.Values{"comp": []string{"list"}}, true, 50)
	if err != nil {
		t.Fatalf("ListEnumeration: %v", err)
	}
	if truncated {
		t.Error("two pages must not report truncation")
	}
	if len(shares) != 2 || shares[0].Name != "one" || shares[1].Name != "two" {
		t.Errorf("shares = %+v", shares)
	}
	if len(markers) != 2 || markers[1] != "m2" {
		t.Errorf("marker round-trip = %v", markers)
	}

	markers = nil
	shares, _, _, _, err = ListEnumeration(&core.Flow{}, a, "/", url.Values{"comp": []string{"list"}}, false, 50)
	if err != nil {
		t.Fatalf("single page: %v", err)
	}
	if len(shares) != 1 || len(markers) != 1 {
		t.Errorf("non-return_all fetched %d pages, want 1", len(markers))
	}
}

func TestStringMapInputAndMetadataHeaders(t *testing.T) {
	inputs := []*core.Connection{
		{Name: "metadata", Type: core.ConnectionTypeObject, Value: `{"owner":"ops","count":3}`},
	}
	h := map[string]string{}
	if err := MetadataHeaders(h, inputs, "metadata"); err != nil {
		t.Fatalf("MetadataHeaders: %v", err)
	}
	if h["x-ms-meta-owner"] != "ops" || h["x-ms-meta-count"] != "3" {
		t.Errorf("headers = %#v", h)
	}

	bad := []*core.Connection{{Name: "metadata", Type: core.ConnectionTypeObject, Value: `{"1bad":"x"}`}}
	if err := MetadataHeaders(map[string]string{}, bad, "metadata"); err == nil {
		t.Error("invalid metadata name must be rejected")
	}
	notObj := []*core.Connection{{Name: "metadata", Type: core.ConnectionTypeObject, Value: `[1,2]`}}
	if err := MetadataHeaders(map[string]string{}, notObj, "metadata"); err == nil {
		t.Error("non-object metadata must be rejected")
	}
}

// ---------------------------------------------------------------------------
// Leases
// ---------------------------------------------------------------------------

// TestBuildLeaseCall — a FILE lease is infinite-only, so acquire always signs
// -1 and there is no duration to read. The rest is the same lifecycle the Blob
// node has, minus renew.
func TestBuildLeaseCall(t *testing.T) {
	const guid = "8b1c6a2e-0f9d-4a3b-9c5e-7d2f1a4b6c8d"

	acquire, err := BuildLeaseCall([]*core.Connection{{Name: "lease_action", Value: "acquire"}})
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if acquire.Headers["x-ms-lease-duration"] != "-1" {
		t.Errorf("acquire duration = %q, want -1 — a file lease has no finite form", acquire.Headers["x-ms-lease-duration"])
	}
	if _, set := acquire.Headers["x-ms-proposed-lease-id"]; set {
		t.Error("a blank proposed_lease_id must send no header — the service mints the ID")
	}

	change, err := BuildLeaseCall([]*core.Connection{
		{Name: "lease_action", Value: "change"},
		{Name: "lease_id", Value: guid},
		{Name: "proposed_lease_id", Value: "0f9d4a3b-8b1c-6a2e-9c5e-7d2f1a4b6c8d"},
	})
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	if change.Headers["x-ms-lease-id"] != guid {
		t.Errorf("change headers = %#v", change.Headers)
	}

	// Break is the only action that does not need the ID — breaking a lease is
	// precisely what an operator who never had it does.
	brk, err := BuildLeaseCall([]*core.Connection{{Name: "lease_action", Value: "break"}})
	if err != nil {
		t.Fatalf("break: %v", err)
	}
	if _, set := brk.Headers["x-ms-lease-id"]; set {
		t.Error("break without a lease_id must send no lease header")
	}

	for name, inputs := range map[string][]*core.Connection{
		"no action":           {},
		"unknown action":      {{Name: "lease_action", Value: "renew"}}, // renew is a BLOB action, rejected on a file
		"release without id":  {{Name: "lease_action", Value: "release"}},
		"change without id":   {{Name: "lease_action", Value: "change"}, {Name: "proposed_lease_id", Value: guid}},
		"change without new":  {{Name: "lease_action", Value: "change"}, {Name: "lease_id", Value: guid}},
		"acquire non-guid":    {{Name: "lease_action", Value: "acquire"}, {Name: "proposed_lease_id", Value: "not-a-guid"}},
		"change new non-guid": {{Name: "lease_action", Value: "change"}, {Name: "lease_id", Value: guid}, {Name: "proposed_lease_id", Value: "nope"}},
	} {
		if _, err := BuildLeaseCall(inputs); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

// TestLeaseHeader — a BLANK lease_id must send no header at all: an empty
// x-ms-lease-id is not "no lease", it is an invalid one, and the service answers
// 400 rather than performing the unleased operation the operator meant.
func TestLeaseHeader(t *testing.T) {
	if h := LeaseHeader(nil, []*core.Connection{{Name: "lease_id", Value: ""}}); h != nil {
		t.Errorf("blank lease_id produced headers %#v, want none", h)
	}
	h := LeaseHeader(map[string]string{"x-ms-write": "update"}, []*core.Connection{{Name: "lease_id", Value: "abc"}})
	if h["x-ms-lease-id"] != "abc" || h["x-ms-write"] != "update" {
		t.Errorf("headers = %#v", h)
	}
}
