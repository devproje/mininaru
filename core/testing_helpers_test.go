// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"

	"github.com/devproje/mininaru/util"
)

func setupTestDB(t *testing.T) {
	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	util.DB, err = util.NewDatabase(util.Path("data.db"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		util.DB.Close()
	})
}
