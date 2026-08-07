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

// A table with two rows: Notion returns each row's content under `cells`
// (an array of rich_text arrays), NOT `rich_text`. The summary must render the
// cell text so a caller can read/verify a row without a separate fetch.
const sampleTableBlocks = `[
  {"id": "row1", "type": "table_row", "table_row": {"cells": [
    [{"type": "text", "plain_text": "3", "text": {"content": "3"}}],
    [{"type": "text", "plain_text": "Mike Ashley", "text": {"content": "Mike Ashley"}}],
    [{"type": "text", "plain_text": "Infrastructure Engineer", "text": {"content": "Infrastructure Engineer"}}],
    [{"type": "text", "plain_text": "GBG", "text": {"content": "GBG"}}],
    [{"type": "text", "plain_text": "Wrexham", "text": {"content": "Wrexham"}}],
    [{"type": "text", "plain_text": "mike.ashley@gbgplc.com", "text": {"content": "mike.ashley@gbgplc.com"}}]
  ]}},
  {"id": "row2", "type": "table_row", "table_row": {"cells": [[], []]}}
]`

func TestSummariseBlocks_RendersTableRowCells(t *testing.T) {
	RegisterTestingT(t)

	var results []interface{}
	Expect(json.Unmarshal([]byte(sampleTableBlocks), &results)).To(Succeed())

	summary, ids := summariseBlocks("table-abc", results)

	// The full row renders pipe-joined so every column is verifiable — this is
	// the exact readback the agent needs to prove a table_row update landed.
	Expect(summary).To(ContainSubstring("[table_row] 3 | Mike Ashley | Infrastructure Engineer | GBG | Wrexham | mike.ashley@gbgplc.com  (id: row1)"))
	// An all-empty two-column row renders the delimiter (both columns blank),
	// which still conveys the column count. The row id is always present.
	Expect(summary).To(ContainSubstring("[table_row]  |   (id: row2)"))
	Expect(ids).To(Equal([]string{"row1", "row2"}))
}

func TestSummariseBlocks_ToleratesMalformedEntries(t *testing.T) {
	RegisterTestingT(t)
	results := []interface{}{"not-a-map", map[string]interface{}{"type": "paragraph"}}
	summary, ids := summariseBlocks("p", results)
	Expect(strings.Contains(summary, "child block(s)")).To(BeTrue())
	// The typeless/id-less map yields no id; the string entry is skipped.
	Expect(ids).To(BeEmpty())
}
