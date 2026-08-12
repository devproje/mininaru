// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import "github.com/devproje/mininaru/util"

func publicFailure(operation string, err error) string {
	util.Log.Error("discord operation failed", "operation", operation, "error", err)
	return "I couldn't finish while " + operation + ". Please try again."
}

func conversationFailure(operation string, err error) string {
	return publicFailure(operation, err) + " If it keeps failing, use `/reset` to start a fresh conversation."
}

func accessDenied(botPaired bool) string {
	if !botPaired {
		return "This bot has not been paired with an admin yet. Ask the bot owner to run `mininaru bot pair`, then use `/pair`."
	}
	return "You don't have access to this bot yet. Ask the bot admin to add you with `/user add`."
}
