// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"
)

func completeUntil(t *testing.T, sh *state, line, want string) []string {
	var items []string
	var res completion
	var deadline time.Time

	if sh.bashComp == nil {
		startBashComplete(sh)
		t.Cleanup(func() { stopBashComplete(sh) })
	}

	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		sh.completeCache = completeCache{}
		res, _ = bashComplete(sh, line)
		items = res.items
		if slices.Contains(items, want) {
			return items
		}

		time.Sleep(200 * time.Millisecond)
	}

	return items
}

func bashCompletionAvailable(t *testing.T) {
	var path string

	var err error

	if !isBash(bashPath()) {
		t.Skip("host shell is not bash")
	}

	_, err = os.Stat("/usr/share/bash-completion/bash_completion")
	if err != nil {
		_, err = os.Stat("/etc/bash_completion")
		if err != nil {
			t.Skip("bash-completion is not installed")
		}
	}

	path, err = exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}

	_ = path
}

func gitRepoWithBranch(t *testing.T) string {
	var dir string
	var step [][]string
	var args []string
	var cmd *exec.Cmd

	var err error

	dir = t.TempDir()
	step = [][]string{
		{"git", "init", "-q"},
		{"git", "config", "user.email", "t@example.com"},
		{"git", "config", "user.name", "t"},
		{"git", "commit", "-q", "--allow-empty", "-m", "init"},
		{"git", "branch", "wip-feature"},
	}

	for _, args = range step {
		cmd = exec.Command(args[0], args[1:]...)
		cmd.Dir = dir

		err = cmd.Run()
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}

	return dir
}

func TestBashCompleteOffersGitRefs(t *testing.T) {
	var sh state
	var items []string

	bashCompletionAvailable(t)

	sh.cwd = gitRepoWithBranch(t)

	items = completeUntil(t, &sh, "git checkout ", "wip-feature")
	if !slices.Contains(items, "wip-feature") {
		t.Fatalf("git checkout completion = %v, want it to contain \"wip-feature\"", items)
	}
}

func TestBashCompleteOffersSubcommands(t *testing.T) {
	var sh state
	var items []string

	bashCompletionAvailable(t)

	sh.cwd = gitRepoWithBranch(t)

	items = completeUntil(t, &sh, "git chec", "checkout")
	if !slices.Contains(items, "checkout") {
		t.Fatalf("git subcommand completion = %v, want it to contain \"checkout\"", items)
	}
}

func TestBashCompleteCachesByLine(t *testing.T) {
	var sh state
	var got completion
	var ok bool

	bashCompletionAvailable(t)

	sh.cwd = gitRepoWithBranch(t)
	sh.completeCache.line = "git checkout "
	sh.completeCache.res = completion{items: []string{"sentinel"}}
	sh.completeCache.ok = true

	got, ok = bashComplete(&sh, "git checkout ")
	if !ok || !slices.Equal(got.items, []string{"sentinel"}) {
		t.Fatalf("bashComplete should return the cached result for an unchanged line, got %v (ok=%v)", got, ok)
	}
}

func TestDedupeTrimsAndSorts(t *testing.T) {
	var got []string

	got = dedupe([]string{"master ", "master ", "main", ""}, true)
	if !slices.Equal(got, []string{"main", "master"}) {
		t.Fatalf("dedupe = %v, want [main master]", got)
	}

	got = dedupe([]string{"b", "a", "b"}, false)
	if !slices.Equal(got, []string{"b", "a"}) {
		t.Fatalf("dedupe(nosort) = %v, want [b a]", got)
	}
}

func TestParseCompReplyHeader(t *testing.T) {
	var c completion

	c = parseCompReply([]string{"\x02nf\tfo", "format", "format:"})
	if c.replace != 2 || !c.noSpace || !c.filenames || c.noSort {
		t.Fatalf("flags: replace=%d noSpace=%v filenames=%v noSort=%v", c.replace, c.noSpace, c.filenames, c.noSort)
	}
	if !slices.Equal(c.items, []string{"format", "format:"}) {
		t.Fatalf("items = %v", c.items)
	}
}

func TestCompleteAnchorsAfterEquals(t *testing.T) {
	var sh state
	var line string

	bashCompletionAvailable(t)
	sh.mode = MODE_SHELL
	sh.cwd = gitRepoWithBranch(t)

	completeUntil(t, &sh, "git log --pretty=m", "medium")

	sh.completeCache = completeCache{}
	line, _, _ = complete(&sh, "git log --pretty=me", false)
	if line != "git log --pretty=medium" {
		t.Fatalf("complete = %q, want %q", line, "git log --pretty=medium")
	}
}

func TestBashCompleteRespawnsAfterKill(t *testing.T) {
	var sh state
	var items []string
	var ok bool

	bashCompletionAvailable(t)
	sh.cwd = gitRepoWithBranch(t)

	items = completeUntil(t, &sh, "git chec", "checkout")
	if !slices.Contains(items, "checkout") {
		t.Fatalf("warm-up completion = %v, want \"checkout\"", items)
	}

	sh.bashComp.mu.Lock()
	sh.bashComp.kill()
	sh.bashComp.mu.Unlock()
	sh.bashComp.broken.Store(true)

	sh.completeCache = completeCache{}
	_, ok = bashComplete(&sh, "git chec")
	if ok {
		t.Fatal("bashComplete on dead proc should report not-ok (fallback)")
	}

	items = completeUntil(t, &sh, "git chec", "checkout")
	if !slices.Contains(items, "checkout") {
		t.Fatalf("post-respawn completion = %v, want \"checkout\"", items)
	}
}
