package core

import (
	"os"
	"testing"

	"github.com/devproje/mininaru/util"
)

func providerSetup(t *testing.T) {
	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	Providers = nil
	DefaultProvider = nil
	Global = nil
	Agents = nil
}

func TestDefaultProviderSurvivesReload(t *testing.T) {
	var err error

	providerSetup(t)

	ProviderCreate(Provider{Name: "alpha", BaseURL: "http://a"})
	ProviderCreate(Provider{Name: "beta", BaseURL: "http://b"})

	err = ProviderSave()
	if err != nil {
		t.Fatal(err)
	}

	err = ProviderDefault("beta")
	if err != nil {
		t.Fatal(err)
	}

	err = ProviderInit()
	if err != nil {
		t.Fatal(err)
	}

	if DefaultProvider == nil || DefaultProvider.Name != "beta" {
		t.Fatalf("default provider after reload = %v, want beta", DefaultProvider)
	}
}

func TestFirstProviderBecomesDefault(t *testing.T) {
	providerSetup(t)

	ProviderCreate(Provider{Name: "alpha", BaseURL: "http://a"})
	ProviderCreate(Provider{Name: "beta", BaseURL: "http://b"})

	if DefaultProvider == nil || DefaultProvider.Name != "alpha" {
		t.Fatalf("default = %v, want the first provider to claim it", DefaultProvider)
	}
}

func TestLegacyArrayFileStillLoads(t *testing.T) {
	var legacy string

	var err error

	providerSetup(t)

	legacy = `[{"id":"p1","name":"alpha","api_key":"k","base_url":"http://a"}]`

	err = os.WriteFile(util.Path(PROVIDER_PATH), []byte(legacy), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = ProviderInit()
	if err != nil {
		t.Fatalf("legacy array provider.json failed to load: %v", err)
	}

	if len(Providers) != 1 || Providers[0].Name != "alpha" {
		t.Fatalf("providers = %v, want the legacy entry", Providers)
	}

	if DefaultProvider == nil || DefaultProvider.Name != "alpha" {
		t.Fatalf("default = %v, want fallback to the only provider", DefaultProvider)
	}
}

func TestDeletingDefaultReassignsIt(t *testing.T) {
	var err error

	providerSetup(t)

	ProviderCreate(Provider{Name: "alpha", BaseURL: "http://a"})
	ProviderCreate(Provider{Name: "beta", BaseURL: "http://b"})

	err = ProviderDefault("alpha")
	if err != nil {
		t.Fatal(err)
	}

	err = ProviderDelete(DefaultProvider.Id)
	if err != nil {
		t.Fatal(err)
	}

	if DefaultProvider == nil || DefaultProvider.Name != "beta" {
		t.Fatalf("default after deleting it = %v, want beta to take over", DefaultProvider)
	}
}

func TestUpdatingDefaultKeepsItCurrent(t *testing.T) {
	var err error

	providerSetup(t)

	ProviderCreate(Provider{Name: "alpha", BaseURL: "http://a"})

	err = ProviderDefault("alpha")
	if err != nil {
		t.Fatal(err)
	}

	err = ProviderUpdate(DefaultProvider.Id, Provider{BaseURL: "http://changed"})
	if err != nil {
		t.Fatal(err)
	}

	if DefaultProvider.BaseURL != "http://changed" {
		t.Fatalf("default still points at the pre-update struct: %q", DefaultProvider.BaseURL)
	}

	if DefaultProvider != Providers[0] {
		t.Fatal("default provider dangles outside the Providers slice")
	}
}

func TestDeletingProviderUsedByAgentIsRejected(t *testing.T) {
	var id string

	var err error

	providerSetup(t)
	ProviderCreate(Provider{Name: "alpha", BaseURL: "http://a"})
	id = Providers[0].Id
	Global = AgentNew("global", "", "", "m", Providers[0])

	err = ProviderDelete(id)
	if err == nil {
		t.Fatal("deleting a provider used by the global agent unexpectedly succeeded")
	}
	if len(Providers) != 1 || Providers[0].Id != id {
		t.Fatal("rejected provider deletion still mutated the provider list")
	}
}

func TestProviderUpdateFieldsCanClearValues(t *testing.T) {
	var empty string

	var err error

	providerSetup(t)
	ProviderCreate(Provider{Name: "alpha", BaseURL: "http://a", ApiKey: "secret"})

	err = ProviderUpdateFields(Providers[0].Id, nil, &empty, &empty)
	if err != nil {
		t.Fatal(err)
	}
	if Providers[0].ApiKey != "" || Providers[0].BaseURL != "" {
		t.Fatalf("provider values were not cleared: %#v", Providers[0])
	}
}
