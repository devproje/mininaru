package bot

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/core"
)

type DiscordConfig struct {
	Token   string
	Agent   string
	GuildId string
}

type Discord struct {
	cfg      DiscordConfig
	registry *core.Registry
	gateway  *discordgo.Session
}

const OriginDiscord = "discord"

var slashCommands []*discordgo.ApplicationCommand = []*discordgo.ApplicationCommand{
	{Name: "reset", Description: "start a fresh conversation in this channel"},
	{
		Name:        "agent",
		Description: "show or switch the agent answering in this channel",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "agent name", Required: false},
		},
	},
}

func NewDiscord(cfg DiscordConfig, registry *core.Registry) (*Discord, error) {
	var bot Discord

	var err error

	if cfg.Token == "" {
		return nil, fmt.Errorf("discord token is required")
	}

	if registry == nil {
		return nil, fmt.Errorf("registry is required")
	}

	bot = Discord{cfg: cfg, registry: registry}

	bot.gateway, err = discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, err
	}

	bot.gateway.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	return &bot, nil
}

func (d *Discord) instance(channelId string) (*core.Instance, error) {
	var bound *core.Session
	var target *core.Instance

	var err error

	bound, err = core.SessionByExternal(OriginDiscord, channelId)
	if err != nil {
		return nil, err
	}

	if bound != nil {
		target, err = d.registry.ByAgentId(bound.AgentId)
		if err == nil {
			return target, nil
		}
	}

	if d.cfg.Agent != "" {
		return d.registry.Get(d.cfg.Agent)
	}

	return d.registry.Default()
}

func (d *Discord) addressed(m *discordgo.MessageCreate) (string, bool) {
	var content string
	var mention *discordgo.User

	if m.Author == nil || m.Author.Bot {
		return "", false
	}

	content = strings.TrimSpace(m.Content)
	if content == "" {
		return "", false
	}

	if m.GuildID == "" {
		return content, true
	}

	for _, mention = range m.Mentions {
		if mention.ID != d.gateway.State.User.ID {
			continue
		}

		content = strings.ReplaceAll(content, "<@"+mention.ID+">", "")
		content = strings.ReplaceAll(content, "<@!"+mention.ID+">", "")

		return strings.TrimSpace(content), true
	}

	return "", false
}

func (d *Discord) answer(ctx context.Context, channelId, content string) {
	var target *core.Instance
	var session *core.Session
	var indicator *typing
	var message *core.Message

	var err error

	target, err = d.instance(channelId)
	if err != nil {
		d.gateway.ChannelMessageSend(channelId, "no agent available: "+err.Error())
		return
	}

	session, err = target.Bind(OriginDiscord, channelId, "discord "+channelId)
	if err != nil {
		d.gateway.ChannelMessageSend(channelId, "could not open a session: "+err.Error())
		return
	}

	indicator = startTyping(d.gateway, channelId)
	message, err = target.Chat(ctx, session, content, nil, nil, nil)
	indicator.stop()

	if err != nil {
		sendReply(d.gateway, channelId, "request failed: "+err.Error())
		return
	}

	sendReply(d.gateway, channelId, message.Content)
}

func (d *Discord) onMessage(gateway *discordgo.Session, m *discordgo.MessageCreate) {
	var content string
	var ok bool

	content, ok = d.addressed(m)
	if !ok {
		return
	}

	go d.answer(context.Background(), m.ChannelID, content)
}

func (d *Discord) respond(interaction *discordgo.InteractionCreate, text string) {
	d.gateway.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: text},
	})
}

func (d *Discord) resetCommand(interaction *discordgo.InteractionCreate) {
	var target *core.Instance
	var channelId string

	var err error

	channelId = interaction.ChannelID

	target, err = d.instance(channelId)
	if err != nil {
		d.respond(interaction, "no agent available: "+err.Error())
		return
	}

	_, err = core.SessionAttach(target.Agent, OriginDiscord, channelId, "discord "+channelId)
	if err != nil {
		d.respond(interaction, "reset failed: "+err.Error())
		return
	}

	d.respond(interaction, "started a fresh conversation with "+target.Agent.Name)
}

func (d *Discord) agentCommand(interaction *discordgo.InteractionCreate) {
	var options []*discordgo.ApplicationCommandInteractionDataOption
	var target *core.Instance
	var name string
	var channelId string

	var err error

	channelId = interaction.ChannelID
	options = interaction.ApplicationCommandData().Options

	if len(options) == 0 {
		target, err = d.instance(channelId)
		if err != nil {
			d.respond(interaction, "no agent available: "+err.Error())
			return
		}

		d.respond(interaction, "this channel talks to "+target.Agent.Name)
		return
	}

	name = options[0].StringValue()

	target, err = d.registry.Get(name)
	if err != nil {
		d.respond(interaction, err.Error())
		return
	}

	_, err = core.SessionAttach(target.Agent, OriginDiscord, channelId, "discord "+channelId)
	if err != nil {
		d.respond(interaction, "switch failed: "+err.Error())
		return
	}

	d.respond(interaction, "this channel now talks to "+name+", starting a fresh conversation")
}

func (d *Discord) onInteraction(gateway *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if interaction.Type != discordgo.InteractionApplicationCommand {
		return
	}

	switch interaction.ApplicationCommandData().Name {
	case "reset":
		d.resetCommand(interaction)
	case "agent":
		d.agentCommand(interaction)
	}
}

func (d *Discord) Start() error {
	var err error

	d.gateway.AddHandler(d.onMessage)
	d.gateway.AddHandler(d.onInteraction)

	err = d.gateway.Open()
	if err != nil {
		return err
	}

	_, err = d.gateway.ApplicationCommandBulkOverwrite(d.gateway.State.User.ID, d.cfg.GuildId, slashCommands)
	if err != nil {
		fmt.Fprintf(os.Stderr, "discord: registering slash commands failed: %v\n", err)
	}

	fmt.Printf("discord bot connected as %s\n", d.gateway.State.User.Username)

	return nil
}

func (d *Discord) Stop() error {
	return d.gateway.Close()
}
