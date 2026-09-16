// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"errors"
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
			var err error

			defer wg.Done()

			unlock, err = SessionLock(t.Context(), "s1")
			if err != nil {
				t.Errorf("SessionLock failed: %v", err)
				return
			}
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
	var err error

	unlockA, err = SessionLock(t.Context(), "a")
	if err != nil {
		t.Fatal(err)
	}
	defer unlockA()

	done = make(chan struct{})
	go func() {
		var lockErr error

		unlockB, lockErr = SessionLock(t.Context(), "b")
		if lockErr != nil {
			t.Errorf("SessionLock failed: %v", lockErr)
			close(done)
			return
		}
		defer unlockB()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SessionLock on a different session id blocked on an unrelated lock")
	}
}

func TestSessionLockStopsWaitingWhenContextIsCanceled(t *testing.T) {
	var ctx context.Context
	var cancel context.CancelFunc
	var unlock func()
	var result chan error

	var err error

	unlock, err = SessionLock(t.Context(), "cancel-wait")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	ctx, cancel = context.WithCancel(t.Context())
	result = make(chan error, 1)
	go func() {
		var lockErr error

		_, lockErr = SessionLock(ctx, "cancel-wait")
		result <- lockErr
	}()

	cancel()
	select {
	case err = <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("SessionLock error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SessionLock did not stop waiting after context cancellation")
	}
}

func TestCanceledSessionLockWaiterDoesNotConsumeTheLock(t *testing.T) {
	var ctx context.Context
	var cancel context.CancelFunc
	var unlock func()
	var nextUnlock func()
	var result chan error
	var locked bool

	var err error

	unlock, err = SessionLock(t.Context(), "canceled-waiter")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel = context.WithCancel(t.Context())
	result = make(chan error, 1)
	go func() {
		var lockErr error

		_, lockErr = SessionLock(ctx, "canceled-waiter")
		result <- lockErr
	}()
	cancel()

	err = <-result
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SessionLock error = %v, want context.Canceled", err)
	}

	unlock()
	nextUnlock, locked = SessionTryLock("canceled-waiter")
	if !locked {
		t.Fatal("canceled waiter consumed the session lock")
	}
	nextUnlock()
}

func TestSessionTryLockRefusesABusySession(t *testing.T) {
	var unlock func()
	var secondUnlock func()
	var locked bool

	unlock, locked = SessionTryLock("busy")
	if !locked {
		t.Fatal("first SessionTryLock failed")
	}
	defer unlock()

	secondUnlock, locked = SessionTryLock("busy")
	if locked || secondUnlock != nil {
		t.Fatal("second SessionTryLock acquired a busy session")
	}
}
