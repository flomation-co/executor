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
