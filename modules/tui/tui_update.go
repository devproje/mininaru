// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devproje/mininaru/modules/client"
)

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var wsize tea.WindowSizeMsg
	var ok bool
	var kmsg tea.KeyMsg
	var imagePath string
	var pastedImage bool
	var suggestions []tuiCmdInfo
	var chunkMsg tuiChunkMsg
	var toolMsg tuiToolMsg
	var summary string
	var messageMsg tuiMessageMsg
	var promptMsg tuiPromptMsg
	var doneMsg tuiDoneMsg
	var cmdResultMsg tuiCmdResultMsg
	var modeSyncedMsg tuiModeSyncedMsg
	var sessionSwitchMsg tuiSessionSwitchMsg
	var gatewaySwitchMsg tuiGatewaySwitchMsg
	var clipMsg tuiClipboardResolvedMsg
	var imgUploadedMsg tuiImgUploadedMsg
	var text string
	var prevValue string
	var cmd tea.Cmd
	var cmds []tea.Cmd

	var err error

	wsize, ok = msg.(tea.WindowSizeMsg)
	if ok {
		m.width = wsize.Width
		m.height = wsize.Height
		m.layout()
	}

	kmsg, ok = msg.(tea.KeyMsg)
	if ok {
		if !m.input.Focused() {
			switch kmsg.String() {
			case "up", "down", "pgup", "pgdown", "esc", "ctrl+c", "ctrl+d":
			default:
				m.input.Focus()
			}
		}

		if kmsg.Paste && m.input.Focused() && m.base != "" {
			imagePath, pastedImage = extractPastedImagePath(string(kmsg.Runes))
			if pastedImage {
				return m, m.queueImage(imagePath, false)
			}
		}

		switch kmsg.String() {
		case "ctrl+d":
			return m, tea.Quit
		case "ctrl+c":
			if m.busy && m.conn != nil {
				m.conn.WriteJSON(client.Frame{Type: "interrupt", SessionId: m.session})

				return m, nil
			}

			return m, tea.Quit
		case "ctrl+v":
			if m.base == "" {
				return m, nil
			}

			return m, resolveClipboardImageCmd()
		case "esc":
			if m.input.Focused() {
				m.input.Blur()
			} else {
				m.input.Focus()
			}

			return m, nil
		case "up":
			if m.awaiting != "" && len(m.awaitOptions) > 0 {
				if m.awaitCursor > 0 {
					m.awaitCursor--
				}

				return m, nil
			}

			if m.input.Focused() {
				suggestions = tuiCmdSuggestions(m.input.Value())
				if m.awaiting == "" && len(suggestions) > 0 && !tuiCmdExact(m.input.Value()) {
					if m.cmdCursor > 0 {
						m.cmdCursor--
					}

					return m, nil
				}

				if m.awaiting == "" && len(m.history) > 0 {
					if m.historyPos == -1 {
						m.historyDraft = m.input.Value()
						m.historyPos = len(m.history) - 1
					} else if m.historyPos > 0 {
						m.historyPos--
					}

					m.input.SetValue(m.history[m.historyPos])
					m.input.CursorEnd()

					return m, nil
				}
			}
		case "down":
			if m.awaiting != "" && len(m.awaitOptions) > 0 {
				if m.awaitCursor < len(m.awaitOptions)-1 {
					m.awaitCursor++
				}

				return m, nil
			}

			if m.input.Focused() {
				suggestions = tuiCmdSuggestions(m.input.Value())
				if m.awaiting == "" && len(suggestions) > 0 && !tuiCmdExact(m.input.Value()) {
					if m.cmdCursor < len(suggestions)-1 {
						m.cmdCursor++
					}

					return m, nil
				}

				if m.awaiting == "" && m.historyPos != -1 {
					if m.historyPos < len(m.history)-1 {
						m.historyPos++
						m.input.SetValue(m.history[m.historyPos])
					} else {
						m.historyPos = -1
						m.input.SetValue(m.historyDraft)
						m.historyDraft = ""
					}

					m.input.CursorEnd()

					return m, nil
				}
			}
		case "shift+tab":
			if m.base == "" {
				return m, nil
			}

			m.mode = nextMode(m.mode)
			m.layout()

			return m, pushModeCmd(m.base, m.apiKey, m.cwd, m.mode)
		case "tab":
			suggestions = tuiCmdSuggestions(m.input.Value())
			if m.awaiting == "" && len(suggestions) > 0 && !tuiCmdExact(m.input.Value()) {
				if m.cmdCursor >= len(suggestions) {
					m.cmdCursor = 0
				}

				m.input.SetValue("/" + suggestions[m.cmdCursor].name + " ")
				m.input.CursorEnd()

				return m, nil
			}
		case "enter":
			if m.awaiting != "" && len(m.awaitOptions) > 0 {
				return m, m.answerCmd(m.awaitOptions[m.awaitCursor])
			}

			return m.submit()
		}
	}

	_, ok = msg.(tuiSpinMsg)
	if ok {
		if !m.busy || m.awaiting != "" {
			return m, nil
		}

		m.spinTick++

		return m, spinTickCmd()
	}

	_, ok = msg.(tuiAnsweredMsg)
	if ok {
		m.awaiting = ""
		m.awaitOptions = nil
		m.awaitLabels = nil
		m.awaitCursor = 0
		m.newLine()
		m.layout()

		if m.busy {
			return m, spinTickCmd()
		}

		return m, nil
	}

	chunkMsg, ok = msg.(tuiChunkMsg)
	if ok {
		if chunkMsg.reasoning != "" && m.streamMode != "reasoning" {
			m.newLine()
			m.streamMode = "reasoning"
		}

		if chunkMsg.content != "" && m.streamMode != "content" {
			m.newLine()
			m.streamMode = "content"
		}

		m.appendStream(chunkMsg.reasoning, chunkMsg.content)
		m.layout()

		return m, nil
	}

	toolMsg, ok = msg.(tuiToolMsg)
	if ok {
		if toolMsg.status == "mode" {
			m.mode = toolMsg.message
			m.layout()

			return m, nil
		}

		if toolMsg.status == "started" {
			m.newLine()
			m.appendText(fmt.Sprintf("%s %s%s%s %s", client.StatusDot(toolMsg.status), client.WHITE, toolMsg.name, client.RESET, "running"))
			m.activeToolLine = len(m.lines) - 1
			m.activeToolName = toolMsg.name
			m.layout()

			return m, nil
		}

		summary = toolSummary(toolMsg.name, toolMsg.status, toolMsg.message)
		if summary != "" {
			if m.activeToolLine >= 0 && m.activeToolLine < len(m.lines) && m.activeToolName == toolMsg.name {
				m.lines[m.activeToolLine] = summary
			} else {
				m.newLine()
				m.appendText(summary)
			}

			if client.LooksLikeDiff(toolMsg.message) {
				m.newLine()
				m.appendText(strings.TrimRight(client.FormatDiff(toolMsg.message), "\n"))
			} else if strings.Contains(toolMsg.message, "\n") {
				m.newLine()
				m.appendText(dimEachLine(toolMsg.message))
			}

			m.activeToolLine = -1
			m.activeToolName = ""
			m.streamMode = ""
		}

		m.layout()

		return m, nil
	}

	messageMsg, ok = msg.(tuiMessageMsg)
	if ok {
		m.newLine()
		m.appendText(fmt.Sprintf("%s←%s %s%s %s", client.GRAY, client.RESET, messageMsg.name, client.RESET, messageMsg.message))
		m.streamMode = ""
		m.layout()

		return m, nil
	}

	promptMsg, ok = msg.(tuiPromptMsg)
	if ok {
		m.awaiting = promptMsg.kind
		m.awaitCursor = 0
		m.streamMode = ""
		m.newLine()

		if promptMsg.kind == "approval" {
			m.awaitOptions = []string{"once", "session", "deny"}
			m.awaitLabels = []string{
				"Yes",
				fmt.Sprintf("Yes, don't ask again for %s this session", promptMsg.name),
				"No",
			}
			m.appendText(fmt.Sprintf("%s●%s %s%s%s", client.GREEN, client.RESET, client.BOLD, formatToolCall(promptMsg.name, promptMsg.arguments), client.RESET))
			m.newLine()
			m.appendText(fmt.Sprintf("%sin %s%s", client.DIM, promptMsg.cwd, client.RESET))
			m.newLine()
			m.appendText(fmt.Sprintf("%sDo you want to proceed?%s", client.GRAY, client.RESET))
		} else {
			m.awaitOptions = promptMsg.options
			m.awaitLabels = promptMsg.options
			m.appendText(fmt.Sprintf("%s%s%s", client.PURPLE, promptMsg.question, client.RESET))
		}

		m.layout()

		return m, nil
	}

	doneMsg, ok = msg.(tuiDoneMsg)
	if ok {
		m.busy = false
		m.streamMode = ""
		m.appendText(m.md.Flush())
		m.md.Reset()

		if doneMsg.errText != "" {
			m.newLine()
			m.appendText(fmt.Sprintf("%s✗ %s%s", client.RED, doneMsg.errText, client.RESET))
		}

		m.newLine()
		m.layout()

		return m, tea.Batch(m.flushQueued(), refreshModeCmd(m.base, m.apiKey, m.cwd))
	}

	cmdResultMsg, ok = msg.(tuiCmdResultMsg)
	if ok {
		m.landAsyncResult(cmdResultMsg.label, cmdResultMsg.lines)
		m.layout()

		return m, m.flushQueued()
	}

	modeSyncedMsg, ok = msg.(tuiModeSyncedMsg)
	if ok {
		if modeSyncedMsg.err != nil {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ mode change failed: %s%s", client.RED, modeSyncedMsg.err, client.RESET))
		} else {
			m.mode = modeSyncedMsg.mode
		}

		m.layout()

		return m, nil
	}

	sessionSwitchMsg, ok = msg.(tuiSessionSwitchMsg)
	if ok {
		if sessionSwitchMsg.err != nil {
			text = fmt.Sprintf("%s✗ %s%s", client.RED, sessionSwitchMsg.err, client.RESET)
		} else {
			err = m.conn.WriteJSON(client.Frame{Type: "attach", SessionId: sessionSwitchMsg.found.Id})
			if err != nil {
				text = fmt.Sprintf("%s✗ %s%s", client.RED, err, client.RESET)
			} else {
				m.session = sessionSwitchMsg.found.Id
				text = fmt.Sprintf("%s⇄%s switched to %s %s(%s)%s", client.BLUE, client.RESET, sessionSwitchMsg.found.Id, client.DIM, sessionSwitchMsg.found.Name, client.RESET)
			}
		}

		m.landAsyncResult("session", []string{text})
		m.layout()

		return m, m.flushQueued()
	}

	gatewaySwitchMsg, ok = msg.(tuiGatewaySwitchMsg)
	if ok {
		if gatewaySwitchMsg.err != nil {
			text = fmt.Sprintf("%s✗ %s%s", client.RED, gatewaySwitchMsg.err, client.RESET)
		} else {
			if m.conn != nil {
				m.conn.Close()
			}

			m.conn = gatewaySwitchMsg.conn
			m.base = gatewaySwitchMsg.base
			m.apiKey = gatewaySwitchMsg.apiKey
			m.agent = gatewaySwitchMsg.agentName
			m.agentId = gatewaySwitchMsg.agentId
			m.session = gatewaySwitchMsg.sessionId

			if m.restartPump != nil {
				m.restartPump(gatewaySwitchMsg.conn)
			}

			text = fmt.Sprintf("%s⇄%s %s %s(%s @ %s)%s", client.BLUE, client.RESET, m.agent, client.DIM, m.session, gatewaySwitchMsg.targetName, client.RESET)
		}

		m.landAsyncResult("gateway", []string{text})
		m.layout()

		return m, m.flushQueued()
	}

	clipMsg, ok = msg.(tuiClipboardResolvedMsg)
	if ok {
		if clipMsg.err != nil {
			m.newLine()
			m.appendText(fmt.Sprintf("%s✗ %s%s", client.RED, clipMsg.err, client.RESET))
			m.newLine()
			m.layout()

			return m, nil
		}

		return m, m.queueImage(clipMsg.path, clipMsg.cleanup)
	}

	imgUploadedMsg, ok = msg.(tuiImgUploadedMsg)
	if ok {
		if imgUploadedMsg.err != nil {
			m.newLine()
			m.appendText(fmt.Sprintf("%s✗ %s%s", client.RED, imgUploadedMsg.err, client.RESET))
			m.newLine()
		} else {
			m.pending = append(m.pending, imgUploadedMsg.id)
		}

		m.layout()

		return m, nil
	}

	prevValue = m.input.Value()

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	if m.historyPos != -1 && m.input.Value() != prevValue {
		m.historyPos = -1
		m.historyDraft = ""
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
