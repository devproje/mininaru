// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestLogInitRejectsUnknownLevelAndFormat(t *testing.T) {
	var err error

	err = LogInit(LogOptions{Level: "chatty", Output: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected an error for an unknown level")
	}

	err = LogInit(LogOptions{Format: "yaml", Output: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected an error for an unknown format")
	}
}

func TestLogInitHonorsLevel(t *testing.T) {
	var out bytes.Buffer

	var err error

	err = LogInit(LogOptions{Level: "warn", Format: LogFormatText, Output: &out})
	if err != nil {
		t.Fatal(err)
	}

	Log.Info("quiet")
	Log.Warn("loud")

	if strings.Contains(out.String(), "quiet") {
		t.Fatalf("info record survived the warn level: %s", out.String())
	}
	if !strings.Contains(out.String(), "loud") {
		t.Fatalf("warn record was dropped: %s", out.String())
	}
}

func TestLogInitDefaultsToJSONOffTerminal(t *testing.T) {
	var out bytes.Buffer
	var record map[string]any

	var err error

	err = LogInit(LogOptions{Output: &out})
	if err != nil {
		t.Fatal(err)
	}

	Log.Info("hello", "agent", "naru")

	err = json.Unmarshal(out.Bytes(), &record)
	if err != nil {
		t.Fatalf("output was not json: %s", out.String())
	}
	if record["msg"] != "hello" || record["agent"] != "naru" {
		t.Fatalf("record = %#v", record)
	}
}

func TestLogHoldDefersOutputUntilRelease(t *testing.T) {
	var out bytes.Buffer
	var release func()

	var err error

	err = LogInit(LogOptions{Format: LogFormatText, Output: &out})
	if err != nil {
		t.Fatal(err)
	}

	release = LogHold()

	Log.Info("during the tui")

	if out.Len() != 0 {
		t.Fatalf("log leaked into the terminal while held: %s", out.String())
	}

	release()

	if !strings.Contains(out.String(), "during the tui") {
		t.Fatalf("held record was lost: %s", out.String())
	}
}

func TestLogRejectsReservedAttributeKeys(t *testing.T) {
	var out bytes.Buffer
	var decoder *json.Decoder
	var record map[string]any
	var raw string
	var keys int

	var err error

	err = LogInit(LogOptions{Format: LogFormatJSON, Output: &out})
	if err != nil {
		t.Fatal(err)
	}

	Log.Warn("collides", "level", "bogus")

	raw = strings.TrimSpace(out.String())

	decoder = json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()

	err = json.Unmarshal([]byte(raw), &record)
	if err != nil {
		t.Fatal(err)
	}

	keys = strings.Count(raw, `"level":`)
	if keys > 1 {
		t.Fatalf("a reserved key was duplicated %d times, which makes the record ambiguous: %s", keys, raw)
	}
}

func TestLogWritesAreConcurrencySafe(t *testing.T) {
	var out bytes.Buffer
	var group sync.WaitGroup
	var writer int
	var release func()

	var err error

	err = LogInit(LogOptions{Format: LogFormatText, Output: &out})
	if err != nil {
		t.Fatal(err)
	}

	for writer = range 4 {
		_ = writer

		group.Add(1)
		go func() {
			defer group.Done()

			var index int

			for index = range 100 {
				_ = index

				Log.Info("concurrent record", "index", index)
			}
		}()
	}

	for writer = range 20 {
		_ = writer

		release = LogHold()
		release()
	}

	group.Wait()

	if !strings.Contains(out.String(), "concurrent record") {
		t.Fatal("no records reached the output")
	}
}
