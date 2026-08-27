// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"context"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const bashCompleteTimeout time.Duration = 2 * time.Second

const bashCompleteDriver = `
BC=/usr/share/bash-completion/bash_completion
[ -r "$BC" ] || BC=/etc/bash_completion
[ -r "$BC" ] && . "$BC" >/dev/null 2>&1
line=$1
COMP_LINE=$line
COMP_POINT=${#line}
IFS=$' \t\n' read -ra COMP_WORDS <<<"$line"
[[ $line =~ [[:space:]]$ ]] && COMP_WORDS+=("")
[ ${#COMP_WORDS[@]} -eq 0 ] && exit 0
COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
cmd=${COMP_WORDS[0]}
declare -F _completion_loader >/dev/null 2>&1 && _completion_loader "$cmd" >/dev/null 2>&1
spec=$(complete -p "$cmd" 2>/dev/null) || exit 0
cur=${COMP_WORDS[COMP_CWORD]}
prev=""
[ $COMP_CWORD -gt 0 ] && prev=${COMP_WORDS[COMP_CWORD-1]}
COMPREPLY=()
if [[ " $spec " == *" -F "* ]]; then
	fn=${spec#*-F }
	fn=${fn%% *}
	"$fn" "$cmd" "$cur" "$prev" >/dev/null 2>&1
elif [[ " $spec " == *" -C "* ]]; then
	ext=${spec#*-C }
	ext=${ext%% *}
	ext=${ext#\'}
	ext=${ext%\'}
	while IFS= read -r reply; do
		COMPREPLY+=("$reply")
	done < <(COMP_LINE=$COMP_LINE COMP_POINT=$COMP_POINT "$ext" "$cmd" "$cur" "$prev" 2>/dev/null)
else
	exit 0
fi
printf '%s\n' "${COMPREPLY[@]}"
`

func parseBashCompletions(out []byte) []string {
	var seen map[string]bool
	var items []string
	var line string
	var entry string

	seen = map[string]bool{}

	for _, line = range strings.Split(string(out), "\n") {
		entry = strings.TrimRight(line, " \t")
		if entry == "" || seen[entry] {
			continue
		}

		seen[entry] = true
		items = append(items, entry)
	}

	sort.Strings(items)

	return items
}

func warmBashComplete() {
	if !isBash(bashPath()) {
		return
	}

	go func() {
		var ctx context.Context
		var cancel context.CancelFunc
		var cmd *exec.Cmd

		ctx, cancel = context.WithTimeout(context.Background(), bashCompleteTimeout)
		defer cancel()

		cmd = exec.CommandContext(ctx, bashPath(), "-c", bashCompleteDriver, "bash", "true ")
		cmd.Env = os.Environ()
		cmd.Run()
	}()
}

func bashComplete(sh *state, line string) []string {
	var ctx context.Context
	var cancel context.CancelFunc
	var cmd *exec.Cmd
	var out []byte

	var err error

	if !isBash(bashPath()) {
		return nil
	}

	if sh.completeCache.line == line {
		return sh.completeCache.items
	}

	ctx, cancel = context.WithTimeout(context.Background(), bashCompleteTimeout)
	defer cancel()

	cmd = exec.CommandContext(ctx, bashPath(), "-c", bashCompleteDriver, "bash", line)
	cmd.Dir = sh.cwd
	cmd.Env = os.Environ()

	out, err = cmd.Output()
	if err != nil {
		return nil
	}

	sh.completeCache.line = line
	sh.completeCache.items = parseBashCompletions(out)

	return sh.completeCache.items
}
