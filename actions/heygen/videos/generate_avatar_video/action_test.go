package generate_avatar_video

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestNormalizeEngine(t *testing.T) {
	RegisterTestingT(t)

	// string -> object (the exact failure agents hit via raw_json)
	b := map[string]interface{}{"engine": "avatar_v"}
	normalizeEngine(b)
	Expect(b["engine"]).To(Equal(map[string]interface{}{"type": "avatar_v"}))

	// already an object -> untouched
	obj := map[string]interface{}{"type": "avatar_iv", "reference_look_id": "look_1"}
	b = map[string]interface{}{"engine": obj}
	normalizeEngine(b)
	Expect(b["engine"]).To(Equal(obj))

	// empty string / nil -> dropped
	b = map[string]interface{}{"engine": ""}
	normalizeEngine(b)
	Expect(b).ToNot(HaveKey("engine"))
	b = map[string]interface{}{"engine": nil}
	normalizeEngine(b)
	Expect(b).ToNot(HaveKey("engine"))
}

func TestNormalizeBackground(t *testing.T) {
	RegisterTestingT(t)

	// hex with # -> colour
	b := map[string]interface{}{"background": "#150e14"}
	normalizeBackground(b)
	Expect(b["background"]).To(Equal(map[string]interface{}{"type": "color", "value": "#150e14"}))

	// bare hex -> colour with # prepended
	b = map[string]interface{}{"background": "150e14"}
	normalizeBackground(b)
	Expect(b["background"]).To(Equal(map[string]interface{}{"type": "color", "value": "#150e14"}))

	// http(s) URL -> image
	b = map[string]interface{}{"background": "https://cdn/room.jpg"}
	normalizeBackground(b)
	Expect(b["background"]).To(Equal(map[string]interface{}{"type": "image", "url": "https://cdn/room.jpg"}))

	// object left untouched; empty dropped
	obj := map[string]interface{}{"type": "image", "asset_id": "a1"}
	b = map[string]interface{}{"background": obj}
	normalizeBackground(b)
	Expect(b["background"]).To(Equal(obj))
	b = map[string]interface{}{"background": ""}
	normalizeBackground(b)
	Expect(b).ToNot(HaveKey("background"))
}
