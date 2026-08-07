package util

import (
	"fmt"
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

func NaruLogo() string {
	var lines [6]string
	var i int

	for i = range 6 {
		lines[i] = fmt.Sprintf("%s%s%s%s%s", ansiReset, MiniArt[i], ansiRed, NaruArt[i], ansiReset)
	}

	return strings.Join(lines[:], "\n")
}

func NaruLogoWithPad(pad string) string {
	var lines [6]string
	var i int

	for i = range 6 {
		lines[i] = fmt.Sprintf("%s%s%s%s%s%s", pad, ansiReset, MiniArt[i], ansiRed, NaruArt[i], ansiReset)
	}

	return strings.Join(lines[:], "\n")
}
