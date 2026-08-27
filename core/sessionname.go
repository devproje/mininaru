// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import "math/rand/v2"

var sessionAdjectives = []string{
	"quiet", "brisk", "amber", "calm", "bold", "soft", "keen", "spry",
	"still", "swift", "mellow", "crisp", "gentle", "lively", "tidy", "vivid",
	"steady", "warm", "cool", "bright", "quick", "plain", "sturdy", "wry",
}

var sessionNouns = []string{
	"otter", "finch", "cedar", "harbor", "meadow", "brook", "falcon", "maple",
	"pebble", "willow", "heron", "canyon", "juniper", "swallow", "birch", "delta",
	"lantern", "thistle", "marten", "orchard", "compass", "ember", "linden", "sparrow",
}

func randomSessionName() string {
	return sessionAdjectives[rand.IntN(len(sessionAdjectives))] + "-" + sessionNouns[rand.IntN(len(sessionNouns))]
}
