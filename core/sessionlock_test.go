// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionLockSerializesConcurrentHolders(t *testing.T) {
	var wg sync.WaitGroup
	var active int32
	var maxActive int32
	var i int

	for i = 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			var unlock func()
			var current int32

			defer wg.Done()

			unlock = SessionLock("s1")
			defer unlock()

			current = atomic.AddInt32(&active, 1)
			if current > atomic.LoadInt32(&maxActive) {
				atomic.StoreInt32(&maxActive, current)
			}

			time.Sleep(time.Millisecond)

			atomic.AddInt32(&active, -1)
		}()
	}

	wg.Wait()

	if maxActive != 1 {
		t.Fatalf("max concurrent SessionLock holders = %d, want 1", maxActive)
	}
}

func TestSessionLockIsIndependentPerSession(t *testing.T) {
	var unlockA func()
	var unlockB func()
	var done chan struct{}

	unlockA = SessionLock("a")
	defer unlockA()

	done = make(chan struct{})
	go func() {
		unlockB = SessionLock("b")
		defer unlockB()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SessionLock on a different session id blocked on an unrelated lock")
	}
}
