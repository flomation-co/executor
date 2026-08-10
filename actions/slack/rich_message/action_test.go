package slack_rich_message

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestParseBlockKitArray(t *testing.T) {
	RegisterTestingT(t)

	// A bare array — the canonical shape.
	arr, err := parseBlockKitArray(`[{"type":"divider"},{"type":"section"}]`)
	Expect(err).ToNot(HaveOccurred())
	Expect(arr).To(HaveLen(2))

	// The Block Kit Builder wrapper object {"blocks":[...]} — the shape that
	// used to fail with "cannot unmarshal object into Go value of type
	// []interface {}". Must now be unwrapped transparently.
	arr, err = parseBlockKitArray(`{"blocks":[{"type":"divider"},{"type":"section"},{"type":"context"}]}`)
	Expect(err).ToNot(HaveOccurred())
	Expect(arr).To(HaveLen(3))

	// Markdown code fences the AI sometimes adds are stripped, wrapper included.
	arr, err = parseBlockKitArray("```json\n{\"blocks\":[{\"type\":\"divider\"}]}\n```")
	Expect(err).ToNot(HaveOccurred())
	Expect(arr).To(HaveLen(1))

	// A fenced bare array too.
	arr, err = parseBlockKitArray("```\n[{\"type\":\"divider\"}]\n```")
	Expect(err).ToNot(HaveOccurred())
	Expect(arr).To(HaveLen(1))

	// Genuinely invalid JSON still errors (surfacing the array-parse error).
	_, err = parseBlockKitArray(`not json`)
	Expect(err).To(HaveOccurred())

	// An object with no "blocks" key is not silently accepted.
	_, err = parseBlockKitArray(`{"text":"hi"}`)
	Expect(err).To(HaveOccurred())
}

func TestParseJSONArrayOrWrapped_Attachments(t *testing.T) {
	RegisterTestingT(t)

	arr, err := parseJSONArrayOrWrapped(`{"attachments":[{"color":"#36a64f"}]}`, "attachments")
	Expect(err).ToNot(HaveOccurred())
	Expect(arr).To(HaveLen(1))

	arr, err = parseJSONArrayOrWrapped(`[{"color":"#36a64f"}]`, "attachments")
	Expect(err).ToNot(HaveOccurred())
	Expect(arr).To(HaveLen(1))
}
