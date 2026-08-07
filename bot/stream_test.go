package bot

import (
	"strings"
	"testing"
)

func TestSplitMessageStaysUnderTheLimit(t *testing.T) {
	var chunks []string
	var chunk string

	chunks = splitMessage(strings.Repeat("a", 4500), messageLimit)
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}

	for _, chunk = range chunks {
		if len([]rune(chunk)) > messageLimit {
			t.Fatalf("chunk of %d runes exceeds the limit", len([]rune(chunk)))
		}
	}

	if strings.Join(chunks, "") != strings.Repeat("a", 4500) {
		t.Fatal("splitting lost or duplicated content")
	}
}

func TestSplitMessagePrefersLineBreaks(t *testing.T) {
	var text string
	var chunks []string

	text = strings.Repeat("x", 1500) + "\n" + strings.Repeat("y", 1000)

	chunks = splitMessage(text, messageLimit)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want 2", len(chunks))
	}

	if chunks[0] != strings.Repeat("x", 1500) || chunks[1] != strings.Repeat("y", 1000) {
		t.Fatalf("split did not fall on the newline: %d / %d runes", len(chunks[0]), len(chunks[1]))
	}
}

func TestSplitMessageCountsRunesNotBytes(t *testing.T) {
	var chunks []string
	var chunk string

	chunks = splitMessage(strings.Repeat("가", 2500), messageLimit)

	for _, chunk = range chunks {
		if len([]rune(chunk)) > messageLimit {
			t.Fatalf("chunk of %d runes exceeds the limit", len([]rune(chunk)))
		}
	}

	if strings.Join(chunks, "") != strings.Repeat("가", 2500) {
		t.Fatal("splitting corrupted multibyte text")
	}
}

func TestSplitMessageKeepsShortTextIntact(t *testing.T) {
	var chunks []string

	chunks = splitMessage("hello", messageLimit)
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Fatalf("chunks = %#v", chunks)
	}

	chunks = splitMessage("", messageLimit)
	if len(chunks) != 1 || chunks[0] != "" {
		t.Fatalf("empty text = %#v", chunks)
	}
}
