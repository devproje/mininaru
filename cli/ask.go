package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

var askIn io.Reader = os.Stdin

var askOut io.Writer = os.Stderr

var askInteractive func() bool = terminalSession

var askBuffer *bufio.Reader

var askBufferFor io.Reader

func askReader() *bufio.Reader {
	if askBuffer != nil && askBufferFor == askIn {
		return askBuffer
	}

	askBuffer = bufio.NewReader(askIn)
	askBufferFor = askIn

	return askBuffer
}

func terminalSession() bool {
	var in *os.File
	var out *os.File
	var ok bool

	in, ok = askIn.(*os.File)
	if !ok {
		return false
	}

	out, ok = askOut.(*os.File)
	if !ok {
		return false
	}

	return term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}

func askLine(label, fallback string) (string, error) {
	var answer string

	var err error

	if fallback != "" {
		fmt.Fprintf(askOut, "%s [%s]: ", label, fallback)
	} else {
		fmt.Fprintf(askOut, "%s: ", label)
	}

	answer, err = askReader().ReadString('\n')
	if err != nil && answer == "" {
		return "", err
	}

	answer = strings.TrimSpace(answer)
	if answer == "" {
		return fallback, nil
	}

	return answer, nil
}

func askText(label, fallback string) (string, error) {
	return askLine(label, fallback)
}

func askRequired(label string) (string, error) {
	var answer string

	var err error

	for {
		answer, err = askLine(label, "")
		if err != nil {
			return "", err
		}

		if answer != "" {
			return answer, nil
		}

		fmt.Fprintf(askOut, "  %s is required\n", label)
	}
}

func askSecret(label string, optional bool) (string, error) {
	var file *os.File
	var ok bool
	var buf []byte

	var err error

	file, ok = askIn.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) || askReader().Buffered() > 0 {
		return askLine(label, "")
	}

	for {
		if optional {
			fmt.Fprintf(askOut, "%s (leave empty to skip): ", label)
		} else {
			fmt.Fprintf(askOut, "%s: ", label)
		}

		buf, err = term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(askOut)
		if err != nil {
			return "", err
		}

		if optional || len(buf) > 0 {
			return strings.TrimSpace(string(buf)), nil
		}

		fmt.Fprintf(askOut, "  %s is required\n", label)
	}
}

func askChoice(label string, options []string, fallback string) (string, error) {
	var index int
	var option string
	var answer string
	var picked int

	var err error

	if len(options) == 0 {
		return fallback, nil
	}

	for index, option = range options {
		if option == fallback {
			fmt.Fprintf(askOut, "  %d) %s [current]\n", index+1, option)
			continue
		}

		fmt.Fprintf(askOut, "  %d) %s\n", index+1, option)
	}

	for {
		answer, err = askLine(label, fallback)
		if err != nil {
			return "", err
		}

		picked, err = strconv.Atoi(answer)
		if err == nil && picked >= 1 && picked <= len(options) {
			return options[picked-1], nil
		}

		for _, option = range options {
			if option == answer {
				return answer, nil
			}
		}

		fmt.Fprintf(askOut, "  pick a number from 1 to %d, or type the value\n", len(options))
	}
}

func askConfirm(label string, fallback bool) (bool, error) {
	var hint string
	var answer string

	var err error

	hint = "y/N"
	if fallback {
		hint = "Y/n"
	}

	for {
		answer, err = askLine(label+" ("+hint+")", "")
		if err != nil {
			return false, err
		}

		switch strings.ToLower(answer) {
		case "":
			return fallback, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}

		fmt.Fprintln(askOut, "  answer y or n")
	}
}
