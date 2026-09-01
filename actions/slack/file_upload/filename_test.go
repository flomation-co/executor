package slack_file_upload

import (
	"testing"

	core "flomation.app/automate/executor"
)

// pngMagic is the PNG file signature — enough for http.DetectContentType to
// report image/png when no MIME type is known.
var pngMagic = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}

// TestDeriveFilename is the regression guard for the bug where a screenshot
// uploaded to Slack landed as an unpreviewable "file.bin" octet-stream because
// the caller supplied no (or a generic) filename.
func TestDeriveFilename(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		isText   bool
		mimeType string
		data     []byte
		want     string
	}{
		// Missing / generic names are fixed up from the MIME type.
		{"empty_png_mime", "", false, "image/png", nil, "file.png"},
		{"filebin_png_mime", "file.bin", false, "image/png", nil, "file.png"},
		{"blob_type_with_charset", "", false, "image/png; charset=binary", nil, "file.png"},
		{"empty_mp4_mime", "", false, "video/mp4", nil, "file.mp4"},
		{"noext_csv_mime", "report", false, "text/csv", nil, "report.csv"},
		// No MIME → sniff the bytes.
		{"empty_sniff_png", "", false, "", pngMagic, "file.png"},
		// No MIME, undetectable, text hint → .txt not .bin.
		{"empty_text_hint", "", true, "", []byte("hello, world"), "file.txt"},
		// A caller-supplied specific name is respected as-is.
		{"explicit_png_kept", "screenshot.png", false, "image/jpeg", nil, "screenshot.png"},
		{"explicit_csv_kept", "data.csv", false, "", nil, "data.csv"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveFilename(tc.in, "", tc.isText, tc.mimeType, tc.data); got != tc.want {
				t.Errorf("deriveFilename(%q, %v, %q) = %q, want %q", tc.in, tc.isText, tc.mimeType, got, tc.want)
			}
		})
	}
}

// TestDeriveFilename_UsesNameCarriedByReference covers the case that used to
// force every caller to invent a name: the file came from somewhere that knew
// what it was called, so the upload should use that rather than "file.png".
func TestDeriveFilename_UsesNameCarriedByReference(t *testing.T) {
	ref := core.WithBlobTokenName(
		"flo:blob:a1b2c3d4e5f60718293a4b5c6d7e8f90?size=9&type=image%2Fpng", "quarterly chart.png")

	if got := deriveFilename("", ref, false, "image/png", nil); got != "quarterly chart.png" {
		t.Errorf("carried name should win over a generated one, got %q", got)
	}
	// An explicit, specific name still beats the carried one.
	if got := deriveFilename("chosen.png", ref, false, "image/png", nil); got != "chosen.png" {
		t.Errorf("explicit name should win, got %q", got)
	}
	// A reference carrying no name falls through to the existing behaviour.
	if got := deriveFilename("", "flo:blob:a1b2c3d4e5f60718293a4b5c6d7e8f90", false, "image/png", nil); got != "file.png" {
		t.Errorf("expected the generated name, got %q", got)
	}
}
