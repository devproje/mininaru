package bot

import (
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

func sendReply(gateway *discordgo.Session, channelId, text string) {
	var chunks []string
	var chunk string

	if strings.TrimSpace(text) == "" {
		text = emptyReply
	}

	chunks = splitMessage(text, messageLimit)

	for _, chunk = range chunks {
		gateway.ChannelMessageSend(channelId, chunk)
	}
}
