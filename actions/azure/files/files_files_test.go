package files_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	core "flomation.app/automate/executor"
	files "flomation.app/automate/executor/actions/azure/files"

	file_copy "flomation.app/automate/executor/actions/azure/files/file_copy"
	file_delete "flomation.app/automate/executor/actions/azure/files/file_delete"
	file_download "flomation.app/automate/executor/actions/azure/files/file_download"
	file_generate_sas "flomation.app/automate/executor/actions/azure/files/file_generate_sas"
	file_get_properties "flomation.app/automate/executor/actions/azure/files/file_get_properties"
	file_lease "flomation.app/automate/executor/actions/azure/files/file_lease"
	file_list_ranges "flomation.app/automate/executor/actions/azure/files/file_list_ranges"
	file_set_metadata "flomation.app/automate/executor/actions/azure/files/file_set_metadata"
	file_upload "flomation.app/automate/executor/actions/azure/files/file_upload"
)

// ---------------------------------------------------------------------------
// file_upload — the two-step write
// ---------------------------------------------------------------------------

// uploadCall is one captured request against the fake File service.
type uploadCall struct {
	Method        string
	Query         string
	ContentLength string // x-ms-content-length (Create File)
	ContentType   string // x-ms-content-type (Create File)
	Write         string // x-ms-write (Put Range)
	Range         string // Range (Put Range)
	IfNoneMatch   string
	Body          []byte
}

// uploadServer is a fake File service that records the call sequence and
// reassembles the ranges into the bytes that actually landed. Nothing else in
// this file can answer the question that matters — a Create with no Put Range
// leaves a file of exactly the right SIZE and none of the right bytes.
type uploadServer struct {
	mu         sync.Mutex
	calls      []uploadCall
	stored     []byte // the sparse file: allocated by Create, filled by Put Range
	rangeCount int
	failRange  int // 1-based index of the Put Range to fail; 0 never fails
	deleted    bool
}

func newUploadServer(t *testing.T, failRange int) (*uploadServer, *httptest.Server) {
	t.Helper()
	u := &uploadServer{failRange: failRange}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		u.mu.Lock()
		defer u.mu.Unlock()
		call := uploadCall{
			Method:        r.Method,
			Query:         r.URL.RawQuery,
			ContentLength: r.Header.Get("x-ms-content-length"),
			ContentType:   r.Header.Get("x-ms-content-type"),
			Write:         r.Header.Get("x-ms-write"),
			Range:         r.Header.Get("Range"),
			IfNoneMatch:   r.Header.Get("If-None-Match"),
			Body:          body,
		}
		u.calls = append(u.calls, call)

		switch {
		case r.Method == http.MethodDelete:
			u.deleted = true
			w.WriteHeader(http.StatusAccepted)
		case call.Query == "comp=range":
			u.rangeCount++
			if u.failRange > 0 && u.rangeCount == u.failRange {
				azureError(w, http.StatusInternalServerError, "InternalError", "The server encountered an internal error.")
				return
			}
			// Write the bytes where the Range header says they go. A signer or
			// a loop that got the offsets wrong shows up here as corruption
			// rather than as a passing test.
			var start, end int
			if _, err := fmt.Sscanf(call.Range, "bytes=%d-%d", &start, &end); err != nil {
				t.Errorf("unparseable Range header %q", call.Range)
			}
			if end-start+1 != len(body) {
				t.Errorf("Range %q covers %d bytes but the body carries %d", call.Range, end-start+1, len(body))
			}
			copy(u.stored[start:end+1], body)
			w.Header().Set("ETag", `"0xRANGE"`)
			w.WriteHeader(http.StatusCreated)
		default: // Create File
			n, _ := strconv.Atoi(call.ContentLength)
			// The service allocates a SPARSE file: N bytes of zeros, no content.
			u.stored = make([]byte, n)
			w.Header().Set("ETag", `"0xCREATE"`)
			w.WriteHeader(http.StatusCreated)
		}
	}))
	return u, srv
}

// content builds an upload payload of an exact byte length that cannot be
// mistaken for base64 (the "!" defeats the resolver's sniff), so the test
// controls the size to the byte.
func content(n int) string {
	return strings.Repeat("x", n-1) + "!"
}

// TestFileUploadIsTwoSteps is the single most important test in this package.
//
// Unlike blob_upload's one Put Blob, an Azure Files upload is Create File
// (allocate N zero bytes) THEN Put Range (write them). Skipping step 2 leaves a
// correctly sized, zero-filled file that looks like a success to anything that
// checks the size — so the assertion is on the BYTES that landed, not on the
// status code.
func TestFileUploadIsTwoSteps(t *testing.T) {
	u, srv := newUploadServer(t, 0)
	defer srv.Close()

	body := content(11)
	out, err := file_upload.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"),
		str("directory", "reports"),
		str("file_name", "notes.txt"),
		text("content", body),
		str("content_type", "text/plain"),
		obj("metadata", `{"source":"flomation"}`),
	))
	result := wantSuccess(t, out, err)

	if len(u.calls) != 2 {
		t.Fatalf("made %d calls, want exactly 2 (Create File then Put Range): %+v", len(u.calls), u.calls)
	}

	// Step 1 — Create File declares the size and writes nothing.
	create := u.calls[0]
	if create.Method != http.MethodPut || create.Query != "" {
		t.Errorf("step 1 = %s ?%s, want a bare PUT", create.Method, create.Query)
	}
	if create.ContentLength != "11" {
		t.Errorf("step 1 x-ms-content-length = %q, want 11 — this is what allocates the file", create.ContentLength)
	}
	if create.ContentType != "text/plain" {
		t.Errorf("step 1 x-ms-content-type = %q — the request's own Content-Type describes a body that does not exist yet", create.ContentType)
	}
	if len(create.Body) != 0 {
		t.Errorf("step 1 carried a %d-byte body — Create File allocates, it does not write", len(create.Body))
	}

	// Step 2 — Put Range writes the bytes.
	put := u.calls[1]
	if put.Query != "comp=range" || put.Write != "update" || put.Range != "bytes=0-10" {
		t.Errorf("step 2 = ?%s x-ms-write=%q Range=%q, want comp=range update bytes=0-10", put.Query, put.Write, put.Range)
	}

	// And the file on the "service" is the file we meant to upload.
	if string(u.stored) != body {
		t.Errorf("stored bytes = %q, want %q — a Create with no Put Range yields the right SIZE and none of the right bytes", u.stored, body)
	}
	if out["size"] != 11 || result["ranges"] != 1 {
		t.Errorf("size = %v, ranges = %v", out["size"], result["ranges"])
	}
	if out["etag"] != `"0xRANGE"` {
		t.Errorf("etag = %v, want the one the last write returned", out["etag"])
	}
	if out["url"] != srv.URL+"/my-share/reports/notes.txt" {
		t.Errorf("url = %v", out["url"])
	}
}

// TestFileUploadChunksAtTheRangeCap pins the boundary. One Put Range is capped
// at 4 MiB — "if you attempt to upload a range that's larger than 4 MiB, the
// service returns status code 413" — so the cap itself is one range and one byte
// past it is two. Both reassemble byte-for-byte.
func TestFileUploadChunksAtTheRangeCap(t *testing.T) {
	for _, tc := range []struct {
		name       string
		size       int
		wantRanges int
		wantFirst  string
		wantLast   string
	}{
		{"one byte under the cap", files.MaxRangeBytes - 1, 1, "bytes=0-4194302", "bytes=0-4194302"},
		{"exactly the cap", files.MaxRangeBytes, 1, "bytes=0-4194303", "bytes=0-4194303"},
		{"one byte over the cap", files.MaxRangeBytes + 1, 2, "bytes=0-4194303", "bytes=4194304-4194304"},
		{"two full ranges plus a tail", 2*files.MaxRangeBytes + 10, 3, "bytes=0-4194303", "bytes=8388608-8388617"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, srv := newUploadServer(t, 0)
			defer srv.Close()

			body := content(tc.size)
			out, err := file_upload.Execute(&core.Flow{}, nil, authInputs(srv.URL,
				str("share", "my-share"), str("file_name", "big.bin"), text("content", body)))
			result := wantSuccess(t, out, err)

			ranges := u.calls[1:]
			if len(ranges) != tc.wantRanges {
				t.Fatalf("made %d Put Range calls for %d bytes, want %d", len(ranges), tc.size, tc.wantRanges)
			}
			if ranges[0].Range != tc.wantFirst {
				t.Errorf("first range = %q, want %q", ranges[0].Range, tc.wantFirst)
			}
			if last := ranges[len(ranges)-1].Range; last != tc.wantLast {
				t.Errorf("last range = %q, want %q", last, tc.wantLast)
			}
			for _, r := range ranges {
				if len(r.Body) > files.MaxRangeBytes {
					t.Errorf("a range carried %d bytes, over the %d cap — the service answers 413", len(r.Body), files.MaxRangeBytes)
				}
			}
			// The reassembled file is the whole point of the loop.
			if string(u.stored) != body {
				t.Errorf("stored %d bytes, want the %d uploaded, byte-for-byte", len(u.stored), len(body))
			}
			if result["ranges"] != tc.wantRanges {
				t.Errorf("reported ranges = %v, want %d", result["ranges"], tc.wantRanges)
			}
		})
	}
}

// TestFileUploadCleansUpAfterAFailedWrite — Create-then-failed-Put-Range is NOT
// atomic and leaves a correctly sized, zero-filled file behind: a corruption
// blob_upload cannot produce. The failure must remove it and say so.
func TestFileUploadCleansUpAfterAFailedWrite(t *testing.T) {
	u, srv := newUploadServer(t, 1)
	defer srv.Close()

	out, err := file_upload.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("file_name", "notes.txt"), text("content", content(11))))
	wantSoftFailure(t, out, err, "failed to write the file's content")
	wantNoSecretLeak(t, out)

	if !u.deleted {
		t.Error("a failed Put Range must delete the sparse file it allocated — otherwise a zero-filled file of the right size is left behind")
	}
	if !strings.Contains(out["error"].(string), "was removed") {
		t.Errorf("error %q must say what happened to the half-written file", out["error"])
	}
}

// TestFileUploadWarnsWhenCleanupFails — best-effort means best-effort. If the
// delete also fails, the message must not claim a cleanliness it cannot verify.
func TestFileUploadWarnsWhenCleanupFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete, r.URL.RawQuery == "comp=range":
			azureError(w, http.StatusInternalServerError, "InternalError", "The server encountered an internal error.")
		default:
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()

	out, err := file_upload.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("file_name", "notes.txt"), text("content", content(11))))
	wantSoftFailure(t, out, err, "WARNING")
	if !strings.Contains(out["error"].(string), "left behind") {
		t.Errorf("error %q must warn that the zero-filled file is still there", out["error"])
	}
}

func TestFileUploadOverwriteOff(t *testing.T) {
	u, srv := newUploadServer(t, 0)
	defer srv.Close()

	// Overwrite on (the default) sends no condition.
	if _, err := file_upload.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "a.txt"), text("content", content(4)))); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if u.calls[0].IfNoneMatch != "" {
		t.Errorf("If-None-Match = %q with Overwrite on, want none", u.calls[0].IfNoneMatch)
	}

	// Off makes the create conditional on absence.
	u2, srv2 := newUploadServer(t, 0)
	defer srv2.Close()
	if _, err := file_upload.Execute(&core.Flow{}, nil, authInputs(srv2.URL,
		str("share", "s"), str("file_name", "a.txt"), text("content", content(4)), boolean("overwrite", false))); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if u2.calls[0].IfNoneMatch != "*" {
		t.Errorf("If-None-Match = %q with Overwrite off, want *", u2.calls[0].IfNoneMatch)
	}
}

func TestFileUploadErrors(t *testing.T) {
	exists := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusConflict, "ResourceAlreadyExists", "The specified resource already exists.")
	}))
	defer exists.Close()

	out, err := file_upload.Execute(&core.Flow{}, nil, authInputs(exists.URL,
		str("share", "my-share"), str("file_name", "a.txt"), text("content", content(4)), boolean("overwrite", false)))
	wantSoftFailure(t, out, err, "already exists")

	// Azure Files has no implicit directories — a blob-shaped mental model
	// ("reports/2026/a.txt just works") lands here, so the message names the fix.
	orphan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusNotFound, "ParentNotFound", "The specified parent path does not exist.")
	}))
	defer orphan.Close()

	out, err = file_upload.Execute(&core.Flow{}, nil, authInputs(orphan.URL,
		str("share", "my-share"), str("directory", "reports/2026"), str("file_name", "a.txt"), text("content", content(4))))
	wantSoftFailure(t, out, err, "no implicit directories")

	out, err = file_upload.Execute(&core.Flow{}, nil, authInputs(orphan.URL,
		str("share", "my-share"), str("file_name", "a.txt")))
	wantSoftFailure(t, out, err, "content is required")
}

// ---------------------------------------------------------------------------
// file_download
// ---------------------------------------------------------------------------

func TestFileDownload(t *testing.T) {
	// MediaScratchFile writes under the working directory, so keep the test's
	// output out of the repo.
	t.Chdir(t.TempDir())

	const payload = `{"event":"hello"}`
	var gotRange string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-ms-meta-owner", "ops")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	out, err := file_download.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("directory", "reports"), str("file_name", "a.json")))
	wantSuccess(t, out, err)

	if gotRange != "" {
		t.Errorf("Range = %q with no byte range set, want none", gotRange)
	}
	if out["content"] != payload {
		t.Errorf("content = %q, want the JSON inline (text-like and under 1 MB)", out["content"])
	}
	if out["content_type"] != "application/json" || out["size"] != len(payload) {
		t.Errorf("content_type = %v, size = %v", out["content_type"], out["size"])
	}
	if out["file"] == "" || out["file"] == nil {
		t.Error("file reference missing — it is how downstream nodes take the bytes")
	}
	if out["id"] != "reports/a.json" {
		t.Errorf("id = %v, want the logical path", out["id"])
	}
}

// TestFileDownloadBinaryIsNotInlined — the inline output is for text a flow can
// read; binary travels only as the file reference.
func TestFileDownloadBinaryIsNotInlined(t *testing.T) {
	t.Chdir(t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0x00, 0x01, 0x02})
	}))
	defer srv.Close()

	out, err := file_download.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "a.bin")))
	wantSuccess(t, out, err)
	if out["content"] != "" {
		t.Errorf("content = %q, want empty for a binary file", out["content"])
	}
}

func TestFileDownloadErrors(t *testing.T) {
	t.Chdir(t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusNotFound, "ResourceNotFound", "The specified resource does not exist.")
	}))
	defer srv.Close()

	out, err := file_download.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "gone.txt")))
	wantSoftFailure(t, out, err, "ResourceNotFound")
	wantNoSecretLeak(t, out)

	out, err = file_download.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "a.txt"), str("range", "0-10")))
	wantSoftFailure(t, out, err, `must look like "bytes=0-1023"`)
}

// ---------------------------------------------------------------------------
// file_delete / file_get_properties / file_set_metadata
// ---------------------------------------------------------------------------

func TestFileDelete(t *testing.T) {
	var gotMethod, gotPath, gotLease string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotLease = r.Method, r.URL.Path, r.Header.Get("x-ms-lease-id")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := file_delete.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("directory", "reports"), str("file_name", "a.txt"),
		str("lease_id", "lease-guid")))
	result := wantSuccess(t, out, err)
	if gotMethod != http.MethodDelete || gotPath != "/my-share/reports/a.txt" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotLease != "lease-guid" {
		t.Errorf("x-ms-lease-id = %q — without it a write to a leased file is refused", gotLease)
	}
	if result["deleted"] != true {
		t.Errorf("result = %#v", result)
	}
}

func TestFileDeleteError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusPreconditionFailed, "LeaseIdMissing", "There is currently a lease on the file.")
	}))
	defer srv.Close()

	out, err := file_delete.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "a.txt")))
	wantSoftFailure(t, out, err, "LeaseIdMissing")
}

func TestFileGetProperties(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Length", "2048")
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("x-ms-type", "File")
		w.Header().Set("x-ms-meta-owner", "ops")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := file_get_properties.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("directory", "reports"), str("file_name", "a.pdf")))
	result := wantSuccess(t, out, err)

	// HEAD, not GET: the properties are all in the headers, so pulling the
	// file's bytes to read its size would be pure waste.
	if gotMethod != http.MethodHead {
		t.Errorf("method = %s, want HEAD", gotMethod)
	}
	if out["size"] != int64(2048) {
		t.Errorf("size = %#v, want int64(2048)", out["size"])
	}
	if result["metadata"].(map[string]interface{})["owner"] != "ops" {
		t.Errorf("metadata = %#v", result["metadata"])
	}
}

func TestFileGetPropertiesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A HEAD failure carries NO body — the x-ms-error-code header is the
		// only thing to go on.
		w.Header().Set("x-ms-error-code", "ResourceNotFound")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	out, err := file_get_properties.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "gone.txt")))
	wantSoftFailure(t, out, err, "ResourceNotFound")
}

func TestFileSetMetadata(t *testing.T) {
	var gotQuery, gotOwner string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery, gotOwner = r.URL.RawQuery, r.Header.Get("x-ms-meta-owner")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := file_set_metadata.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "a.txt"), obj("metadata", `{"owner":"ops"}`)))
	wantSuccess(t, out, err)
	if gotQuery != "comp=metadata" || gotOwner != "ops" {
		t.Errorf("request = ?%s with owner %q", gotQuery, gotOwner)
	}
}

func TestFileSetMetadataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := file_set_metadata.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "a.txt")))
	wantSoftFailure(t, out, err, "metadata is required")
}

// ---------------------------------------------------------------------------
// file_copy
// ---------------------------------------------------------------------------

func TestFileCopySameAccountAndPolls(t *testing.T) {
	var gotSource string
	var heads int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			heads++
			w.Header().Set("x-ms-copy-status", "success")
			w.WriteHeader(http.StatusOK)
			return
		}
		gotSource = r.Header.Get("x-ms-copy-source")
		w.Header().Set("x-ms-copy-id", "copy-1")
		w.Header().Set("x-ms-copy-status", "pending")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := file_copy.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "dest-share"), str("file_name", "copy.pdf"),
		str("source_share", "src-share"), str("source_path", "reports/a.pdf")))
	result := wantSuccess(t, out, err)

	// A same-account source needs no SAS — the destination request's own
	// authorization covers reading it.
	if gotSource != srv.URL+"/src-share/reports/a.pdf" {
		t.Errorf("x-ms-copy-source = %q", gotSource)
	}
	if out["copy_status"] != "success" || out["copy_id"] != "copy-1" {
		t.Errorf("copy_status = %v, copy_id = %v", out["copy_status"], out["copy_id"])
	}
	if heads == 0 {
		t.Error("wait_for_completion defaults on — a pending copy must be polled")
	}
	if result["sourceShare"] != "src-share" {
		t.Errorf("result = %#v", result)
	}
}

// TestFileCopyRedactsTheSourceSAS — a source_url carrying a SAS would otherwise
// put a live sig= into the run record and every downstream node. The output is
// as much of a leak as the error string.
func TestFileCopyRedactsTheSourceSAS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-copy-status", "success")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := file_copy.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "dest"), str("file_name", "a.pdf"),
		str("source_url", "https://other.file.core.windows.net/s/a.pdf?sv=2023-11-03&sp=r&sig=LIVESIGNATURE")))
	result := wantSuccess(t, out, err)

	source := result["source"].(string)
	if strings.Contains(source, "LIVESIGNATURE") {
		t.Errorf("the source SAS signature survived into the output: %q", source)
	}
	if !strings.Contains(source, "sig=REDACTED") || !strings.Contains(source, "sp=r") {
		t.Errorf("source = %q — the signature goes, the provenance stays", source)
	}
}

func TestFileCopyErrors(t *testing.T) {
	// A copy is async: the PUT only starts it, and the reason it failed arrives
	// on the polled HEAD. Reporting "failed" without that description would hand
	// the operator a dead end.
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("x-ms-copy-status", "failed")
			w.Header().Set("x-ms-copy-status-description", "the source could not be read")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("x-ms-copy-status", "pending")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer failed.Close()

	out, err := file_copy.Execute(&core.Flow{}, nil, authInputs(failed.URL,
		str("share", "dest"), str("file_name", "a.pdf"), str("source_url", "https://x/y")))
	wantSoftFailure(t, out, err, "the source could not be read")

	// The two source forms are mutually exclusive, and neither is optional.
	for name, extra := range map[string][]*core.Connection{
		"both forms":                 {str("source_url", "https://x/y"), str("source_share", "s"), str("source_path", "a")},
		"neither":                    {},
		"half a same-account source": {str("source_share", "s")},
		"bad url":                    {str("source_url", "ftp://x/y")},
	} {
		inputs := append(authInputs(failed.URL, str("share", "dest"), str("file_name", "a.pdf")), extra...)
		out, err := file_copy.Execute(&core.Flow{}, nil, inputs)
		if err != nil || out["success"] != false {
			t.Errorf("%s: expected a soft failure, got %v / %v", name, out, err)
		}
	}
}

// ---------------------------------------------------------------------------
// file_generate_sas
// ---------------------------------------------------------------------------

func TestFileGenerateSAS(t *testing.T) {
	out, err := file_generate_sas.Execute(&core.Flow{}, nil, authInputs("",
		str("share", "my-share"), str("path", "reports/a.pdf"), str("permissions", "rw"), integer("expiry_hours", 1)))
	wantSuccess(t, out, err)

	token := out["sas_token"].(string)
	for _, want := range []string{"sr=f", "sp=rw", "sig=", "sv="} {
		if !strings.Contains(token, want) {
			t.Errorf("token %q missing %q", token, want)
		}
	}
	url := out["sas_url"].(string)
	if !strings.HasPrefix(url, "https://devstoreaccount1.file.core.windows.net/my-share/reports/a.pdf?") {
		t.Errorf("sas_url = %q, want the .file. host and the escaped resource path", url)
	}
	if out["expires_at"] == "" {
		t.Error("expires_at missing")
	}
}

func TestFileGenerateSASShare(t *testing.T) {
	out, err := file_generate_sas.Execute(&core.Flow{}, nil, authInputs("",
		str("share", "my-share"), str("resource", "share"), str("permissions", "rl")))
	wantSuccess(t, out, err)
	if !strings.Contains(out["sas_token"].(string), "sr=s") {
		t.Errorf("token = %q, want sr=s for a share SAS", out["sas_token"])
	}
}

func TestFileGenerateSASErrors(t *testing.T) {
	// Entra has no account key to sign with — the same refusal blob_generate_sas
	// makes, and on Files the OAuth path would carry the ACL bypass besides.
	entra := []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "auth_method", Type: core.ConnectionTypeString, Value: "entra"},
		{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Value: "t"},
		{Name: "azure_client_id", Type: core.ConnectionTypeString, Value: "c"},
		{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Value: "s"},
		{Name: "share", Type: core.ConnectionTypeString, Value: "my-share"},
		{Name: "path", Type: core.ConnectionTypeString, Value: "a.pdf"},
	}
	out, err := file_generate_sas.Execute(&core.Flow{}, nil, entra)
	wantSoftFailure(t, out, err, "requires Shared Key auth")

	for name, extra := range map[string][]*core.Connection{
		// "a" (append) is a BLOB permission; the Files alphabet is rcwdl.
		"blob permission":    {str("path", "a.pdf"), str("permissions", "a")},
		"out of order":       {str("path", "a.pdf"), str("permissions", "wr")},
		"list on a file":     {str("path", "a.pdf"), str("permissions", "rl")},
		"missing path":       {},
		"expiry in the past": {str("path", "a.pdf"), str("expiry", "2020-01-01T00:00:00Z")},
		"unparseable expiry": {str("path", "a.pdf"), str("expiry", "next tuesday")},
		"bad resource":       {str("resource", "directory"), str("path", "a.pdf")},
	} {
		inputs := append(authInputs("", str("share", "my-share")), extra...)
		out, err := file_generate_sas.Execute(&core.Flow{}, nil, inputs)
		if err != nil || out["success"] != false {
			t.Errorf("%s: expected a soft failure, got %v / %v", name, out, err)
		}
	}
}

// ---------------------------------------------------------------------------
// file_list_ranges
// ---------------------------------------------------------------------------

func TestFileListRanges(t *testing.T) {
	var gotQuery, gotRange string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery, gotRange = r.URL.RawQuery, r.Header.Get("x-ms-range")
		xmlBody(w, `<Ranges>
			<Range><Start>0</Start><End>511</End></Range>
			<Range><Start>1024</Start><End>1535</End></Range>
			<ClearRange><Start>512</Start><End>1023</End></ClearRange>
		</Ranges>`)
	}))
	defer srv.Close()

	out, err := file_list_ranges.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "sparse.bin"), str("range", "bytes=0-2047")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotQuery != "comp=rangelist" || gotRange != "bytes=0-2047" {
		t.Errorf("request = ?%s with x-ms-range %q", gotQuery, gotRange)
	}
	if out["count"] != 3 {
		t.Fatalf("count = %v, want 3 (2 written + 1 cleared)", out["count"])
	}
	results := out["results"].([]interface{})
	first := results[0].(map[string]interface{})
	if first["start"] != int64(0) || first["end"] != int64(511) || first["bytes"] != int64(512) || first["type"] != "range" {
		t.Errorf("first range = %#v", first)
	}
	// Cleared ranges are tagged, not dropped: they describe the same axis.
	if last := results[2].(map[string]interface{}); last["type"] != "clear" {
		t.Errorf("last entry = %#v, want the cleared range tagged", last)
	}
	if !strings.Contains(out["tool_result"].(string), "2 written ranges covering 1024 bytes") {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

func TestFileListRangesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusNotFound, "ResourceNotFound", "The specified resource does not exist.")
	}))
	defer srv.Close()

	out, err := file_list_ranges.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "gone.bin")))
	wantSoftFailure(t, out, err, "ResourceNotFound")

	out, err = file_list_ranges.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "a.bin"), str("range", "0-10")))
	wantSoftFailure(t, out, err, `must look like "bytes=0-1023"`)
}

// ---------------------------------------------------------------------------
// file_lease
// ---------------------------------------------------------------------------

func TestFileLeaseAcquire(t *testing.T) {
	var gotQuery, gotAction, gotDuration string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		gotAction = r.Header.Get("x-ms-lease-action")
		gotDuration = r.Header.Get("x-ms-lease-duration")
		w.Header().Set("x-ms-lease-id", "minted-guid")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out, err := file_lease.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "a.txt"), str("lease_action", "acquire")))
	result := wantSuccess(t, out, err)

	if gotQuery != "comp=lease" || gotAction != "acquire" {
		t.Errorf("request = ?%s action %q", gotQuery, gotAction)
	}
	// A FILE lease is infinite-only; the 15-60s window is a share/blob thing.
	if gotDuration != "-1" {
		t.Errorf("x-ms-lease-duration = %q, want -1 — a file lease has no finite form", gotDuration)
	}
	if out["lease_id"] != "minted-guid" {
		t.Errorf("lease_id = %v — it is the whole point of an acquire", out["lease_id"])
	}
	if result["leaseAction"] != "acquire" {
		t.Errorf("result = %#v", result)
	}
	// The infinite lease outlives the flow, so the summary has to say so.
	if !strings.Contains(out["tool_result"].(string), "release it") {
		t.Errorf("tool_result = %q, want the un-released lease called out", out["tool_result"])
	}
}

func TestFileLeaseErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusConflict, "LeaseAlreadyPresent", "There is already a lease present.")
	}))
	defer srv.Close()

	out, err := file_lease.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "a.txt"), str("lease_action", "acquire")))
	wantSoftFailure(t, out, err, "LeaseAlreadyPresent")

	// Renew is a BLOB action — an infinite lease has nothing to renew.
	out, err = file_lease.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "a.txt"), str("lease_action", "renew"), str("lease_id", "x")))
	wantSoftFailure(t, out, err, "is not supported")

	out, err = file_lease.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "s"), str("file_name", "a.txt"), str("lease_action", "release")))
	wantSoftFailure(t, out, err, "lease_id is required")
}
