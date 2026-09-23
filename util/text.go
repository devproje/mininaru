// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package util

func TruncateRunes(s string, max int) string {
	var runes []rune

	runes = []rune(s)
	if len(runes) <= max {
		return s
	}

	return string(runes[:max])
}
