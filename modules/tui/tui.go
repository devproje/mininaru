// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules/client"
	"github.com/devproje/mininaru/util"
	"github.com/gorilla/websocket"
)

type tuiSpinMsg struct{}

type tuiModel struct {
	viewport       viewport.Model
	input          textarea.Model
	spinTick       int
	agent          string
	session        string
	width          int
	height         int
	ready          bool
	lines          []string
	conn           *websocket.Conn
	cwd            string
	mode           string
	noCache        bool
	busy           bool
	awaiting       string
	awaitOptions   []string
	awaitLabels    []string
	awaitCursor    int
	answers        chan string
	md             client.MdRenderer
	base           string
	apiKey         string
	cmdCursor      int
	agentId        string
	gateways       []client.Gateway
	restartPump    func(*websocket.Conn)
	streamMode     string
	activeToolLine int
	activeToolName string
	pending        []string
	imageCount     int
	history        []string
	historyPos     int
	historyDraft   string
	queuedMsgs     []string
}

const tuiFooterHeight int = 4
const maxCmdSuggestionRows int = 5

var bannerBox = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("141")).
	Padding(0, 1)

var tuiHint = lipgloss.NewStyle().
	Foreground(lipgloss.Color("240"))

var tuiUserPrefix = lipgloss.NewStyle().
	Foreground(lipgloss.Color("255")).
	Bold(true)

func tuiInputBoxStyle(colorCode string) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorCode)).
		Padding(0, 1)
}

func tuiHeaderBlock(agent, session, modeColor string, width int) string {
	var icon string
	var text []string
	var body string
	var box lipgloss.Style

	icon = fmt.Sprintf("\n%s", util.NaruLogoWithPad(""))

	text = []string{
		"",
		fmt.Sprintf("%smininaru%s %s%s (%s)%s", client.BOLD, client.RESET, client.DIM, util.AppVersion, util.AppHash, client.RESET),
		fmt.Sprintf("%s●%s %s %s%s%s", modeColor, client.RESET, agent, client.DIM, session, client.RESET),
		fmt.Sprintf("%s/help%s for commands", client.GRAY, client.RESET),
	}

	body = lipgloss.JoinVertical(lipgloss.Left, icon, strings.Join(text, "\n"))

	box = bannerBox
	if width > 0 {
		box = box.Width(width - box.GetHorizontalBorderSize())
	}

	return box.Render(body)
}

func newTuiModel(agent, session string) tuiModel {
	var ta textarea.Model

	ta = textarea.New()
	ta.Placeholder = "message (/ for commands)"
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.SetHeight(1)
	ta.Prompt = "> "
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ta.KeyMap.InsertNewline.SetEnabled(false)

	return tuiModel{
		input:          ta,
		agent:          agent,
		session:        session,
		mode:           core.ModeDefault,
		lines:          []string{tuiHeaderBlock(agent, session, client.ModeColor(core.ModeDefault), 0), ""},
		activeToolLine: -1,
		historyPos:     -1,
	}
}

func spinTickCmd() tea.Cmd {
	return tea.Tick(client.SpinnerTick, func(time.Time) tea.Msg {
		return tuiSpinMsg{}
	})
}

func newTuiSessionModel(agent, agentId, session, cwd, base, apiKey, mode string, noCache bool, conn *websocket.Conn, answers chan string, gateways []client.Gateway, restartPump func(*websocket.Conn)) tuiModel {
	var m tuiModel

	m = newTuiModel(agent, session)
	m.conn = conn
	m.cwd = cwd
	m.mode = mode
	m.lines[0] = tuiHeaderBlock(agent, session, client.ModeColor(mode), 0)
	m.noCache = noCache
	m.answers = answers
	m.base = base
	m.apiKey = apiKey
	m.agentId = agentId
	m.gateways = gateways
	m.restartPump = restartPump

	return m
}

func (m tuiModel) Init() tea.Cmd {
	return textarea.Blink
}
