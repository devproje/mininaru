// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package memory

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
)

func setupTestMemoryFS(t *testing.T) {
	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatalf("InitFS() error = %v", err)
	}
}

func TestMemorySaveReadForgetRoundTrip(t *testing.T) {
	var save modules.Tool
	var read modules.Tool
	var forget modules.Tool
	var result string

	var err error

	setupTestMemoryFS(t)

	save = Tools("agent-1")[0]
	read = Tools("agent-1")[1]
	forget = Tools("agent-1")[2]

	result, err = save.Execute(context.Background(), `{"name":"likes-pnpm","description":"prefers pnpm over npm","type":"feedback","content":"always use pnpm"}`)
	if err != nil {
		t.Fatalf("memory_save error = %v", err)
	}
	if !strings.Contains(result, "likes-pnpm.md") {
		t.Fatalf("memory_save result = %q, want it to mention the saved file", result)
	}

	result, err = read.Execute(context.Background(), `{"name":"likes-pnpm"}`)
	if err != nil {
		t.Fatalf("memory_read error = %v", err)
	}
	if !strings.Contains(result, "always use pnpm") {
		t.Fatalf("memory_read result = %q, want it to contain the saved content", result)
	}
	if !strings.Contains(result, "type: feedback") {
		t.Fatalf("memory_read result = %q, want frontmatter with type: feedback", result)
	}

	result = LoadIndex("agent-1")
	if !strings.Contains(result, "likes-pnpm.md") {
		t.Fatalf("LoadIndex() = %q, want it to list likes-pnpm.md", result)
	}

	result, err = forget.Execute(context.Background(), `{"name":"likes-pnpm"}`)
	if err != nil {
		t.Fatalf("memory_forget error = %v", err)
	}
	if !strings.Contains(result, "likes-pnpm.md") {
		t.Fatalf("memory_forget result = %q, want it to mention the forgotten file", result)
	}

	_, err = read.Execute(context.Background(), `{"name":"likes-pnpm"}`)
	if err == nil {
		t.Fatalf("memory_read after forget should fail")
	}

	result = LoadIndex("agent-1")
	if strings.Contains(result, "likes-pnpm.md") {
		t.Fatalf("LoadIndex() after forget = %q, should no longer list likes-pnpm.md", result)
	}
}

func TestMemorySaveRejectsPathEscapingNames(t *testing.T) {
	var save modules.Tool

	var err error

	setupTestMemoryFS(t)

	save = Tools("agent-1")[0]

	_, err = save.Execute(context.Background(), `{"name":"../escape","description":"x","type":"user","content":"x"}`)
	if err == nil {
		t.Fatalf("memory_save with a path-escaping name should fail")
	}

	_, err = save.Execute(context.Background(), `{"name":"sub/dir","description":"x","type":"user","content":"x"}`)
	if err == nil {
		t.Fatalf("memory_save with a separator in the name should fail")
	}
}

func TestMemorySaveRejectsUnknownType(t *testing.T) {
	var save modules.Tool

	var err error

	setupTestMemoryFS(t)

	save = Tools("agent-1")[0]

	_, err = save.Execute(context.Background(), `{"name":"x","description":"x","type":"bogus","content":"x"}`)
	if err == nil {
		t.Fatalf("memory_save with an unknown type should fail")
	}
}

func TestMemorySaveUpsertsIndexLineInsteadOfDuplicating(t *testing.T) {
	var save modules.Tool
	var index string

	var err error

	setupTestMemoryFS(t)

	save = Tools("agent-1")[0]

	_, err = save.Execute(context.Background(), `{"name":"note","description":"first","type":"user","content":"a"}`)
	if err != nil {
		t.Fatalf("first memory_save error = %v", err)
	}
	_, err = save.Execute(context.Background(), `{"name":"note","description":"second","type":"user","content":"b"}`)
	if err != nil {
		t.Fatalf("second memory_save error = %v", err)
	}

	index = LoadIndex("agent-1")
	if strings.Count(index, "note.md") != 1 {
		t.Fatalf("LoadIndex() = %q, want exactly one line for note.md", index)
	}
	if !strings.Contains(index, "second") {
		t.Fatalf("LoadIndex() = %q, want the updated description", index)
	}
}

func TestMemoryIsScopedPerAgent(t *testing.T) {
	var saveA modules.Tool
	var readB modules.Tool

	var err error

	setupTestMemoryFS(t)

	saveA = Tools("agent-a")[0]
	readB = Tools("agent-b")[1]

	_, err = saveA.Execute(context.Background(), `{"name":"secret","description":"x","type":"user","content":"a-only"}`)
	if err != nil {
		t.Fatalf("memory_save for agent-a error = %v", err)
	}

	if LoadIndex("agent-b") != "" {
		t.Fatalf("agent-b should not see agent-a's memory index")
	}

	_, err = readB.Execute(context.Background(), `{"name":"secret"}`)
	if err == nil {
		t.Fatalf("agent-b should not be able to read agent-a's memory")
	}
}

func TestLoadIndexCapsAtLineLimit(t *testing.T) {
	var save modules.Tool
	var index string
	var lines []string
	var i int

	var err error

	setupTestMemoryFS(t)

	save = Tools("agent-1")[0]

	for i = 0; i < indexMaxLines+20; i++ {
		_, err = save.Execute(context.Background(), `{"name":"note-`+strconv.Itoa(i)+`","description":"x","type":"user","content":"x"}`)
		if err != nil {
			t.Fatalf("memory_save #%d error = %v", i, err)
		}
	}

	index = LoadIndex("agent-1")
	lines = strings.Split(index, "\n")
	if len(lines) > indexMaxLines+3 {
		t.Fatalf("LoadIndex() returned %d lines, want it capped near %d", len(lines), indexMaxLines)
	}
	if !strings.Contains(index, "truncated") {
		t.Fatalf("LoadIndex() = %q, want a truncation notice past the cap", index)
	}
}
