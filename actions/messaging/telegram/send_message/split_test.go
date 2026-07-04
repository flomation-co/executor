package telegram

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitTelegramMessage_ShortUnchanged(t *testing.T) {
	in := "hello world"
	got := splitTelegramMessage(in)
	if len(got) != 1 || got[0] != in {
		t.Fatalf("short message must be one unchanged chunk, got %d: %q", len(got), got)
	}
}

func TestSplitTelegramMessage_LongSplitsAndPreservesContent(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 2000; i++ {
		b.WriteString("line\n") // 10000 chars, newline boundaries
	}
	in := b.String()

	chunks := splitTelegramMessage(in)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if utf16Len(c) > maxMessageLen {
			t.Errorf("chunk %d exceeds the 4096-unit limit: %d", i, utf16Len(c))
		}
	}
	if strings.Join(chunks, "") != in {
		t.Error("re-joined chunks must equal the original — no content lost")
	}
}

func TestSplitTelegramMessage_MultiByteNotCutMidCharacter(t *testing.T) {
	// 3000 astral emoji = 6000 UTF-16 units — must split; each is 2 units.
	in := strings.Repeat("😀", 3000)
	chunks := splitTelegramMessage(in)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if utf16Len(c) > maxMessageLen {
			t.Errorf("chunk %d exceeds 4096 units: %d", i, utf16Len(c))
		}
		if !utf8.ValidString(c) {
			t.Errorf("chunk %d is not valid UTF-8 — cut mid-character", i)
		}
		if strings.Count(c, "😀")*len("😀") != len(c) {
			t.Errorf("chunk %d was cut mid-emoji", i)
		}
	}
	if strings.Join(chunks, "") != in {
		t.Error("re-joined chunks must equal the original")
	}
}

func TestSplitTelegramMessage_LongTokenHardSplit(t *testing.T) {
	// One 10000-char token with no spaces/newlines — hard rune-boundary split.
	in := strings.Repeat("A", 10000)
	chunks := splitTelegramMessage(in)
	if len(chunks) < 3 {
		t.Fatalf("expected >=3 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if utf16Len(c) > maxMessageLen {
			t.Errorf("chunk %d exceeds 4096 units", i)
		}
	}
	if strings.Join(chunks, "") != in {
		t.Error("re-joined chunks must equal the original")
	}
}
