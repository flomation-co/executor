package ai_common

import (
	"encoding/base64"
	"testing"
)

// TestBuildAnthropicToolResultContent_NoImages keeps the backwards-compatible
// behaviour: a tool result with no images stays a plain string.
func TestBuildAnthropicToolResultContent_NoImages(t *testing.T) {
	out := BuildAnthropicToolResultContent("plain result", nil)
	s, ok := out.(string)
	if !ok || s != "plain result" {
		t.Fatalf("expected plain string, got %T %v", out, out)
	}
}

// TestBuildAnthropicToolResultContent_WithImage asserts a screenshot-bearing
// tool result becomes a block array of image(s) followed by the text block, in
// Anthropic's expected shape — this is what lets the model actually see it.
func TestBuildAnthropicToolResultContent_WithImage(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 1, 2, 3}
	out := BuildAnthropicToolResultContent("screenshot captured", []VisionBlob{{Mime: "image/png", Bytes: png}})

	blocks, ok := out.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected []map, got %T", out)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks (image + text), got %d", len(blocks))
	}
	if blocks[0]["type"] != "image" {
		t.Errorf("block[0] type = %v, want image", blocks[0]["type"])
	}
	src, ok := blocks[0]["source"].(map[string]interface{})
	if !ok || src["media_type"] != "image/png" || src["type"] != "base64" {
		t.Fatalf("bad image source: %v", blocks[0]["source"])
	}
	if src["data"] != base64.StdEncoding.EncodeToString(png) {
		t.Errorf("image data not base64-encoded correctly")
	}
	if blocks[1]["type"] != "text" || blocks[1]["text"] != "screenshot captured" {
		t.Errorf("block[1] = %v, want text 'screenshot captured'", blocks[1])
	}
}

// TestBuildAnthropicToolResultContent_ImageOnly drops the text block when the
// text is empty, leaving just the image.
func TestBuildAnthropicToolResultContent_ImageOnly(t *testing.T) {
	out := BuildAnthropicToolResultContent("   ", []VisionBlob{{Mime: "image/jpeg", Bytes: []byte{1}}})
	blocks, ok := out.([]map[string]interface{})
	if !ok || len(blocks) != 1 || blocks[0]["type"] != "image" {
		t.Fatalf("expected a single image block, got %v", out)
	}
}
