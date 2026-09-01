// Package download fetches a file from an http(s) URL into the flow workspace
// and returns a media reference (flo:blob: for small files, flo:file: for large
// ones) that downstream actions accept directly — e.g. Slack File Upload,
// Google Drive Upload, or an email attachment. This is the plain "URL to file"
// bridge: no re-encoding, no base64 wiring.
//
// A guarded dialer refuses to connect to non-public addresses (loopback,
// private, link-local — including the cloud metadata endpoint) to limit SSRF.
package download

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Download File"
	Description  = "Download a file from a URL into the workspace as a reference you can upload or attach."
	Website      = "https://www.flomation.co"
	Icon         = "globe+arrow-down"
	Date         = "11/08/2026"
	Type         = core.ActionTypeAction

	requestTimeout = 120 * time.Second
	maxDownload    = 250 << 20 // 250 MB safety cap
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "File URL", Placeholder: "https://…", Required: true},
	{Name: "filename", Type: core.ConnectionTypeString, Label: "Filename (optional, e.g. demo.mp4 — otherwise derived from the URL)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "file", Type: core.ConnectionTypeString, Label: "File reference (feed to Slack File Upload, Drive Upload, etc.)"},
	{Name: "filename", Type: core.ConnectionTypeString, Label: "Filename"},
	{Name: "mime_type", Type: core.ConnectionTypeString, Label: "MIME type"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	rawURL := optStr("url", inputs)
	if rawURL == "" {
		return errResult("url is required"), nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errResult("url must be a valid http(s) URL"), nil
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, rawURL, nil)
	if err != nil {
		return errResult(err.Error()), nil
	}
	req.Header.Set("Accept", "*/*")

	resp, err := guardedClient().Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("download failed: %s", err)), nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errResult(fmt.Sprintf("download failed: HTTP %d", resp.StatusCode)), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return errResult(fmt.Sprintf("reading response: %s", err)), nil
	}
	if len(body) > maxDownload {
		return errResult(fmt.Sprintf("file exceeds the %d MB download limit", maxDownload>>20)), nil
	}

	mimeType := cleanMime(resp.Header.Get("Content-Type"))
	filename := deriveFilename(optStr("filename", inputs), resp.Header.Get("Content-Disposition"), u, mimeType)

	// Write under the real filename rather than a random scratch handle. The
	// name is the whole reason to prefer this over a raw fetch: whatever
	// consumes the reference — a Drive upload, a Slack attachment, Meta's
	// image library — sends it onwards.
	scratch, err := flow.MediaScratchFileNamed(filename)
	if err != nil {
		return errResult(fmt.Sprintf("workspace unavailable: %s", err)), nil
	}
	if err := os.WriteFile(scratch, body, 0o600); err != nil {
		return errResult(fmt.Sprintf("writing file: %s", err)), nil
	}
	ref, err := flow.EmitMediaFile(scratch)
	if err != nil {
		return errResult(fmt.Sprintf("saving to workspace: %s", err)), nil
	}

	return map[string]interface{}{
		// Surface the file reference IN tool_result: an agent only reads
		// tool_result, so without the handle here it can't chain the download
		// into an upload/attach action (it lives in the `file` output too, for
		// node-wired flows).
		"tool_result": fmt.Sprintf("Downloaded %s (%d bytes, %s). Pass this file reference to an upload/attach action's file input: %s", filename, len(body), mimeType, ref),
		"file":        ref,
		"filename":    filename,
		"mime_type":   mimeType,
		"size_bytes":  int64(len(body)),
		"success":     true,
		"error":       "",
	}, nil
}

// guardedClient returns an HTTP client whose dialer rejects non-public targets.
func guardedClient() *http.Client {
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	return &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
				if err != nil {
					return nil, err
				}
				for _, ip := range ips {
					if isDisallowedIP(ip) {
						return nil, fmt.Errorf("refusing to connect to non-public address %s", ip)
					}
				}
				// Dial the resolved (checked) IP to avoid a DNS-rebind window.
				return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
			},
		},
	}
}

// isDisallowedIP blocks loopback, private, link-local (incl. the 169.254.169.254
// metadata endpoint), unspecified and multicast addresses.
func isDisallowedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast()
}

// deriveFilename picks the best filename available, preferring what the caller
// asked for, then what the server said the file is called, then the URL's last
// path segment, and finally a generated name carrying the right extension.
//
// Content-Disposition is worth consulting because the interesting URLs often
// have no filename in them at all — a signed S3 link, an export endpoint, a
// /download/<id> route — and the header is where the real name lives.
func deriveFilename(explicit, contentDisposition string, u *url.URL, mimeType string) string {
	if n := core.SanitiseFilename(explicit); n != "" {
		return core.EnsureFilenameExtension(n, mimeType)
	}
	if n := core.SanitiseFilename(filenameFromDisposition(contentDisposition)); n != "" {
		return core.EnsureFilenameExtension(n, mimeType)
	}
	if base := path.Base(u.Path); strings.Contains(base, ".") {
		if n := core.SanitiseFilename(base); n != "" {
			return n
		}
	}
	return core.UploadFilename("", "", mimeType, "download")
}

// filenameFromDisposition reads the filename out of a Content-Disposition
// header, preferring RFC 5987's filename* (which carries an encoding, so it
// can express non-ASCII names) over the plain filename.
func filenameFromDisposition(header string) string {
	if header == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	// ParseMediaType already decodes filename* into "filename" when present,
	// so one lookup covers both forms.
	return params["filename"]
}

func cleanMime(ct string) string {
	if ct == "" {
		return "application/octet-stream"
	}
	if mt, _, err := mime.ParseMediaType(ct); err == nil {
		return mt
	}
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		return strings.TrimSpace(ct[:i])
	}
	return ct
}

func optStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return strings.TrimSpace(*c.String())
}

func errResult(msg string) map[string]interface{} {
	return map[string]interface{}{"tool_result": "Error: " + msg, "success": false, "error": msg}
}
