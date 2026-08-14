// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/bot/discord/commands"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
)

func TestInteractionUserFallsBackToGuildMember(t *testing.T) {
	var interaction *discordgo.InteractionCreate
	var found *discordgo.User

	interaction = &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Member: &discordgo.Member{User: &discordgo.User{ID: "guild-user"}},
	}}
	found = interactionUser(interaction)
	if found == nil || found.ID != "guild-user" {
		t.Fatalf("interaction user = %#v, want guild member user", found)
	}
}

func TestInteractionUserPrefersDirectUser(t *testing.T) {
	var interaction *discordgo.InteractionCreate
	var found *discordgo.User

	interaction = &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		User:   &discordgo.User{ID: "direct-user"},
		Member: &discordgo.Member{User: &discordgo.User{ID: "guild-user"}},
	}}
	found = interactionUser(interaction)
	if found == nil || found.ID != "direct-user" {
		t.Fatalf("interaction user = %#v, want direct user", found)
	}
}

func TestContextPromptsOnlySearchWithWebTool(t *testing.T) {
	var prompt string
	var defs []modules.Def

	prompt, defs = contextPrompt("Message Analyzer")
	if prompt == "" || len(defs) != 0 {
		t.Fatalf("analyzer prompt=%q tools=%d", prompt, len(defs))
	}
	prompt, defs = contextPrompt("Content Search")
	if prompt == "" || len(defs) != 1 || defs[0].Name != "web_search" {
		t.Fatalf("search prompt=%q tools=%v", prompt, defs)
	}
}

func TestCommandSetsKeepUserAppCommandsGlobal(t *testing.T) {
	var guild []*discordgo.ApplicationCommand
	var global []*discordgo.ApplicationCommand
	var command *discordgo.ApplicationCommand
	var foundAnalyzer bool
	var foundChat bool

	guild, global = commands.Sets("guild-1")
	if len(guild) == 0 {
		t.Fatal("guild commands are empty")
	}
	for _, command = range global {
		foundAnalyzer = foundAnalyzer || command.Name == "Message Analyzer"
		foundChat = foundChat || command.Name == "chat"
	}
	if !foundAnalyzer || !foundChat {
		t.Fatalf("global commands missing analyzer=%v chat=%v", foundAnalyzer, foundChat)
	}
}

func TestAgentCommandEnablesAutocomplete(t *testing.T) {
	var global []*discordgo.ApplicationCommand
	var command *discordgo.ApplicationCommand

	_, global = commands.Sets("")
	for _, command = range global {
		if command.Name != "agent" {
			continue
		}
		if len(command.Options) != 1 || !command.Options[0].Autocomplete {
			t.Fatalf("agent command option = %#v, want autocomplete", command.Options)
		}
		return
	}
	t.Fatal("agent command missing")
}

func TestAgentChoicesFilterAndStayWithinDiscordLimit(t *testing.T) {
	var index int
	var instances []*core.Instance
	var choices []*discordgo.ApplicationCommandOptionChoice

	for index = 0; index < 30; index++ {
		instances = append(instances, &core.Instance{Agent: &core.NaruAgent{Name: fmt.Sprintf("Agent-%02d", index)}})
	}
	choices = agentChoices(instances, "agent-")
	if len(choices) != autocompleteLimit {
		t.Fatalf("choices = %d, want %d", len(choices), autocompleteLimit)
	}
	choices = agentChoices(instances, "-07")
	if len(choices) != 1 || choices[0].Value != "Agent-07" {
		t.Fatalf("filtered choices = %#v", choices)
	}
	instances = append([]*core.Instance{{Agent: &core.NaruAgent{Name: strings.Repeat("x", autocompleteNameLimit+1)}}}, instances...)
	choices = agentChoices(instances, strings.Repeat("x", autocompleteNameLimit))
	if len(choices) != 0 {
		t.Fatalf("overlong agent was offered: %#v", choices)
	}
}

func TestResetConfirmationBindsControlsToRequester(t *testing.T) {
	var encoded []byte
	var text string

	var err error

	encoded, err = json.Marshal(resetConfirmationComponents("naru", "user-1"))
	if err != nil {
		t.Fatal(err)
	}
	text = string(encoded)
	if !strings.Contains(text, "cannot be undone") || !strings.Contains(text, "reset:yes:user-1") ||
		!strings.Contains(text, "reset:no:user-1") {
		t.Fatalf("reset confirmation = %s", text)
	}
}

func TestThreadNameUsesTheFirstRequestAndStaysWithinDiscordLimit(t *testing.T) {
	var name string

	name = threadName("  # Deploy\n   error\x00 report  ", false)
	if name != "mininaru · Deploy error report" {
		t.Fatalf("thread name = %q", name)
	}
	name = threadName("", true)
	if name != "mininaru · attachment" {
		t.Fatalf("attachment thread name = %q", name)
	}
	name = threadName(strings.Repeat("가", threadNameLimit+20), false)
	if len([]rune(name)) != threadNameLimit || !strings.HasSuffix(name, "…") {
		t.Fatalf("bounded thread name has %d runes: %q", len([]rune(name)), name)
	}
}

func TestRememberMessageRejectsDuplicatesAndEvictsOldEntries(t *testing.T) {
	var bot Discord
	var index int

	if !bot.rememberMessage("first") || bot.rememberMessage("first") {
		t.Fatal("duplicate message was not rejected")
	}
	for index = 0; index < seenMessageLimit; index++ {
		if !bot.rememberMessage(fmt.Sprintf("message-%d", index)) {
			t.Fatalf("new message %d was rejected", index)
		}
	}
	if !bot.rememberMessage("first") {
		t.Fatal("oldest message was not evicted from the bounded cache")
	}
}

func TestQueueTurnPreservesArrivalOrder(t *testing.T) {
	var bot Discord
	var release chan struct{}
	var completed chan struct{}
	var wait sync.WaitGroup
	var order []int

	bot.lifetime = context.Background()
	release = make(chan struct{})
	completed = make(chan struct{})
	wait.Add(3)
	bot.queueTurn("channel", func() {
		<-release
		order = append(order, 1)
		wait.Done()
	})
	bot.queueTurn("channel", func() {
		order = append(order, 2)
		wait.Done()
	})
	bot.queueTurn("channel", func() {
		order = append(order, 3)
		wait.Done()
	})
	close(release)
	go func() {
		wait.Wait()
		close(completed)
	}()
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("queued turns did not complete")
	}
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("turn order = %v", order)
	}
}

func TestThreadNoticesExplainContextAndFallback(t *testing.T) {
	var encoded []byte
	var text string

	var err error

	encoded, err = json.Marshal(threadWelcomeComponents("naru"))
	if err != nil {
		t.Fatal(err)
	}
	text = string(encoded)
	if !strings.Contains(text, "own context") || !strings.Contains(text, "`/reset`") {
		t.Fatalf("thread welcome = %s", text)
	}
	encoded, err = json.Marshal(threadFallbackComponents())
	if err != nil {
		t.Fatal(err)
	}
	text = string(encoded)
	if !strings.Contains(text, "Could not start") || !strings.Contains(text, "create public threads") {
		t.Fatalf("thread fallback = %s", text)
	}
}

func TestUserAppPresentationExplainsVisibilityAndPersistence(t *testing.T) {
	var view userAppPresentation
	var encoded []byte
	var text string

	var err error

	view = userAppView("chat", true)
	encoded, err = json.Marshal(userAppComponents(view, userAppDone, "answer"))
	if err != nil {
		t.Fatal(err)
	}
	text = string(encoded)
	if !strings.Contains(text, "Private one-time result") || !strings.Contains(text, "Not saved") {
		t.Fatalf("private app result = %s", text)
	}
	view = userAppView("chat", false)
	if !strings.Contains(view.note, "Visible in this channel") {
		t.Fatalf("public app note = %q", view.note)
	}
}

func TestUserAppPresentationMakesFailuresAndTruncationExplicit(t *testing.T) {
	var view userAppPresentation
	var encoded []byte
	var text string

	var err error

	view = userAppView("Content Search", true)
	encoded, err = json.Marshal(userAppComponents(view, userAppFailed, strings.Repeat("x", userAppContentLimit+100)))
	if err != nil {
		t.Fatal(err)
	}
	text = string(encoded)
	if !strings.Contains(text, "❌") || !strings.Contains(text, "Result shortened") || !strings.Contains(text, "Uses the public web") {
		t.Fatalf("failed app result = %s", text)
	}
}

func TestBotWithoutStoredIdDoesNotGrantAdminRole(t *testing.T) {
	var bot Discord
	var role string

	var err error

	bot = Discord{cfg: Config{BotId: ""}}
	role, err = bot.role("any-user")
	if err != nil {
		t.Fatal(err)
	}
	if role != "" {
		t.Fatalf("bot without stored id role = %q, want no access", role)
	}
}

func TestStopCancelsInFlightTurns(t *testing.T) {
	var bot *Discord
	var ctx context.Context
	var cancel context.CancelFunc

	var err error

	bot, err = New(Config{Token: "test-token", BotId: "bot-1"}, core.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel = bot.turnContext()
	defer cancel()

	if ctx.Err() != nil {
		t.Fatalf("fresh turn context already done: %v", ctx.Err())
	}

	bot.shutdown()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel the in-flight turn context")
	}
}

func TestTurnContextCarriesADeadline(t *testing.T) {
	var bot *Discord
	var ctx context.Context
	var cancel context.CancelFunc
	var deadline time.Time
	var ok bool

	var err error

	bot, err = New(Config{Token: "test-token", BotId: "bot-1"}, core.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	defer bot.shutdown()

	ctx, cancel = bot.turnContext()
	defer cancel()

	deadline, ok = ctx.Deadline()
	if !ok {
		t.Fatal("turn context has no deadline")
	}
	if time.Until(deadline) > replyTimeout+time.Second {
		t.Fatalf("deadline is %v away, want at most %v", time.Until(deadline), replyTimeout)
	}
}
