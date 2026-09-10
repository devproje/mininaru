// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"sync"
)

type sessionLock chan struct{}

var sessionLocks sync.Map

func loadSessionLock(sessionId string) sessionLock {
	var stored any
	var lock sessionLock

	stored, _ = sessionLocks.LoadOrStore(sessionId, make(sessionLock, 1))
	lock = stored.(sessionLock)

	return lock
}

func SessionLock(ctx context.Context, sessionId string) (func(), error) {
	var lock sessionLock

	var err error

	err = ctx.Err()
	if err != nil {
		return nil, err
	}

	lock = loadSessionLock(sessionId)
	select {
	case lock <- struct{}{}:
		err = ctx.Err()
		if err != nil {
			<-lock
			return nil, err
		}
		return func() { <-lock }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func SessionTryLock(sessionId string) (func(), bool) {
	var lock sessionLock

	lock = loadSessionLock(sessionId)
	select {
	case lock <- struct{}{}:
		return func() { <-lock }, true
	default:
		return nil, false
	}
}
