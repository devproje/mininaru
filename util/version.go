package util

import (
	"fmt"
	"runtime"
	"strings"
)

const ansiReset = "[0m"
const ansiRed = "[31m"

var (
	AppVersion = "dev"
	AppBranch  = "unknown"
	AppHash    = "unknown"
	AppDebug   = false
)

var MiniArt = []string{
	`███╗   ███╗██╗███╗   ██╗██╗`,
	`████╗ ████║██║████╗  ██║██║`,
	`██╔████╔██║██║██╔██╗ ██║██║`,
	`██║╚██╔╝██║██║██║╚██╗██║██║`,
	`██║ ╚═╝ ██║██║██║ ╚████║██║`,
	`╚═╝     ╚═╝╚═╝╚═╝  ╚═══╝╚═╝`,
}

var NaruArt = []string{
	`███╗   ██╗ █████╗ ██████╗ ██╗   ██╗`,
	`████╗  ██║██╔══██╗██╔══██╗██║   ██║`,
	`██╔██╗ ██║███████║██████╔╝██║   ██║`,
	`██║╚██╗██║██╔══██║██╔══██╗██║   ██║`,
	`██║ ╚████║██║  ██║██║  ██║╚██████╔╝`,
	`╚═╝  ╚═══╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝`,
}

func RuntimeIdentity() string {
	return fmt.Sprintf("mininaru %s-%s (branch: %s) %s/%s",
		AppVersion, AppHash, AppBranch, runtime.GOOS, runtime.GOARCH)
}

func NaruLogo() string {
	var i int
	var lines [6]string

	for i = range 6 {
		lines[i] = fmt.Sprintf("%s%s%s%s%s", ansiReset, MiniArt[i], ansiRed, NaruArt[i], ansiReset)
	}

	return strings.Join(lines[:], "\n")
}

func NaruLogoWithPad(pad string) string {
	var i int
	var lines [6]string

	for i = range 6 {
		lines[i] = fmt.Sprintf("%s%s%s%s%s%s", pad, ansiReset, MiniArt[i], ansiRed, NaruArt[i], ansiReset)
	}

	return strings.Join(lines[:], "\n")
}
