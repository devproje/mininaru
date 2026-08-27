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
	var deadline time.Time

	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		sh.completeCache = completeCache{}
		items = bashComplete(sh, line)
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

	bashCompletionAvailable(t)

	sh.cwd = gitRepoWithBranch(t)
	sh.completeCache.line = "git checkout "
	sh.completeCache.items = []string{"sentinel"}

	if !slices.Equal(bashComplete(&sh, "git checkout "), []string{"sentinel"}) {
		t.Fatal("bashComplete should return the cached items for an unchanged line")
	}
}

func TestParseBashCompletionsDedupesAndTrims(t *testing.T) {
	var got []string

	got = parseBashCompletions([]byte("master \nmaster \nmain\n\n"))
	if !slices.Equal(got, []string{"main", "master"}) {
		t.Fatalf("parseBashCompletions = %v, want [main master]", got)
	}
}
