package storage

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
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
// Authorization header. The literal is hand-built from the official spec —
// only the final HMAC step is computed here.
func expectSharedKeyAuth(t *testing.T, a Auth, literalStringToSign string) string {
	t.Helper()
	return "SharedKey " + testAccount + ":" + hmacSHA256B64(a.AccountKey, literalStringToSign)
}

// ---------------------------------------------------------------------------
// Shared Key signing vectors
// ---------------------------------------------------------------------------

// TestSharedKeyAuthorizationHeaderVector drives a canned request through Do()
// against a capture server and compares the FULL Authorization header with
// one derived from a hand-written string-to-sign. The zero-length body must
// sign as an EMPTY Content-Length slot, and the x-ms-* headers sort byte-wise.
func TestSharedKeyAuthorizationHeaderVector(t *testing.T) {
	pinClock(t)

	// The string-to-sign for:
	//   PUT /mycontainer/hello world.txt?comp=metadata
	//   x-ms-meta-owner: ops   (plus the always-on x-ms-date / x-ms-version)
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
		"x-ms-date:" + fixedDateHeader + "\n" +
		"x-ms-meta-owner:ops\n" +
		"x-ms-version:" + APIVersion + "\n" +
		"/" + testAccount + "/mycontainer/hello%20world.txt" + "\n" +
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
		Path:    BlobPath("mycontainer", "hello world.txt"),
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
	if gotPath != "/mycontainer/hello%20world.txt" {
		t.Errorf("request path = %q, want the segment-escaped blob path", gotPath)
	}
	if gotDate != fixedDateHeader {
		t.Errorf("x-ms-date = %q, want %q", gotDate, fixedDateHeader)
	}
	if gotVersion != APIVersion {
		t.Errorf("x-ms-version = %q, want %q", gotVersion, APIVersion)
	}
}

// TestSharedKeyOfficialSlotOrderVector pins the slot order n8n got wrong:
// Content-Encoding comes BEFORE Content-Language. A body is present, so
// Content-Length signs as its decimal length.
func TestSharedKeyOfficialSlotOrderVector(t *testing.T) {
	pinClock(t)

	want := "PUT\n" +
		"gzip\n" + // Content-Encoding FIRST (n8n signs en-GB here)
		"en-GB\n" + // Content-Language second
		"5\n" + // Content-Length of "hello"
		"\n" + // Content-MD5
		"text/plain\n" + // Content-Type
		"\n\n\n\n\n\n" + // Date, If-*, Range — all empty
		"x-ms-date:" + fixedDateHeader + "\n" +
		"x-ms-version:" + APIVersion + "\n" +
		"/" + testAccount + "/mycontainer/blob.txt"

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	a := testAuth(t, srv.URL)
	if _, err := Do(&core.Flow{}, a, Request{
		Method: http.MethodPut,
		Path:   BlobPath("mycontainer", "blob.txt"),
		Query:  url.Values{},
		Headers: map[string]string{
			"Content-Encoding": "gzip",
			"Content-Language": "en-GB",
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

// TestSharedKeyAzuriteHostStyle proves the path-style endpoint signs the SAME
// canonicalized resource as the host-style default: the account appears once,
// prefixed to the logical path, while the wire path carries it from the
// endpoint. The expected header is derived from the identical literal.
func TestSharedKeyAzuriteHostStyle(t *testing.T) {
	pinClock(t)

	// The canonicalized resource is "/{account}" + the path actually sent, and
	// Azurite's endpoint puts the account in that path — so the account
	// legitimately appears TWICE. Signing it once yields a flat 403 from a
	// real Azurite (verified against the emulator); no httptest server can
	// catch that, because a mock validates no signature.
	want := "GET\n\n\n\n\n\n\n\n\n\n\n\n" +
		"x-ms-date:" + fixedDateHeader + "\n" +
		"x-ms-version:" + APIVersion + "\n" +
		"/" + testAccount + "/" + testAccount + "/mycontainer" + "\n" +
		"restype:container"

	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.EscapedPath()
	}))
	defer srv.Close()

	// Azurite style: the account is the endpoint's leading path segment.
	a := testAuth(t, srv.URL+"/"+testAccount)
	if _, err := Do(&core.Flow{}, a, Request{
		Method: http.MethodGet,
		Path:   ContainerPath("mycontainer"),
		Query:  url.Values{"restype": []string{"container"}},
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if want := expectSharedKeyAuth(t, a, want); gotAuth != want {
		t.Errorf("Authorization header:\n got: %s\nwant: %s", gotAuth, want)
	}
	if gotPath != "/"+testAccount+"/mycontainer" {
		t.Errorf("request path = %q, want the account-prefixed Azurite path", gotPath)
	}
}

// ---------------------------------------------------------------------------
// SAS vectors
// ---------------------------------------------------------------------------

// TestServiceSASBlobVector compares BuildServiceSAS against a hand-written
// service-SAS string-to-sign (2020-12-06+ layout: 16 fields, rsct last, no
// trailing newline). Only the HMAC is computed here.
func TestServiceSASBlobVector(t *testing.T) {
	want := "r\n" + // signedPermissions
		"\n" + // signedStart
		"2026-07-17T10:00:00Z\n" + // signedExpiry
		"/blob/" + testAccount + "/mycontainer/hello.txt\n" + // canonicalizedResource (DECODED names)
		"\n" + // signedIdentifier
		"\n" + // signedIP
		"\n" + // signedProtocol
		APIVersion + "\n" + // signedVersion
		"b\n" + // signedResource
		"\n" + // signedSnapshotTime
		"\n" + // signedEncryptionScope
		"\n" + // rscc
		"\n" + // rscd
		"\n" + // rsce
		"\n" + // rscl
		"" // rsct — final field, no trailing newline

	a := testAuth(t, "")
	token, err := BuildServiceSAS(a, SASParams{
		Resource:    "b",
		Container:   "mycontainer",
		Blob:        "hello.txt",
		Permissions: "r",
		Expiry:      time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC),
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
		"sv": APIVersion, "sr": "b", "sp": "r", "se": "2026-07-17T10:00:00Z",
	} {
		if got := q.Get(param); got != want {
			t.Errorf("%s = %q, want %q", param, got, want)
		}
	}
	if q.Get("st") != "" || q.Get("sip") != "" || q.Get("rscd") != "" {
		t.Errorf("unset optional fields must be omitted from the token, got %q", token)
	}
}

// TestServiceSASContainerVector covers the container-resource shape (sr=c, no
// blob path segment) plus the optional start/IP/disposition fields.
func TestServiceSASContainerVector(t *testing.T) {
	want := "rl\n" +
		"2026-07-16T09:00:00Z\n" +
		"2026-07-17T10:00:00Z\n" +
		"/blob/" + testAccount + "/mycontainer\n" +
		"\n" +
		"10.0.0.1-10.0.0.9\n" +
		"\n" +
		APIVersion + "\n" +
		"c\n" +
		"\n\n\n" +
		"attachment\n" +
		"\n\n" +
		""

	a := testAuth(t, "")
	token, err := BuildServiceSAS(a, SASParams{
		Resource:           "c",
		Container:          "mycontainer",
		Permissions:        "rl",
		Start:              time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC),
		Expiry:             time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC),
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
		"sr": "c", "sp": "rl", "st": "2026-07-16T09:00:00Z",
		"sip": "10.0.0.1-10.0.0.9", "rscd": "attachment",
	} {
		if got := q.Get(param); got != want {
			t.Errorf("%s = %q, want %q", param, got, want)
		}
	}
}

func TestBuildServiceSASRequiresSharedKey(t *testing.T) {
	a := Auth{AccountName: testAccount, Method: AuthEntra}
	if _, err := BuildServiceSAS(a, SASParams{Resource: "b", Container: "c", Blob: "b", Permissions: "r", Expiry: fixedNow}); err == nil {
		t.Fatal("expected an error for Entra auth — a service SAS needs the account key")
	}
}

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

func TestCanonicalizedHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("x-ms-version", "2023-11-03")
	h.Set("x-ms-date", "d")
	h.Set("x-ms-meta-b", " padded ")
	h.Set("x-ms-blob-type", "BlockBlob")
	h.Set("Content-Type", "text/plain") // not x-ms-*, excluded
	got := canonicalizedHeaders(h)
	want := "x-ms-blob-type:BlockBlob\n" +
		"x-ms-date:d\n" +
		"x-ms-meta-b:padded\n" +
		"x-ms-version:2023-11-03\n"
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

// TestDoRejectsABodyOverTheCap — the cap must FAIL, not clip. A LimitReader at
// the cap returns io.EOF with no error, so an oversized blob would otherwise
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
		Method: http.MethodGet, Path: BlobPath("c", "at-cap.bin"), Query: url.Values{}, MaxBody: bodyCap,
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
		Method: http.MethodGet, Path: BlobPath("c", "over-cap.bin"), Query: url.Values{}, MaxBody: bodyCap,
	})
	if err == nil {
		t.Fatal("a body past the cap must error — silently truncating it reports success on corrupt data")
	}
	for _, want := range []string{"1 MB", "Byte Range", "Copy Blob"} {
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

// TestDoReturnsBlobBytesAsStored — Content-Encoding is a STORED property of a
// blob, not a transfer encoding. If net/http is allowed to offer gzip it
// unwraps the blob on arrival and strips Content-Encoding/Content-Length, so a
// download returns different bytes than were uploaded — and disagrees with the
// same blob fetched with a Range header, which net/http leaves alone.
func TestDoReturnsBlobBytesAsStored(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(`{"event":"stored gzip-encoded"}`)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	stored := buf.Bytes()

	var gotAcceptEncoding string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAcceptEncoding = r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(stored)))
		_, _ = w.Write(stored)
	}))
	defer srv.Close()

	resp, err := Do(&core.Flow{}, testAuth(t, srv.URL), Request{
		Method: http.MethodGet, Path: BlobPath("c", "logs.json.gz"), Query: url.Values{}, MaxBody: MaxDownloadBody,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAcceptEncoding != "" {
		t.Errorf("Accept-Encoding = %q, want none — offering gzip makes net/http unwrap the blob's stored encoding", gotAcceptEncoding)
	}
	if !bytes.Equal(resp.Body, stored) {
		t.Errorf("body = %d bytes, want the %d stored bytes verbatim", len(resp.Body), len(stored))
	}
	if resp.Headers.Get("Content-Encoding") != "gzip" {
		t.Errorf("Content-Encoding = %q, want the stored property intact", resp.Headers.Get("Content-Encoding"))
	}
	if got := resp.Headers.Get("Content-Length"); got != strconv.Itoa(len(stored)) {
		t.Errorf("Content-Length = %q, want %d", got, len(stored))
	}
}

// ---------------------------------------------------------------------------
// Error envelope & redaction
// ---------------------------------------------------------------------------

func TestCheckResponse(t *testing.T) {
	if err := CheckResponse(&APIResponse{StatusCode: 201}); err != nil {
		t.Errorf("2xx must pass, got %v", err)
	}

	xmlBody := []byte(`<?xml version="1.0" encoding="utf-8"?><Error><Code>BlobNotFound</Code><Message>The specified blob does not exist.
RequestId:xyz</Message></Error>`)
	err := CheckResponse(&APIResponse{StatusCode: 404, Body: xmlBody, Headers: http.Header{}})
	if err == nil || !strings.Contains(err.Error(), "BlobNotFound: The specified blob does not exist.") {
		t.Errorf("XML error not surfaced: %v", err)
	}
	if strings.Contains(err.Error(), "RequestId") {
		t.Errorf("RequestId noise must be trimmed: %v", err)
	}

	// HEAD failures have no body — the x-ms-error-code header is the fallback.
	h := http.Header{}
	h.Set("x-ms-error-code", "BlobNotFound")
	err = CheckResponse(&APIResponse{StatusCode: 404, Headers: h})
	if err == nil || !strings.Contains(err.Error(), "BlobNotFound") {
		t.Errorf("header fallback not surfaced: %v", err)
	}

	if code := ErrorCode(&APIResponse{StatusCode: 404, Body: xmlBody, Headers: http.Header{}}); code != "BlobNotFound" {
		t.Errorf("ErrorCode = %q", code)
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

func TestEnumerationResultsParsing(t *testing.T) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<EnumerationResults ServiceEndpoint="https://x" ContainerName="c">
  <Blobs>
    <Blob>
      <Name>a.txt</Name>
      <Properties>
        <Last-Modified>Thu, 16 Jul 2026 10:00:00 GMT</Last-Modified>
        <Etag>0x8D</Etag>
        <Content-Length>1024</Content-Length>
        <Content-Type>text/plain</Content-Type>
        <BlobType>BlockBlob</BlobType>
        <AccessTier>Hot</AccessTier>
        <ServerEncrypted>true</ServerEncrypted>
      </Properties>
      <Metadata><owner>ops</owner></Metadata>
      <Tags><TagSet><Tag><Key>project</Key><Value>alpha</Value></Tag></TagSet></Tags>
    </Blob>
  </Blobs>
  <NextMarker>cursor-1</NextMarker>
</EnumerationResults>`
	var env EnumerationResults
	if err := xml.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.NextMarker != "cursor-1" || len(env.Blobs) != 1 {
		t.Fatalf("envelope = %+v", env)
	}
	m := BlobMap(env.Blobs[0])
	props := m["properties"].(map[string]interface{})
	if props["contentLength"] != int64(1024) {
		t.Errorf("contentLength = %#v, want int64(1024)", props["contentLength"])
	}
	if props["serverEncrypted"] != true {
		t.Errorf("serverEncrypted = %#v, want true", props["serverEncrypted"])
	}
	if props["blobType"] != "BlockBlob" || props["lastModified"] == nil {
		t.Errorf("camelCased properties missing: %#v", props)
	}
	if m["tags"].(map[string]interface{})["project"] != "alpha" {
		t.Errorf("tags = %#v", m["tags"])
	}
	if m["metadata"].(map[string]interface{})["owner"] != "ops" {
		t.Errorf("metadata = %#v", m["metadata"])
	}
}

// TestListMetadataMatchesTheHeaderPath is the shape contract between the two
// ways a flow can read the same metadata: the list envelope (<Metadata> XML)
// and the properties HEAD (x-ms-meta-* headers). They must agree, or a filter
// written against one silently matches nothing on the other.
//
// The two transforms <Properties> needs are both corrupting here: camelKey
// mangles the operator's name (ORDER_ID → oRDER_ID) and coerceScalar eats a
// zero-padded reference ("0012" → 12).
func TestListMetadataMatchesTheHeaderPath(t *testing.T) {
	const body = `<?xml version="1.0" encoding="utf-8"?>
<EnumerationResults>
  <Containers>
    <Container>
      <Name>c</Name>
      <Metadata><Project_Code>0012</Project_Code><ORDER_ID>00123</ORDER_ID><Archived>true</Archived></Metadata>
    </Container>
  </Containers>
  <Blobs>
    <Blob>
      <Name>a.txt</Name>
      <Properties><Content-Length>1024</Content-Length></Properties>
      <Metadata><Project_Code>0012</Project_Code><ORDER_ID>00123</ORDER_ID><Archived>true</Archived></Metadata>
    </Blob>
  </Blobs>
</EnumerationResults>`

	var env EnumerationResults
	if err := xml.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The same metadata as the service returns it on a properties HEAD.
	h := http.Header{}
	h.Set("x-ms-meta-Project_Code", "0012")
	h.Set("x-ms-meta-ORDER_ID", "00123")
	h.Set("x-ms-meta-Archived", "true")
	want := HeadersResult("a.txt", h)["metadata"]

	if got := want.(map[string]interface{}); got["project_code"] != "0012" {
		t.Fatalf("the header path itself changed shape: %#v", got)
	}
	if got := BlobMap(env.Blobs[0])["metadata"]; !reflect.DeepEqual(got, want) {
		t.Errorf("blob_get_all metadata disagrees with blob_get_properties:\n got: %#v\nwant: %#v", got, want)
	}
	if got := ContainerMap(env.Containers[0])["metadata"]; !reflect.DeepEqual(got, want) {
		t.Errorf("container_get_all metadata disagrees with the header path:\n got: %#v\nwant: %#v", got, want)
	}

	// <Properties> keeps the camelKey + coercion the header path also applies.
	if props := BlobMap(env.Blobs[0])["properties"].(map[string]interface{}); props["contentLength"] != int64(1024) {
		t.Errorf("properties = %#v, want contentLength coerced to int64", props)
	}
}

func TestTagsHelpers(t *testing.T) {
	tags := map[string]string{"b key": "v&1", "a": "x"}
	if got, want := TagsHeaderValue(tags), "a=x&b+key=v%261"; got != want {
		t.Errorf("TagsHeaderValue = %q, want %q", got, want)
	}

	var doc TagsDocument
	if err := xml.Unmarshal(TagsXMLBody(map[string]string{"project": "alpha", "env": "dev"}), &doc); err != nil {
		t.Fatalf("TagsXMLBody does not round-trip: %v", err)
	}
	m := doc.TagsMap()
	if m["project"] != "alpha" || m["env"] != "dev" {
		t.Errorf("round-tripped tags = %#v", m)
	}

	if err := ValidateTags(map[string]string{"ok": "fine", "bad\"key": "v"}); err == nil {
		t.Error("invalid tag charset must be rejected")
	}
	big := map[string]string{}
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"} {
		big[k] = "v"
	}
	if err := ValidateTags(big); err == nil {
		t.Error("more than 10 tags must be rejected")
	}
}

func TestHeadersResult(t *testing.T) {
	h := http.Header{}
	h.Set("ETag", `"0x8D"`)
	h.Set("Last-Modified", "Thu, 16 Jul 2026 10:00:00 GMT")
	h.Set("Content-Length", "42")
	h.Set("Content-Type", "text/plain")
	h.Set("x-ms-blob-type", "BlockBlob")
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
	if props["blobType"] != "BlockBlob" || props["leaseStatus"] != "unlocked" {
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
			_, _ = w.Write([]byte(`<EnumerationResults><Containers><Container><Name>one</Name></Container></Containers><NextMarker>m2</NextMarker></EnumerationResults>`))
			return
		}
		_, _ = w.Write([]byte(`<EnumerationResults><Containers><Container><Name>two</Name></Container></Containers><NextMarker></NextMarker></EnumerationResults>`))
	}))
	defer srv.Close()

	a := testAuth(t, srv.URL)
	containers, _, truncated, err := ListEnumeration(&core.Flow{}, a, "/", url.Values{"comp": []string{"list"}}, true, 50)
	if err != nil {
		t.Fatalf("ListEnumeration: %v", err)
	}
	if truncated {
		t.Error("two pages must not report truncation")
	}
	if len(containers) != 2 || containers[0].Name != "one" || containers[1].Name != "two" {
		t.Errorf("containers = %+v", containers)
	}
	if len(markers) != 2 || markers[1] != "m2" {
		t.Errorf("marker round-trip = %v", markers)
	}

	markers = nil
	containers, _, _, err = ListEnumeration(&core.Flow{}, a, "/", url.Values{"comp": []string{"list"}}, false, 50)
	if err != nil {
		t.Fatalf("single page: %v", err)
	}
	if len(containers) != 1 || len(markers) != 1 {
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
