// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type bashCompProc struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	out    *bufio.Reader
	ready  atomic.Bool
	broken atomic.Bool
}

type compReply struct {
	lines []string
	ok    bool
}

const (
	queryTimeout time.Duration = 250 * time.Millisecond
	warmTimeout  time.Duration = 5 * time.Second
	compSentinel string        = "\x01"
)

const bashCompleteLoop = `
BC=/usr/share/bash-completion/bash_completion
[ -r "$BC" ] || BC=/etc/bash_completion
[ -r "$BC" ] && . "$BC" >/dev/null 2>&1
while IFS= read -r cwd && IFS= read -r line; do
	cd "$cwd" 2>/dev/null
	COMP_LINE=$line
	COMP_POINT=${#line}
	IFS=$' \t\n' read -ra COMP_WORDS <<<"$line"
	[[ $line =~ [[:space:]]$ ]] && COMP_WORDS+=("")
	cur=""
	opts=""
	COMPREPLY=()
	if [ ${#COMP_WORDS[@]} -gt 0 ]; then
		COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
		cmd=${COMP_WORDS[0]}
		word=${COMP_WORDS[COMP_CWORD]}
		cur=${word##*[:=]}
		prev=""
		[ $COMP_CWORD -gt 0 ] && prev=${COMP_WORDS[COMP_CWORD-1]}
		declare -F _completion_loader >/dev/null 2>&1 && _completion_loader "$cmd" >/dev/null 2>&1
		spec=$(complete -p "$cmd" 2>/dev/null)
		[[ " $spec " == *" -o nospace "*   ]] && opts+=n
		[[ " $spec " == *" -o filenames "* ]] && opts+=f
		[[ " $spec " == *" -o nosort "*    ]] && opts+=s
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
		fi
	fi
	printf '\002%s\t%s\n' "$opts" "$cur"
	printf '%s\n' "${COMPREPLY[@]}"
	printf '\001\n'
done
`

func dedupe(raw []string, sortItems bool) []string {
	var seen map[string]bool
	var items []string
	var entry string

	seen = map[string]bool{}

	for _, entry = range raw {
		entry = strings.TrimRight(entry, " \t")
		if entry == "" || seen[entry] {
			continue
		}

		seen[entry] = true
		items = append(items, entry)
	}

	if sortItems {
		sort.Strings(items)
	}

	return items
}

func parseCompReply(lines []string) completion {
	var c completion
	var opts string
	var cur string

	if len(lines) > 0 && strings.HasPrefix(lines[0], "\x02") {
		opts, cur, _ = strings.Cut(strings.TrimPrefix(lines[0], "\x02"), "\t")
		lines = lines[1:]
	}

	c.replace = len(cur)
	c.noSpace = strings.Contains(opts, "n")
	c.filenames = strings.Contains(opts, "f")
	c.noSort = strings.Contains(opts, "s")
	c.items = dedupe(lines, !c.noSort)

	return c
}

func (p *bashCompProc) kill() {
	if p.cmd == nil {
		return
	}

	if p.stdin != nil {
		p.stdin.Close()
	}

	p.cmd.Process.Kill()
	p.cmd.Wait()
	p.cmd, p.stdin, p.out = nil, nil, nil
	p.ready.Store(false)
}

func (p *bashCompProc) query(dir, line string, timeout time.Duration) (completion, bool) {
	var done chan compReply
	var reply compReply
	var out *bufio.Reader

	var err error

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || p.broken.Load() {
		return completion{}, false
	}

	_, err = io.WriteString(p.stdin, dir+"\n"+line+"\n")
	if err != nil {
		p.broken.Store(true)
		return completion{}, false
	}

	done = make(chan compReply, 1)
	out = p.out

	go func() {
		var lines []string
		var text string

		var readErr error

		for {
			text, readErr = out.ReadString('\n')
			if readErr != nil {
				done <- compReply{ok: false}
				return
			}

			if strings.TrimRight(text, "\r\n") == compSentinel {
				done <- compReply{lines: lines, ok: true}
				return
			}

			lines = append(lines, strings.TrimRight(text, "\r\n"))
		}
	}()

	select {
	case reply = <-done:
		if !reply.ok {
			p.broken.Store(true)
			return completion{}, false
		}

		return parseCompReply(reply.lines), true
	case <-time.After(timeout):
		p.broken.Store(true)
		return completion{}, false
	}
}

func (p *bashCompProc) spawn(dir string) {
	var cmd *exec.Cmd
	var stdin io.WriteCloser
	var stdout io.ReadCloser

	var err error

	p.mu.Lock()
	p.kill()

	cmd = exec.Command(bashPath(), "-c", bashCompleteLoop)
	cmd.Env = os.Environ()

	stdin, err = cmd.StdinPipe()
	if err != nil {
		p.mu.Unlock()
		return
	}

	stdout, err = cmd.StdoutPipe()
	if err != nil {
		p.mu.Unlock()
		return
	}

	err = cmd.Start()
	if err != nil {
		p.mu.Unlock()
		return
	}

	p.cmd, p.stdin, p.out = cmd, stdin, bufio.NewReader(stdout)
	p.mu.Unlock()

	p.broken.Store(false)
	p.query(dir, "true ", warmTimeout)
	p.ready.Store(true)
}

func startBashComplete(sh *state) {
	if !isBash(bashPath()) {
		return
	}

	sh.bashComp = &bashCompProc{}
	go sh.bashComp.spawn(sh.cwd)
}

func stopBashComplete(sh *state) {
	if sh.bashComp == nil {
		return
	}

	sh.bashComp.mu.Lock()
	sh.bashComp.kill()
	sh.bashComp.mu.Unlock()
}

func bashComplete(sh *state, line string) (completion, bool) {
	if sh.completeCache.line == line {
		return sh.completeCache.res, sh.completeCache.ok
	}

	if sh.bashComp == nil {
		return completion{}, false
	}

	if sh.bashComp.broken.Load() {
		go sh.bashComp.spawn(sh.cwd)
		return completion{}, false
	}

	if !sh.bashComp.ready.Load() {
		return completion{}, false
	}

	sh.completeCache.line = line
	sh.completeCache.res, sh.completeCache.ok = sh.bashComp.query(sh.cwd, line, queryTimeout)

	return sh.completeCache.res, sh.completeCache.ok
}
