package fill_table

import (
	"testing"

	. "github.com/onsi/gomega"
)

// A document with one intro paragraph and a 2x2 table. Cell content carries the
// structural start/end indices the Docs API needs to target a cell.
const sampleDoc = `{
  "body": { "content": [
    {"startIndex":1,"endIndex":10,"paragraph":{"elements":[{"textRun":{"content":"Questions\n"}}]}},
    {"startIndex":10,"endIndex":40,"table":{"tableRows":[
      {"tableCells":[
        {"content":[{"startIndex":12,"endIndex":20}]},
        {"content":[{"startIndex":20,"endIndex":22}]}
      ]},
      {"tableCells":[
        {"content":[{"startIndex":24,"endIndex":30}]},
        {"content":[{"startIndex":30,"endIndex":32}]}
      ]}
    ]}}
  ]}
}`

func TestResolveCellEdits_MapsCellsAndSortsDescending(t *testing.T) {
	RegisterTestingT(t)

	edits, err := resolveCellEdits([]byte(sampleDoc), 0, []cellTarget{
		{Row: 0, Column: 1, Text: "A"},
		{Row: 1, Column: 1, Text: "B"},
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(edits).To(HaveLen(2))
	// Highest start index first, so a single batchUpdate doesn't shift indices.
	Expect(edits[0]).To(Equal(cellEdit{Start: 30, End: 32, Text: "B"}))
	Expect(edits[1]).To(Equal(cellEdit{Start: 20, End: 22, Text: "A"}))
}

func TestResolveCellEdits_OutOfRange(t *testing.T) {
	RegisterTestingT(t)

	_, err := resolveCellEdits([]byte(sampleDoc), 0, []cellTarget{{Row: 5, Column: 0, Text: "x"}})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("row 5 out of range"))

	_, err = resolveCellEdits([]byte(sampleDoc), 0, []cellTarget{{Row: 0, Column: 9, Text: "x"}})
	Expect(err.Error()).To(ContainSubstring("column 9 out of range"))

	_, err = resolveCellEdits([]byte(sampleDoc), 3, []cellTarget{{Row: 0, Column: 0, Text: "x"}})
	Expect(err.Error()).To(ContainSubstring("table 3 not found"))
}

func TestBuildRequests_InsertOnly(t *testing.T) {
	RegisterTestingT(t)

	reqs := buildRequests([]cellEdit{{Start: 30, End: 32, Text: "B"}, {Start: 20, End: 22, Text: "A"}}, false)
	Expect(reqs).To(HaveLen(2))
	// Each is an insertText at the cell start.
	ins := reqs[0]["insertText"].(map[string]interface{})
	Expect(ins["text"]).To(Equal("B"))
	Expect(ins["location"].(map[string]interface{})["index"]).To(Equal(30))
}

func TestBuildRequests_ClearThenInsert(t *testing.T) {
	RegisterTestingT(t)

	// End-1 > Start, so a non-empty cell gets a delete before the insert.
	reqs := buildRequests([]cellEdit{{Start: 20, End: 25, Text: "A"}}, true)
	Expect(reqs).To(HaveLen(2))
	del := reqs[0]["deleteContentRange"].(map[string]interface{})["range"].(map[string]interface{})
	Expect(del["startIndex"]).To(Equal(20))
	Expect(del["endIndex"]).To(Equal(24)) // trailing newline (25-1) preserved
	Expect(reqs[1]).To(HaveKey("insertText"))

	// An empty cell (End-1 == Start) gets no delete, just an insert.
	reqs = buildRequests([]cellEdit{{Start: 20, End: 21, Text: "A"}}, true)
	Expect(reqs).To(HaveLen(1))
	Expect(reqs[0]).To(HaveKey("insertText"))
}
