// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"sync"
)

var sessionLocks sync.Map

func SessionLock(sessionId string) func() {
	var stored any
	var mu *sync.Mutex

	stored, _ = sessionLocks.LoadOrStore(sessionId, &sync.Mutex{})
	mu = stored.(*sync.Mutex)

	mu.Lock()

	return mu.Unlock
}
