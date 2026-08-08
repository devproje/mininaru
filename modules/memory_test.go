package modules

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devproje/mininaru/util"
)

func TestMemoryCRUDAndSnapshot(t *testing.T) {
	var def Def
	var result string
	var entries []memoryEntry
	var snapshot string

	var err error

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	def = Memory()

	result, err = def.Execute(context.Background(), `{"action":"add","content":"User likes Go"}`)
	if err != nil || !strings.Contains(result, "User likes Go") {
		t.Fatalf("add result=%q err=%v", result, err)
	}
	entries, err = memoryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%#v err=%v", entries, err)
	}

	result, err = def.Execute(context.Background(), `{"action":"replace","id":"`+entries[0].Id+`","content":"User prefers Go"}`)
	if err != nil || !strings.Contains(result, "User prefers Go") {
		t.Fatalf("replace result=%q err=%v", result, err)
	}
	snapshot = MemorySnapshot()
	if snapshot != "- User prefers Go" {
		t.Fatalf("snapshot=%q", snapshot)
	}

	_, err = def.Execute(context.Background(), `{"action":"remove","id":"`+entries[0].Id+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	if MemorySnapshot() != "" {
		t.Fatalf("removed memory remains: %q", MemorySnapshot())
	}
}

func TestSafeToolsExcludeMemory(t *testing.T) {
	var def Def

	for _, def = range SafeTools() {
		if def.Name == MemoryToolName {
			t.Fatal("safe daemon tools exposed privileged memory")
		}
	}
}
