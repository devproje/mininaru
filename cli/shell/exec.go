// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func changeDir(sh *state, args []string) {
	var target string

	var err error

	target = ""
	if len(args) > 1 {
		target = args[1]
	}

	if target == "" || target == "~" {
		target, err = os.UserHomeDir()
		if err != nil {
			notice(RED, "✖", "cd: %v", err)
			return
		}
	}

	err = os.Chdir(target)
	if err != nil {
		notice(RED, "✖", "cd: %v", err)
		return
	}

	sh.cwd, err = os.Getwd()
	if err != nil {
		notice(RED, "✖", "cd: %v", err)
		return
	}

	refreshGitBranch(sh)

	if sh.conn != nil {
		refreshYoloMode(sh)
	}
}

func bashPathUnix() string {
	var name string

	name = os.Getenv("SHELL")
	if name == "" {
		name = "/bin/bash"
	}

	return name
}

func bashPathWindows() string {
	var name string

	name = os.Getenv("COMSPEC")
	if name == "" {
		name = "cmd.exe"
	}

	return name
}

func bashPath() string {
	if runtime.GOOS == "windows" {
		return bashPathWindows()
	}

	return bashPathUnix()
}

func shellInvokeFlags() []string {
	if runtime.GOOS == "windows" {
		return []string{"/C"}
	}

	return []string{"-i", "-c"}
}

func exitCode(cmd *exec.Cmd) int {
	if cmd.ProcessState == nil {
		return -1
	}

	return cmd.ProcessState.ExitCode()
}

func runBash(sh *state, line string) {
	var cmd *exec.Cmd
	var exitErr *exec.ExitError
	var args []string

	var err error

	args = append(shellInvokeFlags(), line)
	cmd = exec.Command(bashPath(), args...)
	cmd.Dir = sh.cwd
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = runForeground(cmd)
	sh.lastExitCode = exitCode(cmd)

	if err != nil && !errors.As(err, &exitErr) {
		notice(RED, "✖", "%s%v%s", DIM, err, RESET)
	}
}

func selfArgs(sh *state) []string {
	var self string
	var argv []string

	var err error

	self, err = os.Executable()
	if err != nil {
		return nil
	}

	argv = []string{self, "shell", "--url", sh.url}

	if sh.session != "" {
		argv = append(argv, "--session", sh.session)
	}

	if sh.agent != "" {
		argv = append(argv, "--agent", sh.agent)
	}

	return argv
}

func quote(argv []string) string {
	var arg string
	var quoted []string

	for _, arg = range argv {
		quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", `'\''`)+"'")
	}

	return strings.Join(quoted, " ")
}

func switchUser(args []string) (string, bool) {
	var arg string
	var user string

	var rest []string
	var sawSu bool
	var bare bool
	var i int

	if runtime.GOOS == "windows" {
		return "", false
	}

	if args[0] != "su" && args[0] != "sudo" {
		return "", false
	}

	user = "root"
	rest = args[1:]

	for i = 0; i < len(rest); i++ {
		arg = rest[i]

		switch arg {
		case "-", "-l", "-i", "-s", "--login":
			continue
		case "-u", "--user":
			if i+1 >= len(rest) {
				return "", false
			}

			i++
			user = rest[i]
			continue
		}

		if strings.HasPrefix(arg, "-") {
			return "", false
		}

		if args[0] == "sudo" && !sawSu && arg == "su" {
			sawSu = true
			continue
		}

		if args[0] == "sudo" && !sawSu {
			return "", false
		}

		if bare {
			return "", false
		}

		bare = true
		user = arg
	}

	return user, true
}

func escalate(sh *state, args []string) (*exec.Cmd, string) {
	var user string
	var switched bool
	var argv []string
	var cmd *exec.Cmd

	user, switched = switchUser(args)
	if !switched {
		return nil, ""
	}

	argv = selfArgs(sh)
	if argv == nil {
		return nil, ""
	}

	if args[0] == "sudo" {
		cmd = exec.Command("sudo", append([]string{"-u", user, "--"}, argv...)...)
	} else {
		cmd = exec.Command("su", user, "-c", quote(argv))
	}

	cmd.Dir = sh.cwd
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, user
}

func runNested(cmd *exec.Cmd, target string) {
	var err error

	notice(YELLOW, "⇅", "switching to %s%s%s %s", BOLD, target, RESET, DIM+"exit the nested shell to come back"+RESET)

	err = runForeground(cmd)
	if err != nil {
		notice(RED, "✖", "%s%v%s", DIM, err, RESET)
	}
}
