package core

import "testing"

const (
	testImgBlob = "flo:blob:abababababababababababababababab?size=100&type=image/png"
	testVidBlob = "flo:blob:cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd?size=100&type=video/mp4"
)

func TestImageRefMime(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"blob_png", testImgBlob, "image/png"},
		{"blob_video", testVidBlob, ""},
		{"file_png", "flo:file:shots/screen.png", "image/png"},
		{"file_mp4", "flo:file:clips/rec.mp4", ""},
		{"plain_text_mentioning_ref", "captured a shot: " + testImgBlob, ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ImageRefMime(tc.in); got != tc.want {
				t.Errorf("ImageRefMime(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCollectImageRefs is the regression guard ensuring a tool's image output
// (a Desktop Screenshot's `image`) is surfaced for the AI vision path, while
// non-image refs and free text that merely mentions a ref are ignored.
func TestCollectImageRefs(t *testing.T) {
	outputs := map[string]interface{}{
		"image":       testImgBlob,
		"video":       testVidBlob,
		"tool_result": "Captured screenshot (100 bytes); ref: " + testImgBlob, // text, must NOT count
		"size_bytes":  int64(100),
		"success":     true,
	}
	got := collectImageRefs(outputs)
	if len(got) != 1 || got[0] != testImgBlob {
		t.Fatalf("collectImageRefs = %v, want [%s]", got, testImgBlob)
	}
}

func TestCollectImageRefs_DedupAndCap(t *testing.T) {
	// Same image under two keys → one entry.
	dup := map[string]interface{}{"a": testImgBlob, "b": testImgBlob}
	if got := collectImageRefs(dup); len(got) != 1 {
		t.Errorf("dedup: got %d refs, want 1: %v", len(got), got)
	}

	// More distinct images than the cap → capped.
	many := map[string]interface{}{}
	for i, h := range []string{
		"11111111111111111111111111111111", "22222222222222222222222222222222",
		"33333333333333333333333333333333", "44444444444444444444444444444444",
		"55555555555555555555555555555555", "66666666666666666666666666666666",
	} {
		many[string(rune('a'+i))] = "flo:blob:" + h + "?size=1&type=image/png"
	}
	if got := collectImageRefs(many); len(got) != maxToolResultImages {
		t.Errorf("cap: got %d refs, want %d", len(got), maxToolResultImages)
	}
}

func TestCollectImageRefs_Empty(t *testing.T) {
	if got := collectImageRefs(nil); got != nil {
		t.Errorf("nil map: got %v, want nil", got)
	}
	if got := collectImageRefs(map[string]interface{}{"x": "no refs here", "n": 5}); got != nil {
		t.Errorf("no refs: got %v, want nil", got)
	}
}
