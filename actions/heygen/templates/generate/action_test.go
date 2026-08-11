package generate

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestBuildVariables(t *testing.T) {
	RegisterTestingT(t)

	// text_values are wrapped as HeyGen text variables.
	out := buildVariables(map[string]interface{}{"headline": "Ditch the legacy scheduler"}, nil)
	Expect(out).To(HaveKey("headline"))
	Expect(out["headline"]).To(Equal(map[string]interface{}{"type": "text", "content": "Ditch the legacy scheduler"}))

	// full typed variables override text_values for the same key.
	img := map[string]interface{}{"type": "image", "asset": map[string]interface{}{"type": "url", "url": "https://x/bg.png"}}
	out = buildVariables(
		map[string]interface{}{"headline": "hi", "shared": "text-form"},
		map[string]interface{}{"shared": img},
	)
	Expect(out["headline"]).To(Equal(map[string]interface{}{"type": "text", "content": "hi"}))
	Expect(out["shared"]).To(Equal(img)) // full-typed wins

	// nothing in -> empty (caller omits the variables field entirely).
	Expect(buildVariables(nil, nil)).To(BeEmpty())
}
