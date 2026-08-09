package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

var rootOnce sync.Once

func runRoot(t *testing.T, args ...string) (string, error) {
	var out bytes.Buffer

	var err error

	t.Helper()

	rootOnce.Do(rootInit)

	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)

	defer root.SetArgs(nil)

	err = root.Execute()

	return out.String(), err
}

func TestHelpListsConvertedSubcommands(t *testing.T) {
	var cases map[string][]string
	var parent string
	var verbs []string
	var verb string
	var help string

	var err error

	cases = map[string][]string{
		"mcp":    {"list", "add", "remove", "enable", "disable"},
		"tools":  {"enable", "disable", "list"},
		"skill":  {"list", "show"},
		"web":    {"show", "provider", "endpoint", "key"},
		"daemon": {"install", "reload", "uninstall"},
	}

	for parent, verbs = range cases {
		help, err = runRoot(t, parent, "--help")
		if err != nil {
			t.Fatalf("%s --help failed: %v", parent, err)
		}

		for _, verb = range verbs {
			if !strings.Contains(help, verb) {
				t.Fatalf("%s --help does not list %q:\n%s", parent, verb, help)
			}
		}
	}
}

func TestMcpAddFlagsStayOnAdd(t *testing.T) {
	var help string

	var err error

	help, err = runRoot(t, "mcp", "add", "--help")
	if err != nil {
		t.Fatalf("mcp add --help failed: %v", err)
	}
	if !strings.Contains(help, "--stdio") {
		t.Fatalf("mcp add --help lost --stdio:\n%s", help)
	}

	help, err = runRoot(t, "mcp", "remove", "--help")
	if err != nil {
		t.Fatalf("mcp remove --help failed: %v", err)
	}
	if strings.Contains(help, "--stdio") {
		t.Fatalf("mcp remove --help still offers --stdio:\n%s", help)
	}
}

func TestUnexpectedArgumentsAreUsageErrors(t *testing.T) {
	var invocations [][]string
	var args []string

	var err error

	invocations = [][]string{
		{"serve", "nonsense"},
		{"provider", "list", "nonsense"},
		{"tools", "list", "nonsense"},
	}

	for _, args = range invocations {
		_, err = runRoot(t, args...)
		if err == nil {
			t.Fatalf("%v accepted an unexpected argument", args)
		}

		if exitCode(err) != exitUsage {
			t.Fatalf("%v mapped to exit code %d, expected %d (%v)", args, exitCode(err), exitUsage, err)
		}
	}
}

func TestUnknownFlagIsAUsageError(t *testing.T) {
	var err error

	_, err = runRoot(t, "serve", "--nope")
	if err == nil {
		t.Fatal("unknown flag was accepted")
	}

	if exitCode(err) != exitUsage {
		t.Fatalf("unknown flag mapped to exit code %d, expected %d", exitCode(err), exitUsage)
	}
}

func TestDangerousToolsFlagIsPersistent(t *testing.T) {
	var target *cobra.Command

	rootOnce.Do(rootInit)

	for _, target = range []*cobra.Command{serve, mcpConfig, toolsConfig} {
		if target.InheritedFlags().Lookup("allow-dangerous-tools") == nil {
			t.Fatalf("%s cannot parse --allow-dangerous-tools", target.Name())
		}
	}
}

func TestCommandsCarryHelpText(t *testing.T) {
	var pending []*cobra.Command
	var current *cobra.Command

	rootOnce.Do(rootInit)

	pending = append(pending, root)

	for len(pending) > 0 {
		current = pending[0]
		pending = append(pending[1:], current.Commands()...)

		if current.Name() == "help" || current.Name() == "completion" {
			continue
		}

		if current.Short == "" {
			t.Fatalf("%s has no Short description", current.CommandPath())
		}
	}
}
