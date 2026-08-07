package core

import (
	"os"
	"testing"

	"github.com/devproje/mininaru/util"
)

func botSetup(t *testing.T) {
	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	Providers = nil
	DefaultProvider = nil
	Agents = nil
	Global = nil
	Bots = nil

	ProviderCreate(Provider{Name: "p", BaseURL: "http://127.0.0.1", ApiKey: "k"})
	Global = AgentNew("naru", "", "", "m", Providers[0])
	Agents = []*NaruAgent{AgentNew("coder", "", "", "q", Providers[0])}
}

func TestBotCreateValidatesItsInputs(t *testing.T) {
	var created *Bot

	var err error

	botSetup(t)

	_, err = BotCreate(Bot{Kind: BotDiscord, Token: "t"})
	if err == nil {
		t.Fatal("nameless bot accepted")
	}

	_, err = BotCreate(Bot{Name: "b", Kind: "irc", Token: "t"})
	if err == nil {
		t.Fatal("unknown kind accepted")
	}

	_, err = BotCreate(Bot{Name: "b", Kind: BotDiscord})
	if err == nil {
		t.Fatal("tokenless bot accepted")
	}

	_, err = BotCreate(Bot{Name: "b", Kind: BotDiscord, Token: "t", Agent: "ghost"})
	if err == nil {
		t.Fatal("bot bound to an unknown agent accepted")
	}

	created, err = BotCreate(Bot{Name: "b", Kind: BotDiscord, Token: "t", Agent: "coder"})
	if err != nil {
		t.Fatal(err)
	}

	if created.Id == "" || !created.Enabled {
		t.Fatalf("created bot = %#v, want an id and enabled by default", created)
	}

	_, err = BotCreate(Bot{Name: "b", Kind: BotDiscord, Token: "t2"})
	if err == nil {
		t.Fatal("duplicate bot name accepted")
	}
}

func TestBotPersistsAcrossReload(t *testing.T) {
	var info os.FileInfo

	var err error

	botSetup(t)

	_, err = BotCreate(Bot{Name: "naru-bot", Kind: BotDiscord, Token: "secret-token", Agent: "naru"})
	if err != nil {
		t.Fatal(err)
	}

	Bots = nil

	err = BotInit()
	if err != nil {
		t.Fatal(err)
	}

	if len(Bots) != 1 || Bots[0].Token != "secret-token" || Bots[0].Agent != "naru" {
		t.Fatalf("reloaded bots = %#v", Bots)
	}

	info, err = os.Stat(util.Path(BOT_PATH))
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0600 {
		t.Fatalf("bot.json mode = %v, want 0600 since it holds a token", info.Mode().Perm())
	}
}

func TestBotInitCreatesAnEmptyFile(t *testing.T) {
	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	Bots = nil

	err = BotInit()
	if err != nil {
		t.Fatal(err)
	}

	if len(Bots) != 0 {
		t.Fatalf("fresh install has %d bots, want none", len(Bots))
	}
}

func TestBotUpdateDistinguishesOmittedFromCleared(t *testing.T) {
	var blank string
	var target *Bot
	var disabled bool

	var err error

	botSetup(t)

	_, err = BotCreate(Bot{Name: "b", Kind: BotDiscord, Token: "t", Agent: "coder", GuildId: "g1"})
	if err != nil {
		t.Fatal(err)
	}

	err = BotUpdateFields("b", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	target, err = BotFind("b")
	if err != nil {
		t.Fatal(err)
	}

	if target.Agent != "coder" || target.GuildId != "g1" || target.Token != "t" {
		t.Fatalf("omitted flags changed the bot: %#v", target)
	}

	blank = ""

	err = BotUpdateFields("b", nil, nil, &blank, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	target, _ = BotFind("b")
	if target.Agent != "" {
		t.Fatalf("agent = %q, want it cleared back to the global agent", target.Agent)
	}

	err = BotUpdateFields("b", nil, &blank, nil, nil, nil)
	if err == nil {
		t.Fatal("blanking the token was accepted")
	}

	disabled = false

	err = BotUpdateFields("b", nil, nil, nil, nil, &disabled)
	if err != nil {
		t.Fatal(err)
	}

	if len(BotsEnabled()) != 0 {
		t.Fatal("disabled bot still counted as enabled")
	}
}

func TestBotDeleteRemovesOnlyTheTarget(t *testing.T) {
	var err error

	botSetup(t)

	_, err = BotCreate(Bot{Name: "first", Kind: BotDiscord, Token: "t1"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = BotCreate(Bot{Name: "second", Kind: BotDiscord, Token: "t2"})
	if err != nil {
		t.Fatal(err)
	}

	err = BotDelete("first")
	if err != nil {
		t.Fatal(err)
	}

	if len(Bots) != 1 || Bots[0].Name != "second" {
		t.Fatalf("bots after delete = %#v", Bots)
	}

	err = BotDelete("ghost")
	if err == nil {
		t.Fatal("deleting an unknown bot succeeded")
	}
}
