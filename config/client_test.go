package config

import (
	"os"
	"testing"

	"github.com/devproje/mininaru/util"
)

func TestLegacyClientConfigGetsDefaultContextBudget(t *testing.T) {
	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(util.Path(CLIENT_PATH), []byte(`{"thinking":{"level":"off","show":true}}`), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = ClientInit()
	if err != nil {
		t.Fatal(err)
	}
	if Client.Context.MaxChars != 32768 {
		t.Fatalf("context max chars = %d, want default 32768", Client.Context.MaxChars)
	}
	if !Client.Tools.Enabled {
		t.Fatal("tools should default to enabled for legacy client config")
	}
}
