// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/util"
	"github.com/openai/openai-go"
)

type Summary struct {
	SessionId        string `json:"session_id"`
	Content          string `json:"content"`
	ThroughMessageId string `json:"through_message_id"`
}

const maxSummaryChars = 2048

const summaryInstruction = `Compress the conversation below into one running summary that replaces the
summary so far rather than being appended to it.

Keep the decisions that were made, the constraints and preferences the user
stated, the facts established about their situation, and anything still
unfinished. Drop pleasantries, restatements, and detail that only mattered while
a step was in progress. Write plain prose in the third person, for whoever picks
this conversation up next, and answer with the summary alone.`

func SummaryLoad(sessionId string) (*Summary, error) {
	var summary Summary

	var err error

	err = util.DB.QueryRow("SELECT session_id, content, through_message_id FROM session_summaries WHERE session_id = ?;", sessionId).
		Scan(&summary.SessionId, &summary.Content, &summary.ThroughMessageId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return &summary, nil
}

func SummarySave(sessionId, content, throughMessageId string) error {
	var err error

	_, err = util.DB.Exec(`INSERT INTO session_summaries (session_id, content, through_message_id) VALUES (?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET content = excluded.content, through_message_id = excluded.through_message_id;`,
		sessionId, content, throughMessageId)

	return err
}

func summaryTail(history []*Message, throughMessageId string) []*Message {
	var index int

	for index = range history {
		if history[index].Id != throughMessageId {
			continue
		}

		return history[index+1:]
	}

	util.Log.Warn("the last summarized message is gone from the history, replaying every turn",
		"message", throughMessageId)

	return history
}

func summaryTranscript(previous string, dropped []*Message) string {
	var builder strings.Builder
	var index int

	builder.WriteString(summaryInstruction)
	fmt.Fprintf(&builder, "\n\nStay under %d characters.\n\n", maxSummaryChars)

	if previous != "" {
		builder.WriteString("<summary-so-far>\n")
		builder.WriteString(previous)
		builder.WriteString("\n</summary-so-far>\n\n")
	}

	builder.WriteString("<turns-to-fold-in>\n")

	for index = range dropped {
		builder.WriteString(dropped[index].Role)
		builder.WriteString(": ")
		builder.WriteString(dropped[index].Content)
		builder.WriteString("\n")
	}

	builder.WriteString("</turns-to-fold-in>")

	return builder.String()
}

func summarize(ctx context.Context, agent *NaruAgent, previous string, dropped []*Message) (string, openai.CompletionUsage, error) {
	var messages []openai.ChatCompletionMessageParamUnion
	var result *Completion
	var text string
	var runes []rune

	var err error

	messages = append(messages, openai.UserMessage(summaryTranscript(previous, dropped)))

	result, err = Complete(ctx, agent, messages, nil, config.ThinkingOff, nil, nil)
	if err != nil {
		return "", openai.CompletionUsage{}, err
	}

	text = strings.TrimSpace(result.Content)
	if text == "" {
		return "", result.Usage, fmt.Errorf("the model returned an empty summary")
	}

	runes = []rune(text)
	if len(runes) > maxSummaryChars {
		text = string(runes[:maxSummaryChars])
	}

	return text, result.Usage, nil
}

func CompactNow(ctx context.Context, agent *NaruAgent, session *Session) (bool, error) {
	var history []*Message
	var previous *Summary
	var text string
	var tail []*Message
	var updated string
	var usage openai.CompletionUsage

	var err error

	if agent == nil || session == nil {
		return false, fmt.Errorf("agent and session are required to compact")
	}

	history, err = MessageList(session.Id)
	if err != nil {
		return false, err
	}

	previous, err = SummaryLoad(session.Id)
	if err != nil {
		return false, err
	}

	tail = history
	if previous != nil {
		text = previous.Content
		tail = summaryTail(history, previous.ThroughMessageId)
	}

	if len(tail) == 0 {
		return false, nil
	}

	updated, usage, err = summarize(ctx, agent, text, tail)
	usageRecord(session.Id, "", UsageCompaction, usage)

	if err != nil {
		return false, err
	}

	err = SummarySave(session.Id, updated, tail[len(tail)-1].Id)
	if err != nil {
		return false, err
	}

	util.Log.Debug("compacted the conversation on request",
		"session", session.Id, "turns", len(tail), "summary_chars", len(updated))

	return true, nil
}

func SessionContextChars(sessionId string) (int, error) {
	var summary *Summary
	var history []*Message
	var tail []*Message
	var kept []*Message
	var calls map[string][]*ToolCall
	var message *Message
	var used int

	var err error

	history, err = MessageList(sessionId)
	if err != nil {
		return 0, err
	}
	summary, err = SummaryLoad(sessionId)
	if err != nil {
		return 0, err
	}
	calls, err = toolCallsBySession(sessionId)
	if err != nil {
		return 0, err
	}
	tail = history
	if summary != nil {
		used = len(summary.Content)
		tail = summaryTail(history, summary.ThroughMessageId)
	}
	kept = trimHistory(tail, calls, config.Client.Context.MaxChars, used)
	for _, message = range kept {
		used += len(message.Content) + toolCost(calls[message.Id])
	}

	return used, nil
}

func compactHistory(ctx context.Context, agent *NaruAgent, session *Session, history []*Message,
	calls map[string][]*ToolCall, reserved int) (string, []*Message) {
	var previous *Summary
	var text string
	var tail []*Message
	var kept []*Message
	var dropped []*Message
	var updated string
	var usage openai.CompletionUsage

	var err error

	previous, err = SummaryLoad(session.Id)
	if err != nil {
		util.Log.Warn("loading the conversation summary failed", "session", session.Id, "error", err)
	}

	tail = history
	if previous != nil {
		text = previous.Content
		tail = summaryTail(history, previous.ThroughMessageId)
	}

	kept = trimHistory(tail, calls, config.Client.Context.MaxChars, reserved+len(text))
	if len(kept) == len(tail) || !config.Client.Context.Compact {
		return text, kept
	}

	dropped = tail[:len(tail)-len(kept)]

	updated, usage, err = summarize(ctx, agent, text, dropped)
	usageRecord(session.Id, "", UsageCompaction, usage)

	if err != nil {
		util.Log.Warn("compacting the conversation failed, dropping its oldest turns instead",
			"session", session.Id, "turns", len(dropped), "error", err)

		return text, kept
	}

	err = SummarySave(session.Id, updated, dropped[len(dropped)-1].Id)
	if err != nil {
		util.Log.Warn("saving the conversation summary failed, dropping its oldest turns instead",
			"session", session.Id, "error", err)

		return text, kept
	}

	util.Log.Debug("compacted the conversation",
		"session", session.Id, "turns", len(dropped), "summary_chars", len(updated))

	return updated, kept
}
