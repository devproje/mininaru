// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type typing struct {
	gateway   *discordgo.Session
	channelId string

	done chan struct{}
	wait sync.WaitGroup
}

const messageLimit = 2000

const replyChunkReserve = 64

const typingInterval = 8 * time.Second

const emptyReply = "(empty response)"

func splitMessage(text string, limit int) []string {
	var runes []rune
	var cut int
	var head string
	var newline int
	var chunks []string

	runes = []rune(text)

	for len(runes) > limit {
		cut = limit
		head = string(runes[:cut])

		newline = strings.LastIndex(head, "\n")
		if newline > 0 && len([]rune(head[:newline])) > limit/2 {
			cut = len([]rune(head[:newline]))

			chunks = append(chunks, string(runes[:cut]))
			runes = runes[cut+1:]

			continue
		}

		chunks = append(chunks, string(runes[:cut]))
		runes = runes[cut:]
	}

	if len(runes) > 0 {
		chunks = append(chunks, string(runes))
	}

	if len(chunks) == 0 {
		chunks = append(chunks, "")
	}

	return chunks
}

func splitReply(text string, limit int) []string {
	var raw []string
	var chunks []string
	var chunk string
	var fence string
	var prefix string
	var index int

	if limit <= replyChunkReserve {
		return splitMessage(text, limit)
	}
	raw = splitMessage(text, limit-replyChunkReserve)
	for _, chunk = range raw {
		prefix = ""
		if fence != "" {
			prefix = fence + "\n"
		}
		fence = replyFenceAfter(fence, chunk)
		chunk = prefix + chunk
		if fence != "" {
			chunk += "\n```"
		}
		chunks = append(chunks, chunk)
	}
	if len(chunks) == 1 {
		return chunks
	}
	for index = range chunks {
		chunks[index] += fmt.Sprintf("\n\n-# Part %d/%d", index+1, len(chunks))
	}
	return chunks
}

func replyFenceAfter(open, text string) string {
	var lines []string
	var line string
	var marker string

	lines = strings.Split(text, "\n")
	for _, line = range lines {
		marker = strings.TrimSpace(line)
		if !strings.HasPrefix(marker, "```") {
			continue
		}
		if open != "" {
			open = ""
			continue
		}
		open = marker
		if len([]rune(open)) > 32 {
			open = "```"
		}
	}
	return open
}

func startTyping(gateway *discordgo.Session, channelId string) *typing {
	var indicator typing

	indicator = typing{gateway: gateway, channelId: channelId, done: make(chan struct{})}

	indicator.gateway.ChannelTyping(indicator.channelId)
	indicator.wait.Add(1)

	go func() {
		var ticker *time.Ticker

		defer indicator.wait.Done()

		ticker = time.NewTicker(typingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				indicator.gateway.ChannelTyping(indicator.channelId)
			case <-indicator.done:
				return
			}
		}
	}()

	return &indicator
}

func (t *typing) stop() {
	close(t.done)
	t.wait.Wait()
}

func (d *Discord) sendReply(channelId, text string) {
	var chunks []string

	if strings.TrimSpace(text) == "" {
		text = emptyReply
	}

	chunks = splitReply(text, messageLimit)
	d.sendChunks(channelId, chunks)
}

func (d *Discord) sendChunks(channelId string, chunks []string) {
	var chunk string

	for _, chunk = range chunks {
		d.gateway.ChannelMessageSendComplex(channelId, &discordgo.MessageSend{
			Content:         chunk,
			AllowedMentions: d.allowedMentions(chunk),
		})
	}
}
