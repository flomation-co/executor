package notion_common

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
)

func unmarshalBlock(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var b map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return b
}

// BlockPlainText is what makes update_block self-verifying and table rows
// readable: it must render a paragraph's rich_text AND a table_row's cells.
func TestBlockPlainText(t *testing.T) {
	RegisterTestingT(t)

	paragraph := unmarshalBlock(t, `{"type":"paragraph","paragraph":{"rich_text":[{"plain_text":"Hello world"}]}}`)
	Expect(BlockPlainText(paragraph)).To(Equal("Hello world"))

	// A table_row's content lives under `cells` (array of rich_text arrays) —
	// Notion DOES return it on read, so we can prove a row update landed.
	tableRow := unmarshalBlock(t, `{"type":"table_row","table_row":{"cells":[
		[{"plain_text":"3"}],
		[{"plain_text":"Mike Ashley"}],
		[{"plain_text":"mike.ashley@gbgplc.com"}]
	]}}`)
	Expect(BlockPlainText(tableRow)).To(Equal("3 | Mike Ashley | mike.ashley@gbgplc.com"))

	// A block with neither rich_text nor cells (e.g. a divider) renders empty.
	divider := unmarshalBlock(t, `{"type":"divider","divider":{}}`)
	Expect(BlockPlainText(divider)).To(Equal(""))
}

func TestExtractCells(t *testing.T) {
	RegisterTestingT(t)

	var cells []interface{}
	Expect(json.Unmarshal([]byte(`[[{"plain_text":"a"}],[{"plain_text":"b"}]]`), &cells)).To(Succeed())
	Expect(ExtractCells(cells)).To(Equal("a | b"))

	Expect(ExtractCells(nil)).To(Equal(""))
}
