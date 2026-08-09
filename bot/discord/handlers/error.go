package handlers

import "github.com/devproje/mininaru/util"

func publicFailure(operation string, err error) string {
	util.Log.Error("discord operation failed", "operation", operation, "error", err)
	return "Something went wrong while " + operation + "."
}
