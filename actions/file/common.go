// Package file holds the category metadata and the shared workspace helpers
// every file action uses. It has no Execute function, so the manifest generator
// excludes it from the action registry.
//
// # The rule these actions enforce
//
// Every path is relative to the FLOW'S OWN WORKSPACE, and nothing can reach
// outside it. The workspace is the per-execution directory the runner creates
// (<execution_directory>/<flo_id>/<execution_id>) and sets as the executor's
// working directory, so confinement to it is also isolation between flows,
// between executions, and between tenants.
//
// # Why os.Root rather than a path check
//
// The obvious implementation — clean the path, join it to the workspace, then
// check the result still starts with the workspace — is wrong in two ways that
// matter. It does not see a SYMLINK inside the workspace pointing out of it,
// and even if it checked, the link could be swapped between the check and the
// open (TOCTOU). os.Root resolves every component against a directory handle
// and refuses anything that leaves the root, enforced by the kernel at each
// step. Verified against absolute paths, ../ traversal, a symlink to a file
// outside, and a symlink to a directory outside — all refused, for read, write,
// mkdir, remove and rename alike.
//
// This replaces the previous guard on file/read and file/write, which was
// `strings.Contains(path, "..")` and did not look at absolute paths at all. See
// AUDIT.md, 2026-09-14.
//
// # What this does NOT cover
//
// Action-level confinement binds these actions only. It is not a sandbox for
// the execution as a whole: script/bash still runs /bin/bash as the service
// user, and makefile actions still shell out. Enforcing the blast radius for
// everything needs OS-level isolation of the executor process. Tracked in
// AUDIT.md.
package file

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	// MaxFileSize caps a single read. Unchanged from the previous behaviour.
	MaxFileSize = 5 << 20 // 5 MB

	// MaxListEntries caps a directory listing. A listing is usually read by an
	// agent, and handing a model an unbounded node_modules exhausts its context
	// for no benefit. The result says when it was truncated rather than
	// pretending it was complete.
	MaxListEntries = 1000

	// MaxWalkEntries bounds a recursive walk separately: recursion is where an
	// accidental "list everything" actually hurts.
	MaxWalkEntries = 5000
)

// Workspace opens the execution workspace as a confined root.
//
// The caller MUST Close it. Every file action goes through here; there is no
// other way into the filesystem in this package, which is what makes the
// guarantee checkable by reading one function.
func Workspace() (*os.Root, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("unable to determine the flow's workspace: %w", err)
	}
	root, err := os.OpenRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("unable to open the flow's workspace: %w", err)
	}
	return root, nil
}

// Rel normalises a user-supplied path into a workspace-relative one.
//
// A leading "/" is read as the workspace root rather than the machine root, so
// an agent that writes "/output/report.txt" gets <workspace>/output/report.txt
// instead of an error it cannot act on. That is a convenience, NOT the security
// boundary — os.Root refuses an escape whatever this function returns, and the
// tests assert exactly that.
//
// The empty path and "." both mean the workspace itself, which is meaningful
// for list and info and rejected by the actions that need a name.
func Rel(raw string) string {
	p := strings.TrimSpace(raw)
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimLeft(p, "/")
	p = path.Clean(p)
	if p == "." || p == "/" {
		return "."
	}
	return p
}

// RequirePath is Rel for actions that need to name something, refusing the
// workspace root itself.
func RequirePath(raw, field string) (string, error) {
	p := Rel(raw)
	if p == "." {
		return "", fmt.Errorf("%s is required, as a path relative to the flow's workspace (for example \"reports/summary.txt\")", field)
	}
	return p, nil
}

// DescribeError turns a confinement refusal into something a flow author or an
// agent can act on.
//
// os.Root reports "path escapes from parent", which is accurate and means
// nothing to the person reading it. Since the whole point is that these actions
// are handed to agents, the message has to explain the model rather than the
// syscall.
func DescribeError(err error, requested string) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "escapes from parent") {
		return fmt.Errorf(
			"%q is outside the flow's workspace, so it cannot be reached. Every path in this action is relative to the workspace this execution runs in — there is no way to read or write anywhere else on the machine",
			requested)
	}
	if IsNotExist(err) {
		return fmt.Errorf("%q does not exist in the flow's workspace", requested)
	}
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("%q cannot be accessed: permission denied", requested)
	}
	return err
}

// Entry is one row of a listing, shaped for both a person and a model.
type Entry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// EntryFrom builds a listing row, tolerating a stat failure so one unreadable
// file does not cost the caller the whole listing.
func EntryFrom(rel string, info fs.FileInfo) Entry {
	e := Entry{Name: path.Base(rel), Path: rel}
	if info == nil {
		return e
	}
	e.IsDir = info.IsDir()
	if !info.IsDir() {
		e.Size = info.Size()
	}
	e.Modified = info.ModTime().UTC().Format("2006-01-02T15:04:05Z")
	return e
}

// --- result shapers ---

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{"tool_result": "Error: " + msg, "success": false, "error": msg}
}

func OkResult(summary string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{"tool_result": summary, "success": true, "error": ""}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// --- input helpers ---

func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	v := strings.TrimSpace(*c.String())
	// An unresolved reference is not a path. Treating "${flow.missing}" as one
	// produces a baffling "does not exist" instead of naming the real problem.
	if strings.HasPrefix(v, "${") {
		return ""
	}
	return v
}

func OptionalBool(name string, inputs []*core.Connection) bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return false
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	// The editor stores some boolean values as strings.
	if s := c.String(); s != nil {
		return strings.EqualFold(strings.TrimSpace(*s), "true")
	}
	return false
}

func OptionalInt(name string, inputs []*core.Connection, fallback int) int {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

// IsNotExist reports whether an error means "not there", so an action can treat
// absence as an answer rather than a failure. os.Root wraps its errors, so
// os.IsNotExist alone is not reliable here.
func IsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
