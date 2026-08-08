package modules

import (
	"context"
	"sync"
	"testing"

	"github.com/devproje/mininaru/util"
)

func TestWebConfigSurvivesConcurrentReload(t *testing.T) {
	var group sync.WaitGroup
	var reader int
	var round int

	var err error

	util.RootDir = t.TempDir()

	err = WebLoad()
	if err != nil {
		t.Fatal(err)
	}

	for reader = range 4 {
		_ = reader

		group.Add(1)
		go func() {
			defer group.Done()

			var index int
			var cfg SearchConfig

			for index = range 200 {
				_ = index

				cfg = WebSearchConfig()
				if cfg.Provider == "" {
					t.Error("search provider went empty during reload")
					return
				}
			}
		}()
	}

	for round = range 50 {
		_ = round

		err = WebReload()
		if err != nil {
			t.Error(err)
			break
		}
	}

	group.Wait()
}

func TestMCPReloadDoesNotBlockToolListing(t *testing.T) {
	var group sync.WaitGroup
	var reader int
	var round int

	var err error

	util.RootDir = t.TempDir()

	err = MCPLoad()
	if err != nil {
		t.Fatal(err)
	}

	for reader = range 4 {
		_ = reader

		group.Add(1)
		go func() {
			defer group.Done()

			var index int

			for index = range 200 {
				_ = index

				DefaultTools()
				SafeTools()
				ToolSource("current_time")
				MCPStatusAll()
			}
		}()
	}

	for round = range 20 {
		_ = round

		err = MCPReload(context.Background())
		if err != nil {
			t.Error(err)
			break
		}
	}

	group.Wait()
}
