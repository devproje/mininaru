// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestSwitchUserRecognizesEscalation(t *testing.T) {
	var cases map[string]string
	var line string
	var want string
	var user string
	var switched bool

	cases = map[string]string{
		"su":            "root",
		"su -":          "root",
		"su - postgres": "postgres",
		"sudo":          "root",
		"sudo -i":       "root",
		"sudo su":       "root",
		"sudo su -":     "root",
		"sudo -u nginx": "nginx",
	}

	for line, want = range cases {
		user, switched = switchUser(strings.Fields(line))
		if !switched || user != want {
			t.Fatalf("%q: got (%q, %v), want (%q, true)", line, user, switched, want)
		}
	}
}

func TestSwitchUserIgnoresPlainCommands(t *testing.T) {
	var lines []string
	var line string
	var switched bool

	lines = []string{"ls -al", "sudo systemctl restart nginx", "su -c whoami", "sudo -- make install"}

	for _, line = range lines {
		_, switched = switchUser(strings.Fields(line))
		if switched {
			t.Fatalf("%q should not be treated as a user switch", line)
		}
	}
}

func TestQuoteEscapesSingleQuotes(t *testing.T) {
	var got string

	got = quote([]string{"/usr/bin/it's", "shell", "--url", "ws://x"})
	if got != `'/usr/bin/it'\''s' 'shell' '--url' 'ws://x'` {
		t.Fatalf("unexpected quoting: %s", got)
	}
}

func TestEscalateBuildsNestedShellCommand(t *testing.T) {
	var sh state
	var cmd *exec.Cmd
	var target string

	sh = state{cwd: "/tmp", session: "abc"}

	cmd, target = escalate(&sh, []string{"sudo", "-i"})
	if cmd == nil || target != "root" {
		t.Fatalf("sudo -i should escalate to root, got %v %q", cmd, target)
	}

	if strings.Join(cmd.Args[:3], " ") != "sudo -u root" || !strings.Contains(strings.Join(cmd.Args, " "), "shell --url") {
		t.Fatalf("unexpected sudo argv: %v", cmd.Args)
	}

	cmd, target = escalate(&sh, []string{"su", "-", "postgres"})
	if cmd == nil || target != "postgres" || cmd.Args[1] != "postgres" || cmd.Args[2] != "-c" {
		t.Fatalf("unexpected su argv: %v", cmd.Args)
	}

	if !strings.Contains(cmd.Args[3], "'--session' 'abc'") {
		t.Fatalf("session was not carried into the nested shell: %s", cmd.Args[3])
	}

	cmd, _ = escalate(&sh, []string{"ls"})
	if cmd != nil {
		t.Fatalf("plain command should not escalate")
	}
}

func TestDisplayWidthCountsWideRunes(t *testing.T) {
	var cases map[string]int
	var text string
	var want int

	cases = map[string]int{"": 0, "abc": 3, "한글": 4, "한글파일.txt": 12, "ｆｕｌｌ": 8}

	for text, want = range cases {
		if displayWidth(text) != want {
			t.Fatalf("%q: got %d, want %d", text, displayWidth(text), want)
		}
	}
}

func TestPathColorReflectsYoloMode(t *testing.T) {
	var cases map[string]string
	var mode string
	var want string
	var sh state

	cases = map[string]string{"": DIM, "off": DIM, "persist": YELLOW, "on": RED}

	for mode, want = range cases {
		sh = state{yoloMode: mode}
		if pathColor(&sh) != want {
			t.Fatalf("pathColor(%q) = %q, want %q", mode, pathColor(&sh), want)
		}
	}
}

func TestAgentLabelFallsBackWhenNameIsUnknown(t *testing.T) {
	var sh state

	sh = state{}
	if agentLabel(&sh) != "agent" {
		t.Fatalf("empty state should fall back to %q, got %q", "agent", agentLabel(&sh))
	}

	sh = state{agent: "requested"}
	if agentLabel(&sh) != "requested" {
		t.Fatalf("requested agent name should be used, got %q", agentLabel(&sh))
	}

	sh = state{agent: "requested", name: "resolved"}
	if agentLabel(&sh) != "resolved" {
		t.Fatalf("resolved agent name should win, got %q", agentLabel(&sh))
	}
}

func TestBashPathUnixPrefersTheShellEnvVar(t *testing.T) {
	var previous string

	previous = os.Getenv("SHELL")
	t.Cleanup(func() { os.Setenv("SHELL", previous) })

	os.Setenv("SHELL", "/usr/bin/zsh")
	if bashPathUnix() != "/usr/bin/zsh" {
		t.Fatalf("bashPathUnix() = %q, want %q", bashPathUnix(), "/usr/bin/zsh")
	}

	os.Unsetenv("SHELL")
	if bashPathUnix() != "/bin/bash" {
		t.Fatalf("bashPathUnix() with no $SHELL = %q, want %q", bashPathUnix(), "/bin/bash")
	}
}

func TestBashPathWindowsPrefersComspec(t *testing.T) {
	var previous string

	previous = os.Getenv("COMSPEC")
	t.Cleanup(func() { os.Setenv("COMSPEC", previous) })

	os.Setenv("COMSPEC", `C:\Windows\System32\cmd.exe`)
	if bashPathWindows() != `C:\Windows\System32\cmd.exe` {
		t.Fatalf("bashPathWindows() = %q, want the COMSPEC value", bashPathWindows())
	}

	os.Unsetenv("COMSPEC")
	if bashPathWindows() != "cmd.exe" {
		t.Fatalf("bashPathWindows() with no $COMSPEC = %q, want %q", bashPathWindows(), "cmd.exe")
	}
}

func TestShellInvokeFlagMatchesTheHostShellSyntax(t *testing.T) {
	var want string

	want = "-c"
	if runtime.GOOS == "windows" {
		want = "/C"
	}

	if shellInvokeFlag() != want {
		t.Fatalf("shellInvokeFlag() = %q, want %q for %s", shellInvokeFlag(), want, runtime.GOOS)
	}
}
