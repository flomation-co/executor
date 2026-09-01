package core

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// FileRefPrefix marks a string value as a workspace-file reference: a pointer to
// a file on the per-execution working directory that flows between nodes instead
// of the bytes themselves. Unlike a blob token (flo:blob:, capped and DB-backed),
// a file ref has no size limit and never round-trips through the API — but it is
// EPHEMERAL: valid only within one execution on one runner, and it does NOT
// survive a HITL/Wait suspension. To keep a file, an action must push it to a
// durable sink (Push to S3, upload, attach). Deliberately a distinct scheme from
// flo:blob: so it stays opaque through DetokeniseInputs / TokeniseLargeOutputs —
// a large file reference is never materialised into RAM until an action opens it.
const FileRefPrefix = "flo:file:"

// mediaScratchDir is the workspace subdirectory media actions write intermediate
// files into. It is cleaned up wholesale with the execution directory.
const mediaScratchDir = ".flomation/media"

// IsFileRef reports whether s is a workspace-file reference.
func IsFileRef(s string) bool {
	return strings.HasPrefix(s, FileRefPrefix)
}

// ParseFileRef returns the workspace-relative path carried by a flo:file: ref.
func ParseFileRef(s string) (relPath string, ok bool) {
	if !strings.HasPrefix(s, FileRefPrefix) {
		return "", false
	}
	return filepath.FromSlash(strings.TrimPrefix(s, FileRefPrefix)), true
}

// ImageRefMime returns the image/* MIME type of a media reference — a flo:blob:
// token (whose type= hint is read) or a flo:file: workspace ref (whose MIME is
// inferred from its path extension) — or "" if the value is not a reference or
// is not an image. It is a cheap classifier (no bytes are read): the AI tool
// loop uses it to decide whether a tool output should be handed to the model as
// a vision block.
func ImageRefMime(ref string) string {
	ref = strings.TrimSpace(ref)
	switch {
	case IsBlobToken(ref):
		if _, _, mime, ok := ParseBlobToken(ref); ok && strings.HasPrefix(mime, "image/") {
			return mime
		}
	case IsFileRef(ref):
		if rel, ok := ParseFileRef(ref); ok {
			if mime := mimeOfFile(rel); strings.HasPrefix(mime, "image/") {
				return mime
			}
		}
	}
	return ""
}

// workspaceDir returns the per-execution working directory. The runner sets it as
// the executor process's cwd, and git/clone plus the shell actions rely on it —
// it is the shared scratch space for a single execution.
func workspaceDir() (string, error) {
	return os.Getwd()
}

// confineToWorkspace resolves a workspace-relative path to an absolute path that
// is GUARANTEED to stay inside the workspace, rejecting traversal and absolute
// escapes. SECURITY-CRITICAL: the input is attacker-influenceable — a flow author
// or AI tool call can supply an arbitrary flo:file: value (e.g. "../../etc/passwd").
func confineToWorkspace(ws, rel string) (string, error) {
	// Making it absolute against "/" first means filepath.Clean can collapse any
	// ".." without ever rising above the (virtual) root; the leftover is then
	// joined under the real workspace.
	clean := filepath.Clean(string(os.PathSeparator) + rel)
	abs := filepath.Join(ws, clean)
	// Belt-and-braces: confirm the result is genuinely under the workspace.
	r, err := filepath.Rel(ws, abs)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes the workspace", rel)
	}
	return abs, nil
}

// MediaScratchFile returns a fresh, unique path inside the workspace media scratch
// directory, with the given extension (leading dot optional; "" for none).
func (f *Flow) MediaScratchFile(ext string) (string, error) {
	ws, err := workspaceDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(ws, mediaScratchDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	nameBytes := make([]byte, 8)
	if _, err := rand.Read(nameBytes); err != nil {
		return "", err
	}
	name := hex.EncodeToString(nameBytes)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return filepath.Join(dir, name+ext), nil
}

// ResolveToLocalFile turns ANY inbound media representation into a real file on
// the execution workspace and returns its absolute path (plus a best-effort MIME
// type). This is the seam that makes local-vs-opaque references interchangeable:
//   - flo:file:<rel>  → the existing workspace file (validated + confined, no copy)
//   - flo:blob:<h>    → fetched from the blob store, written to scratch
//   - base64 / bytes  → decoded (if base64) and written to scratch
//
// URLs are deliberately NOT fetched here — that belongs in an explicit,
// SSRF-guarded "Fetch File" action, not a helper every action inherits.
//
// The "open a potentially huge file" moment is here, called deliberately by an
// action — never implicitly as a side-effect of graph wiring.
func (f *Flow) ResolveToLocalFile(value string) (path string, mimeType string, err error) {
	ws, err := workspaceDir()
	if err != nil {
		return "", "", err
	}

	switch {
	case IsFileRef(value):
		rel, _ := ParseFileRef(value)
		abs, err := confineToWorkspace(ws, rel)
		if err != nil {
			return "", "", err
		}
		// The file must exist; harden against a symlink that points outside the
		// workspace by re-checking containment after resolving links.
		real, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return "", "", fmt.Errorf("resolve file ref: %w", err)
		}
		if r, rerr := filepath.Rel(ws, real); rerr != nil || r == ".." || strings.HasPrefix(r, ".."+string(os.PathSeparator)) {
			return "", "", fmt.Errorf("file ref %q escapes the workspace via a symlink", rel)
		}
		return abs, mimeOfFile(abs), nil

	case IsBlobToken(value):
		b, err := f.Blobs().Get(value)
		if err != nil {
			return "", "", fmt.Errorf("fetch blob: %w", err)
		}
		_, _, blobMime, _ := ParseBlobToken(value)
		return f.writeScratch(b, extForMime(blobMime))

	case isHTTPURL(value):
		// Writing the URL text to a file and calling it the image is worse
		// than failing: the upload "succeeds" locally and the remote service
		// complains about the file contents instead. Fetching it here is not
		// the answer either — that needs the SSRF guard that lives in the
		// Download File action — so point at that action by name.
		return "", "", fmt.Errorf(
			"%q is a URL, not file content: use the Download File action first "+
				"and pass its file reference here", truncateForErr(strings.TrimSpace(value)))

	default:
		// A de-tokenised blob (raw bytes as string) or a legacy *_base64 value.
		return f.writeScratch(decodeMaybeBase64(value), "")
	}
}

// isHTTPURL reports whether s is a bare http(s) URL. Deliberately narrow: it
// must be the WHOLE value, so a base64 payload or a text file that merely
// mentions a URL is unaffected.
func isHTTPURL(s string) bool {
	t := strings.TrimSpace(s)
	if strings.ContainsAny(t, " \t\r\n") {
		return false
	}
	if !strings.HasPrefix(t, "http://") && !strings.HasPrefix(t, "https://") {
		return false
	}
	u, err := url.Parse(t)
	return err == nil && u.Host != ""
}

// ResolveToBytes turns any inbound media representation into its raw bytes (plus
// a best-effort MIME type). This is the seam for delivery/sink actions (Slack
// file, Drive upload, attach, …) so they accept a large flo:file: output the same
// way they already accept a flo:blob: token or base64 — without the caller having
// to know which form arrived. flo:file: reads are workspace-confined; blob reads
// come straight from the store (no scratch round-trip).
func (f *Flow) ResolveToBytes(value string) ([]byte, string, error) {
	switch {
	case IsFileRef(value):
		path, mimeType, err := f.ResolveToLocalFile(value) // applies confinement
		if err != nil {
			return nil, "", err
		}
		b, err := os.ReadFile(path)
		return b, mimeType, err
	case IsBlobToken(value):
		b, err := f.Blobs().Get(value)
		if err != nil {
			return nil, "", fmt.Errorf("fetch blob: %w", err)
		}
		_, _, blobMime, _ := ParseBlobToken(value)
		return b, blobMime, nil
	case isHTTPURL(value):
		// Same reasoning as ResolveToLocalFile: a sink action handed a URL
		// would otherwise upload a file whose contents are the URL text.
		return nil, "", fmt.Errorf(
			"%q is a URL, not file content: use the Download File action first "+
				"and pass its file reference here", truncateForErr(strings.TrimSpace(value)))
	default:
		return decodeMaybeBase64(value), "", nil
	}
}

// writeScratch persists bytes to a fresh workspace scratch file and returns its
// path + MIME type.
func (f *Flow) writeScratch(b []byte, ext string) (string, string, error) {
	p, err := f.MediaScratchFile(ext)
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return "", "", err
	}
	return p, mimeOfFile(p), nil
}

// EmitLocalFile converts an absolute (or workspace-relative) path that an action
// produced into a flo:file: reference to hand downstream. Rejects paths outside
// the workspace, so an action cannot accidentally leak a host path.
func (f *Flow) EmitLocalFile(path string) (string, error) {
	ws, err := workspaceDir()
	if err != nil {
		return "", err
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(ws, abs)
	}
	rel, err := filepath.Rel(ws, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("emit path %q is outside the workspace", path)
	}
	return FileRefPrefix + filepath.ToSlash(rel), nil
}

// mediaBlobLimitBytes mirrors the blob store's per-object cap. Files at or below
// it are emitted as blob tokens (previewable, sink-compatible, survive a
// suspension); larger files stay workspace-only.
const mediaBlobLimitBytes = 25 * 1024 * 1024

// EmitMediaFile returns the most useful reference to a workspace file an action
// produced, using dual-tier storage:
//   - small enough for the blob store (≤ mediaBlobLimitBytes) → a flo:blob: token
//     with a MIME hint, so the editor previews it inline, blob sinks can consume
//     it, and it survives a HITL/Wait suspension.
//   - otherwise → a flo:file: workspace reference (unlimited size, ephemeral).
//
// Either form is transparently accepted downstream by ResolveToLocalFile, so the
// choice is invisible to a media→media chain. Any blob failure falls back to the
// file reference, so this never fails where EmitLocalFile would succeed.
func (f *Flow) EmitMediaFile(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if bs := f.Blobs(); bs != nil && fi.Size() <= mediaBlobLimitBytes {
		if b, rerr := os.ReadFile(path); rerr == nil {
			if tok, perr := bs.Put(b, mimeOfFile(path)); perr == nil && tok != "" {
				return tok, nil
			}
		}
		// Any read/put failure (no backend, quota, oversize) → workspace ref.
	}
	return f.EmitLocalFile(path)
}

// mimeOfFile returns a best-effort MIME type: extension first, then a content
// sniff of the first bytes. Never errors — returns "" if undetermined.
func mimeOfFile(path string) string {
	if t := mime.TypeByExtension(filepath.Ext(path)); t != "" {
		return strings.SplitN(t, ";", 2)[0]
	}
	fh, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = fh.Close() }()
	head := make([]byte, 512)
	n, _ := fh.Read(head)
	if n == 0 {
		return ""
	}
	return strings.SplitN(http.DetectContentType(head[:n]), ";", 2)[0]
}

// canonicalExtForMime pins the extension for the media types that leave the
// platform in an upload. mime.ExtensionsByType returns the host's list in the
// order its mime.types file happens to define, so image/jpeg resolves to
// ".jfif" on macOS and ".jpg" on most Linux hosts. An extension that varies by
// runner is a bad thing to send an external API as a filename — Meta's
// /adimages is picky about it — so the common cases are decided here.
var canonicalExtForMime = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"image/bmp":       ".bmp",
	"image/tiff":      ".tif",
	"image/svg+xml":   ".svg",
	"video/mp4":       ".mp4",
	"video/quicktime": ".mov",
	"video/webm":      ".webm",
	"audio/mpeg":      ".mp3",
	"audio/mp4":       ".m4a",
	"audio/ogg":       ".ogg",
	"audio/wav":       ".wav",
	"application/pdf": ".pdf",
}

// extForMime maps a MIME type to a file extension (with dot), best-effort.
func extForMime(m string) string {
	if m == "" {
		return ""
	}
	m = strings.ToLower(strings.TrimSpace(strings.SplitN(m, ";", 2)[0]))
	if ext, ok := canonicalExtForMime[m]; ok {
		return ext
	}
	if exts, err := mime.ExtensionsByType(m); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ""
}

// decodeMaybeBase64 returns the base64-decoded bytes when s is plausibly base64,
// otherwise the raw string bytes. Guarded by a length floor so a short text value
// that happens to be valid base64 isn't mangled.
func decodeMaybeBase64(s string) []byte {
	t := strings.TrimSpace(s)
	if len(t) >= 16 && isLikelyBase64(t) {
		if b, err := base64.StdEncoding.DecodeString(t); err == nil {
			return b
		}
		if b, err := base64.URLEncoding.DecodeString(t); err == nil {
			return b
		}
	}
	return []byte(s)
}

func isLikelyBase64(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '+', r == '/', r == '-', r == '_', r == '=':
		default:
			return false
		}
	}
	return true
}
