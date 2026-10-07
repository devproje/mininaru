// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/devproje/mininaru/util"
)

func emptyHelper(ctx context.Context, id, tbl, param string) error {
	var tx *sql.Tx
	var query string

	var err error

	tx, err = util.DB.Begin()
	if err != nil {
		return err
	}

	query = fmt.Sprintf("UPDATE %s SET %s = '' WHERE id = ?;", tbl, param)
	_, err = tx.ExecContext(ctx, query, id)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	return nil
}
