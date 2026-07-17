package files_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	core "flomation.app/automate/executor"

	directory_create "flomation.app/automate/executor/actions/azure/files/directory_create"
	directory_delete "flomation.app/automate/executor/actions/azure/files/directory_delete"
	directory_get_all "flomation.app/automate/executor/actions/azure/files/directory_get_all"
	directory_get_properties "flomation.app/automate/executor/actions/azure/files/directory_get_properties"
)

// ---------------------------------------------------------------------------
// directory_create
// ---------------------------------------------------------------------------

// TestDirectoryCreateMakesParents — Azure Files has no mkdir -p. Every level of
// a nested path needs its own call, and a missing parent is a hard 404
// ParentNotFound, so the toggle is the difference between "create a directory"
// working and not.
func TestDirectoryCreateMakesParents(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	var leafMeta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		paths = append(paths, r.URL.Path)
		if r.URL.RawQuery != "restype=directory" {
			t.Errorf("query = %q, want restype=directory", r.URL.RawQuery)
		}
		if r.URL.Path == "/my-share/reports/2026/q1" {
			leafMeta = r.Header.Get("x-ms-meta-team")
		} else if m := r.Header.Get("x-ms-meta-team"); m != "" {
			t.Errorf("%s got metadata %q — it belongs to the leaf, not the scaffolding", r.URL.Path, m)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out, err := directory_create.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"),
		str("directory", "reports/2026/q1"),
		obj("metadata", `{"team":"ops"}`),
	))
	result := wantSuccess(t, out, err)

	want := []string{"/my-share/reports", "/my-share/reports/2026", "/my-share/reports/2026/q1"}
	if len(paths) != len(want) {
		t.Fatalf("made %d calls (%v), want one per level: %v", len(paths), paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("call %d = %s, want %s (parents must be created top-down)", i, paths[i], want[i])
		}
	}
	if leafMeta != "ops" {
		t.Errorf("leaf metadata = %q, want ops", leafMeta)
	}
	if result["directoriesCreated"] != 3 {
		t.Errorf("directoriesCreated = %v, want 3", result["directoriesCreated"])
	}
}

// TestDirectoryCreateTolerantOfExistingParents — an existing PARENT is exactly
// what Create Parents expects to find. An existing LEAF is not: that is what the
// operator asked to create.
func TestDirectoryCreateTolerantOfExistingParents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/my-share/reports/2026/q1" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		azureError(w, http.StatusConflict, "ResourceAlreadyExists", "The specified resource already exists.")
	}))
	defer srv.Close()

	out, err := directory_create.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("directory", "reports/2026/q1")))
	result := wantSuccess(t, out, err)
	if result["directoriesCreated"] != 1 {
		t.Errorf("directoriesCreated = %v, want 1 — the two existing parents were skipped", result["directoriesCreated"])
	}
}

func TestDirectoryCreateErrors(t *testing.T) {
	existing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusConflict, "ResourceAlreadyExists", "The specified resource already exists.")
	}))
	defer existing.Close()

	out, err := directory_create.Execute(&core.Flow{}, nil, authInputs(existing.URL,
		str("share", "my-share"), str("directory", "reports")))
	wantSoftFailure(t, out, err, `directory "reports" already exists`)
	wantNoSecretLeak(t, out)

	// With Create Parents off, a missing parent is named along with the fix.
	orphan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusNotFound, "ParentNotFound", "The specified parent path does not exist.")
	}))
	defer orphan.Close()

	out, err = directory_create.Execute(&core.Flow{}, nil, authInputs(orphan.URL,
		str("share", "my-share"), str("directory", "reports/2026/q1"), boolean("create_parents", false)))
	wantSoftFailure(t, out, err, "turn on Create Parents")

	// A reserved character is caught client-side: file and directory names take
	// a wide charset, but not these.
	out, err = directory_create.Execute(&core.Flow{}, nil, authInputs(orphan.URL,
		str("share", "my-share"), str("directory", "reports:2026")))
	wantSoftFailure(t, out, err, "reserved character")
}

// TestDirectoryCreateWithoutParentsCallsOnce pins the off switch: exactly one
// call, for the leaf.
func TestDirectoryCreateWithoutParentsCallsOnce(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/my-share/a/b/c" {
			t.Errorf("path = %s, want only the leaf", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out, err := directory_create.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("directory", "a/b/c"), boolean("create_parents", false)))
	wantSuccess(t, out, err)
	if calls != 1 {
		t.Errorf("made %d calls, want 1", calls)
	}
}

// ---------------------------------------------------------------------------
// directory_get_all
// ---------------------------------------------------------------------------

// TestDirectoryGetAll — the listing that most distinguishes this node from Blob:
// real <File> AND <Directory> entries in one envelope, each tagged so a flow can
// tell them apart without a naming convention.
func TestDirectoryGetAll(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		xmlBody(w, `<EnumerationResults ShareName="my-share" DirectoryPath="reports">
			<Entries>
				<File><Name>summary.pdf</Name><Properties><Content-Length>1024</Content-Length></Properties></File>
				<Directory><Name>2026</Name></Directory>
			</Entries>
			<NextMarker />
		</EnumerationResults>`)
	}))
	defer srv.Close()

	out, err := directory_get_all.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("directory", "reports")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["count"] != 2 {
		t.Fatalf("count = %v, want 2 (out: %v)", out["count"], out)
	}
	if gotPath != "/my-share/reports" {
		t.Errorf("path = %q", gotPath)
	}
	for _, want := range []string{"restype=directory", "comp=list"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}

	// Directories first, then files.
	results := out["results"].([]interface{})
	dir := results[0].(map[string]interface{})
	file := results[1].(map[string]interface{})
	if dir["type"] != "directory" || dir["name"] != "2026" {
		t.Errorf("first entry = %#v, want the directory", dir)
	}
	if file["type"] != "file" || file["name"] != "summary.pdf" {
		t.Errorf("second entry = %#v, want the file", file)
	}
	if !strings.Contains(out["tool_result"].(string), "1 directories, 1 files") {
		t.Errorf("tool_result = %q, want the breakdown", out["tool_result"])
	}
}

// TestDirectoryGetAllRoot — a blank directory is the share's ROOT, which the
// service addresses as the share itself.
func TestDirectoryGetAllRoot(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		xmlBody(w, `<EnumerationResults><Entries /><NextMarker /></EnumerationResults>`)
	}))
	defer srv.Close()

	out, err := directory_get_all.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "my-share")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotPath != "/my-share" {
		t.Errorf("path = %q, want the share itself for the root directory", gotPath)
	}
	if out["count"] != 0 {
		t.Errorf("count = %v, want 0", out["count"])
	}
	if _, ok := out["results"].([]interface{}); !ok {
		t.Errorf("results = %#v, want an empty array rather than nil", out["results"])
	}
}

func TestDirectoryGetAllError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusNotFound, "ResourceNotFound", "The specified resource does not exist.")
	}))
	defer srv.Close()

	out, err := directory_get_all.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("directory", "gone")))
	wantSoftFailure(t, out, err, "ResourceNotFound")
	wantNoSecretLeak(t, out)
}

// ---------------------------------------------------------------------------
// directory_delete
// ---------------------------------------------------------------------------

func TestDirectoryDelete(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := directory_delete.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("directory", "reports/2026")))
	result := wantSuccess(t, out, err)
	if gotMethod != http.MethodDelete || gotPath != "/my-share/reports/2026" || gotQuery != "restype=directory" {
		t.Errorf("request = %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	if result["deleted"] != true {
		t.Errorf("result = %#v", result)
	}
}

// TestDirectoryDeleteNotEmpty — the difference from container_delete that will
// actually bite. A container cascades; a directory does not, and the raw 409
// says only "DirectoryNotEmpty".
func TestDirectoryDeleteNotEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusConflict, "DirectoryNotEmpty", "The specified directory is not empty.")
	}))
	defer srv.Close()

	out, err := directory_delete.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("directory", "reports")))
	wantSoftFailure(t, out, err, "deletes only empty directories")
	if !strings.Contains(out["error"].(string), "List Directory") {
		t.Errorf("error %q should point at the step that shows what is left", out["error"])
	}
}

// ---------------------------------------------------------------------------
// directory_get_properties
// ---------------------------------------------------------------------------

func TestDirectoryGetProperties(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/my-share/reports" || r.URL.RawQuery != "restype=directory" {
			t.Errorf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("x-ms-file-attributes", "Directory")
		w.Header().Set("x-ms-meta-team", "ops")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := directory_get_properties.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), str("directory", "reports")))
	result := wantSuccess(t, out, err)
	if result["path"] != "reports" {
		t.Errorf("path = %v", result["path"])
	}
	if result["metadata"].(map[string]interface{})["team"] != "ops" {
		t.Errorf("metadata = %#v", result["metadata"])
	}
}

func TestDirectoryGetPropertiesRootAndError(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/my-share" {
			t.Errorf("path = %q, want the share itself for the root directory", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()

	out, err := directory_get_properties.Execute(&core.Flow{}, nil, authInputs(ok.URL, str("share", "my-share")))
	wantSuccess(t, out, err)
	if out["id"] != "/" {
		t.Errorf("id = %v, want / for the share root", out["id"])
	}

	missing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusNotFound, "ResourceNotFound", "The specified resource does not exist.")
	}))
	defer missing.Close()

	out, err = directory_get_properties.Execute(&core.Flow{}, nil, authInputs(missing.URL,
		str("share", "my-share"), str("directory", "gone")))
	wantSoftFailure(t, out, err, "ResourceNotFound")
}
