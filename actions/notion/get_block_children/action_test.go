package notion_get_block_children

import (
	"encoding/json"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// A representative /blocks/:id/children result: a paragraph, a heading with a
// nested child, and an id-less/typeless entry that must be tolerated.
const sampleBlocks = `[
  {"id": "aaaa1111", "type": "paragraph", "paragraph": {"rich_text": [{"type": "text", "plain_text": "Hello", "text": {"content": "Hello"}}]}},
  {"id": "bbbb2222", "type": "heading_2", "has_children": true, "heading_2": {"rich_text": [{"type": "text", "plain_text": "A heading", "text": {"content": "A heading"}}]}},
  {"id": "cccc3333", "type": "divider", "divider": {}}
]`

func TestSummariseBlocks_IncludesIDsAndChildHint(t *testing.T) {
	RegisterTestingT(t)

	var results []interface{}
	Expect(json.Unmarshal([]byte(sampleBlocks), &results)).To(Succeed())

	summary, ids := summariseBlocks("page-123", results)

	// Every block's ID must appear in the AI-readable summary — the whole point.
	Expect(summary).To(ContainSubstring("(id: aaaa1111)"))
	Expect(summary).To(ContainSubstring("(id: bbbb2222)"))
	Expect(summary).To(ContainSubstring("(id: cccc3333)"))
	// Text and type still render.
	Expect(summary).To(ContainSubstring("[paragraph] Hello"))
	Expect(summary).To(ContainSubstring("[heading_2] A heading"))
	// A divider has no text but still lists its id and type.
	Expect(summary).To(ContainSubstring("[divider]  (id: cccc3333)"))
	// has_children is flagged.
	Expect(summary).To(ContainSubstring("A heading  (id: bbbb2222) (has children)"))

	// The block_ids output is the ordered list of IDs.
	Expect(ids).To(Equal([]string{"aaaa1111", "bbbb2222", "cccc3333"}))
}

func TestSummariseBlocks_ToleratesMalformedEntries(t *testing.T) {
	RegisterTestingT(t)
	results := []interface{}{"not-a-map", map[string]interface{}{"type": "paragraph"}}
	summary, ids := summariseBlocks("p", results)
	Expect(strings.Contains(summary, "child block(s)")).To(BeTrue())
	// The typeless/id-less map yields no id; the string entry is skipped.
	Expect(ids).To(BeEmpty())
}
