// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type daemonPreset struct {
	Host        string
	Port        uint16
	CorsOrigins []string
	WebDir      string
}

const (
	daemonUnitName    = "mininaru.service"
	daemonLaunchLabel = "net.projecttl.mininaru"
	daemonTaskName    = "mininaru"

	daemonEnvBegin = "# >>> mininaru env >>>"
	daemonEnvEnd   = "# <<< mininaru env <<<"
)

var (
	daemonHostRef        string
	daemonPortRef        uint16
	daemonCorsOriginsRef []string
	daemonWebDirRef      string
)

var daemonCmd *cobra.Command = &cobra.Command{
	Use:   "daemon",
	Short: "run \"mininaru serve\" as a background service",
	Long: "Run `mininaru serve` in the background as a per-user service.\n\n" +
		"linux    systemd --user unit (~/.config/systemd/user/" + daemonUnitName + ")\n" +
		"macOS    launchd agent (~/Library/LaunchAgents/" + daemonLaunchLabel + ".plist)\n" +
		"windows  Scheduled Task \"" + daemonTaskName + "\" that starts at logon\n\n" +
		"The service runs with NARU_PATH pinned to the current data directory.",
	Example: "  mininaru daemon install\n  mininaru daemon restart\n  mininaru daemon uninstall",
}

var daemonInstallCmd *cobra.Command = &cobra.Command{
	Use:   "install",
	Short: "install and start the service",
	Long:  "Write the service definition and start it now. Run again to overwrite an\nexisting one with the current binary path.",
	Args:  cobra.NoArgs,
	RunE:  daemonInstallExecute,
}

var daemonRestartCmd *cobra.Command = &cobra.Command{
	Use:     "restart",
	Aliases: []string{"reload"},
	Short:   "restart the service",
	Long:    "Reload the service definition from disk and restart, so it picks up config\nchanges and a definition rewritten by a newer binary.",
	Args:    cobra.NoArgs,
	RunE:    daemonRestartExecute,
}

var daemonUninstallCmd *cobra.Command = &cobra.Command{
	Use:     "uninstall",
	Aliases: []string{"remove"},
	Short:   "stop and remove the service",
	Args:    cobra.NoArgs,
	RunE:    daemonUninstallExecute,
}

func init() {
	daemonInstallCmd.Flags().StringVar(&daemonHostRef, "host", SERVER_DEFAULT_HOST, "address to bind the server")
	daemonInstallCmd.Flags().Uint16Var(&daemonPortRef, "port", SERVER_DEFAULT_PORT, "port to bind the server")
	daemonInstallCmd.Flags().StringSliceVar(&daemonCorsOriginsRef, "cors-origin", nil, "allow cross-origin requests from this origin (repeatable)")
	daemonInstallCmd.Flags().StringVar(&daemonWebDirRef, "web-dir", "", "serve a built web client from this directory at /")

	daemonCmd.AddCommand(daemonInstallCmd, daemonRestartCmd, daemonUninstallCmd)
}

func run(ctx context.Context, name string, args ...string) error {
	var out []byte

	var err error

	out, err = exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %s", name, args, string(out))
	}

	return nil
}

func daemonBinary() (string, error) {
	var path string

	var err error

	path, err = os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(path)
}

func notInstalled(name string) error {
	return fmt.Errorf("%s is not installed, run `mininaru daemon install` first", name)
}

func shellRcPath() (string, error) {
	var home string
	var zdotdir string

	var err error

	home, err = os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if filepath.Base(os.Getenv("SHELL")) != "zsh" {
		return filepath.Join(home, ".bashrc"), nil
	}

	zdotdir = os.Getenv("ZDOTDIR")
	if zdotdir != "" {
		home = zdotdir
	}

	return filepath.Join(home, ".zshrc"), nil
}

func pinNaruPath(dir string) {
	var rc string
	var body []byte
	var block string
	var file *os.File

	var err error

	rc, err = shellRcPath()
	if err != nil {
		return
	}

	body, err = os.ReadFile(rc)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if strings.Contains(string(body), daemonEnvBegin) {
		return
	}

	block = fmt.Sprintf("\n%s\nexport NARU_PATH=%q\n%s\n", daemonEnvBegin, dir, daemonEnvEnd)

	file, err = os.OpenFile(rc, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	_, err = file.WriteString(block)
	if err == nil {
		fmt.Printf("  pinned NARU_PATH=%s in %s\n", dir, rc)
	}
}

func unpinNaruPath() {
	var rc string
	var body []byte
	var begin int
	var end int
	var trimmed []byte

	var err error

	rc, err = shellRcPath()
	if err != nil {
		return
	}

	body, err = os.ReadFile(rc)
	if err != nil {
		return
	}

	begin = strings.Index(string(body), daemonEnvBegin)
	end = strings.Index(string(body), daemonEnvEnd)
	if begin < 0 || end < begin {
		return
	}

	end += len(daemonEnvEnd)
	if end < len(body) && body[end] == '\n' {
		end++
	}
	if begin > 0 && body[begin-1] == '\n' && (begin < 2 || body[begin-2] == '\n') {
		begin--
	}

	trimmed = append(body[:begin:begin], body[end:]...)

	err = os.WriteFile(rc, trimmed, 0644)
	if err == nil {
		fmt.Printf("  removed the NARU_PATH pin from %s\n", rc)
	}
}

func systemctl(ctx context.Context, args ...string) error {
	return run(ctx, "systemctl", append([]string{"--user"}, args...)...)
}

func linuxRequireSystemctl() error {
	var err error

	_, err = exec.LookPath("systemctl")
	if err != nil {
		return fmt.Errorf("systemctl not found; the daemon command needs systemd on Linux")
	}

	return nil
}

func linuxUnitPath() (string, error) {
	var dir string

	var err error

	dir, err = os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "systemd", "user", daemonUnitName), nil
}

func quoteArg(s string) string {
	if strings.ContainsAny(s, " \t") {
		return "'" + s + "'"
	}

	return s
}

func linuxUnit(binary, dataDir string, args []string) string {
	var quoted []string
	var i int

	quoted = make([]string, len(args))
	for i = range args {
		quoted[i] = quoteArg(args[i])
	}

	return fmt.Sprintf(`[Unit]
Description=mininaru HTTP API server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
UMask=0077
Environment=NARU_PATH=%s
Environment=MININARU_NO_UPDATE_CHECK=1
ExecStart=%s %s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, dataDir, quoteArg(binary), strings.Join(quoted, " "))
}

func shellTokenize(line string) []string {
	var pattern *regexp.Regexp
	var matches []string
	var i int

	pattern = regexp.MustCompile(`'[^']*'|"[^"]*"|\S+`)
	matches = pattern.FindAllString(line, -1)

	for i = range matches {
		matches[i] = strings.Trim(matches[i], `'"`)
	}

	return matches
}

func parseServeArgs(tokens []string) daemonPreset {
	var i int
	var preset daemonPreset
	var port uint64

	for i = 0; i < len(tokens); i++ {
		switch tokens[i] {
		case "--host":
			i++
			if i < len(tokens) {
				preset.Host = tokens[i]
			}
		case "--port":
			i++
			if i < len(tokens) {
				port, _ = strconv.ParseUint(tokens[i], 10, 16)
				preset.Port = uint16(port)
			}
		case "--cors-origin":
			i++
			if i < len(tokens) {
				preset.CorsOrigins = append(preset.CorsOrigins, tokens[i])
			}
		case "--web-dir":
			i++
			if i < len(tokens) {
				preset.WebDir = tokens[i]
			}
		}
	}

	return preset
}

func linuxExistingConfig(ctx context.Context) (daemonPreset, bool, error) {
	var unitPath string
	var body []byte
	var pattern *regexp.Regexp
	var match []string

	var err error

	unitPath, err = linuxUnitPath()
	if err != nil {
		return daemonPreset{}, false, err
	}

	body, err = os.ReadFile(unitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return daemonPreset{}, false, nil
		}

		return daemonPreset{}, false, err
	}

	pattern = regexp.MustCompile(`(?m)^ExecStart=(.*)$`)
	match = pattern.FindStringSubmatch(string(body))
	if match == nil {
		return daemonPreset{}, true, nil
	}

	return parseServeArgs(shellTokenize(match[1])), true, nil
}

func daemonExecArgs() []string {
	var args []string
	var origin string

	args = []string{"serve", "--host", daemonHostRef, "--port", strconv.Itoa(int(daemonPortRef))}

	for _, origin = range daemonCorsOriginsRef {
		args = append(args, "--cors-origin", origin)
	}

	if daemonWebDirRef != "" {
		args = append(args, "--web-dir", daemonWebDirRef)
	}

	return args
}

func linuxDaemonInstall(ctx context.Context, binary string) error {
	var unitPath string

	var err error

	err = linuxRequireSystemctl()
	if err != nil {
		return err
	}

	unitPath, err = linuxUnitPath()
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(unitPath), 0700)
	if err != nil {
		return err
	}

	err = util.WriteFileAtomic(unitPath, []byte(linuxUnit(binary, util.RootDir, daemonExecArgs())), 0600)
	if err != nil {
		return err
	}

	err = systemctl(ctx, "daemon-reload")
	if err != nil {
		return err
	}

	err = systemctl(ctx, "enable", "--now", daemonUnitName)
	if err != nil {
		return err
	}

	pinNaruPath(util.RootDir)

	fmt.Printf("installed and started %s\n", daemonUnitName)
	fmt.Printf("  unit    %s\n", unitPath)
	fmt.Printf("  status  systemctl --user status %s\n", daemonUnitName)
	fmt.Printf("  logs    journalctl --user -u %s -f\n", daemonUnitName)
	fmt.Println("  note    run `loginctl enable-linger` to keep it running after logout")

	return nil
}

func linuxDaemonRestart(ctx context.Context) error {
	var unitPath string

	var err error

	err = linuxRequireSystemctl()
	if err != nil {
		return err
	}

	unitPath, err = linuxUnitPath()
	if err != nil {
		return err
	}

	_, err = os.Stat(unitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return notInstalled(daemonUnitName)
		}
		return err
	}

	err = systemctl(ctx, "daemon-reload")
	if err != nil {
		return err
	}

	err = systemctl(ctx, "restart", daemonUnitName)
	if err != nil {
		return err
	}

	fmt.Printf("restarted %s\n", daemonUnitName)

	return nil
}

func linuxDaemonUninstall(ctx context.Context) error {
	var unitPath string

	var err error

	err = linuxRequireSystemctl()
	if err != nil {
		return err
	}

	unitPath, err = linuxUnitPath()
	if err != nil {
		return err
	}

	_, err = os.Stat(unitPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("%s was not installed\n", daemonUnitName)
			return nil
		}
		return err
	}

	err = systemctl(ctx, "disable", "--now", daemonUnitName)
	if err != nil {
		return err
	}

	err = os.Remove(unitPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	err = systemctl(ctx, "daemon-reload")
	if err != nil {
		return err
	}

	unpinNaruPath()

	fmt.Printf("removed %s\n", daemonUnitName)

	return nil
}

func darwinPlistPath() (string, error) {
	var home string

	var err error

	home, err = os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "Library", "LaunchAgents", daemonLaunchLabel+".plist"), nil
}

func darwinProgramArguments(binary string, args []string) string {
	var builder strings.Builder
	var arg string

	fmt.Fprintf(&builder, "\t\t<string>%s</string>\n", binary)

	for _, arg = range args {
		fmt.Fprintf(&builder, "\t\t<string>%s</string>\n", arg)
	}

	return strings.TrimRight(builder.String(), "\n")
}

func darwinPlist(binary, dataDir string, args []string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s
	</array>
	<key>EnvironmentVariables</key>
	<dict>
		<key>NARU_PATH</key>
		<string>%s</string>
		<key>MININARU_NO_UPDATE_CHECK</key>
		<string>1</string>
	</dict>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
`, daemonLaunchLabel, darwinProgramArguments(binary, args), dataDir)
}

func darwinExistingConfig(ctx context.Context) (daemonPreset, bool, error) {
	var plistPath string
	var body []byte
	var arrayPattern *regexp.Regexp
	var arrayMatch []string
	var stringPattern *regexp.Regexp
	var matches [][]string
	var m []string
	var tokens []string

	var err error

	plistPath, err = darwinPlistPath()
	if err != nil {
		return daemonPreset{}, false, err
	}

	body, err = os.ReadFile(plistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return daemonPreset{}, false, nil
		}

		return daemonPreset{}, false, err
	}

	arrayPattern = regexp.MustCompile(`(?s)<key>ProgramArguments</key>\s*<array>(.*?)</array>`)
	arrayMatch = arrayPattern.FindStringSubmatch(string(body))
	if arrayMatch == nil {
		return daemonPreset{}, true, nil
	}

	stringPattern = regexp.MustCompile(`<string>(.*?)</string>`)
	matches = stringPattern.FindAllStringSubmatch(arrayMatch[1], -1)
	for _, m = range matches {
		tokens = append(tokens, m[1])
	}

	return parseServeArgs(tokens), true, nil
}

func darwinDaemonInstall(ctx context.Context, binary string) error {
	var plistPath string

	var err error

	plistPath, err = darwinPlistPath()
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(plistPath), 0755)
	if err != nil {
		return err
	}

	err = util.WriteFileAtomic(plistPath, []byte(darwinPlist(binary, util.RootDir, daemonExecArgs())), 0644)
	if err != nil {
		return err
	}

	run(ctx, "launchctl", "unload", plistPath)

	err = run(ctx, "launchctl", "load", "-w", plistPath)
	if err != nil {
		return err
	}

	pinNaruPath(util.RootDir)

	fmt.Printf("installed and started %s\n", daemonLaunchLabel)
	fmt.Printf("  plist   %s\n", plistPath)
	fmt.Printf("  status  launchctl print gui/%d/%s\n", os.Getuid(), daemonLaunchLabel)

	return nil
}

func darwinDaemonRestart(ctx context.Context) error {
	var plistPath string

	var err error

	plistPath, err = darwinPlistPath()
	if err != nil {
		return err
	}

	_, err = os.Stat(plistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return notInstalled(daemonLaunchLabel)
		}
		return err
	}

	err = run(ctx, "launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/%s", os.Getuid(), daemonLaunchLabel))
	if err != nil {
		return err
	}

	fmt.Printf("restarted %s\n", daemonLaunchLabel)

	return nil
}

func darwinDaemonUninstall(ctx context.Context) error {
	var plistPath string

	var err error

	plistPath, err = darwinPlistPath()
	if err != nil {
		return err
	}

	_, err = os.Stat(plistPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("%s was not installed\n", daemonLaunchLabel)
			return nil
		}
		return err
	}

	run(ctx, "launchctl", "unload", "-w", plistPath)

	err = os.Remove(plistPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	unpinNaruPath()

	fmt.Printf("removed %s\n", daemonLaunchLabel)

	return nil
}

func winQuoteArg(s string) string {
	if strings.Contains(s, " ") {
		return `"` + s + `"`
	}

	return s
}

func windowsTaskAction(binary string, args []string) string {
	var quoted []string
	var i int

	quoted = make([]string, len(args))
	for i = range args {
		quoted[i] = winQuoteArg(args[i])
	}

	return fmt.Sprintf(`"%s" %s`, binary, strings.Join(quoted, " "))
}

func windowsTaskExists(ctx context.Context) bool {
	return exec.CommandContext(ctx, "schtasks", "/Query", "/TN", daemonTaskName).Run() == nil
}

func windowsExistingConfig(ctx context.Context) (daemonPreset, bool, error) {
	var out []byte
	var pattern *regexp.Regexp
	var match []string

	var err error

	if !windowsTaskExists(ctx) {
		return daemonPreset{}, false, nil
	}

	out, err = exec.CommandContext(ctx, "schtasks", "/Query", "/TN", daemonTaskName, "/V", "/FO", "LIST").Output()
	if err != nil {
		return daemonPreset{}, true, err
	}

	pattern = regexp.MustCompile(`(?m)^Task To Run:\s*(.*)$`)
	match = pattern.FindStringSubmatch(string(out))
	if match == nil {
		return daemonPreset{}, true, nil
	}

	return parseServeArgs(shellTokenize(match[1])), true, nil
}

func windowsDaemonInstall(ctx context.Context, binary string) error {
	var err error

	err = run(ctx, "setx", "NARU_PATH", util.RootDir)
	if err != nil {
		return err
	}

	err = run(ctx, "schtasks", "/Create", "/TN", daemonTaskName,
		"/TR", windowsTaskAction(binary, daemonExecArgs()), "/SC", "ONLOGON", "/RL", "LIMITED", "/F")
	if err != nil {
		return err
	}

	err = run(ctx, "schtasks", "/Run", "/TN", daemonTaskName)
	if err != nil {
		return err
	}

	fmt.Printf("installed and started Scheduled Task %q\n", daemonTaskName)
	fmt.Printf("  NARU_PATH  %s (pinned as a user environment variable)\n", util.RootDir)
	fmt.Printf("  status     schtasks /Query /TN %s /V /FO LIST\n", daemonTaskName)

	return nil
}

func windowsDaemonRestart(ctx context.Context) error {
	var err error

	if !windowsTaskExists(ctx) {
		return notInstalled("Scheduled Task " + strconv.Quote(daemonTaskName))
	}

	run(ctx, "schtasks", "/End", "/TN", daemonTaskName)

	err = run(ctx, "schtasks", "/Run", "/TN", daemonTaskName)
	if err != nil {
		return err
	}

	fmt.Printf("restarted Scheduled Task %q\n", daemonTaskName)

	return nil
}

func windowsDaemonUninstall(ctx context.Context) error {
	var err error

	if !windowsTaskExists(ctx) {
		fmt.Printf("Scheduled Task %q was not installed\n", daemonTaskName)
		return nil
	}

	run(ctx, "schtasks", "/End", "/TN", daemonTaskName)

	err = run(ctx, "schtasks", "/Delete", "/TN", daemonTaskName, "/F")
	if err != nil {
		return err
	}

	run(ctx, "setx", "NARU_PATH", "")

	fmt.Printf("removed Scheduled Task %q\n", daemonTaskName)
	fmt.Println("  cleared the NARU_PATH user environment variable")

	return nil
}

func daemonWizardNeeded(cmd *cobra.Command) bool {
	var changed bool

	changed = cmd.Flags().Changed("host") || cmd.Flags().Changed("port") ||
		cmd.Flags().Changed("cors-origin") || cmd.Flags().Changed("web-dir")

	return !changed && term.IsTerminal(int(os.Stdin.Fd()))
}

func existingConfig(ctx context.Context) (daemonPreset, bool, error) {
	switch runtime.GOOS {
	case "linux":
		return linuxExistingConfig(ctx)
	case "darwin":
		return darwinExistingConfig(ctx)
	case "windows":
		return windowsExistingConfig(ctx)
	default:
		return daemonPreset{}, false, nil
	}
}

func applyPreset(preset daemonPreset) {
	if preset.Host != "" {
		daemonHostRef = preset.Host
	}
	if preset.Port != 0 {
		daemonPortRef = preset.Port
	}
	if len(preset.CorsOrigins) > 0 {
		daemonCorsOriginsRef = preset.CorsOrigins
	}
	if preset.WebDir != "" {
		daemonWebDirRef = preset.WebDir
	}
}

func askDefault(reader *bufio.Reader, label string, def string) string {
	var line string

	var err error

	if def != "" {
		fmt.Printf("  %s [%s]: ", label, def)
	} else {
		fmt.Printf("  %s: ", label)
	}

	line, err = reader.ReadString('\n')
	if err != nil {
		return def
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}

	return line
}

func daemonWizard(preset daemonPreset, existed bool) {
	var reader *bufio.Reader
	var text string
	var port uint64
	var origins []string
	var i int
	var info os.FileInfo

	var err error

	if existed {
		applyPreset(preset)

		fmt.Println("mininaru daemon setup (updating the already-installed service)")
	} else {
		fmt.Println("mininaru daemon setup")
	}

	reader = bufio.NewReader(os.Stdin)

	daemonHostRef = askDefault(reader, "host", daemonHostRef)

	text = askDefault(reader, "port", strconv.Itoa(int(daemonPortRef)))
	port, err = strconv.ParseUint(text, 10, 16)
	if err == nil {
		daemonPortRef = uint16(port)
	}

	text = askDefault(reader, "cors origins, comma separated (needed for a browser web client on another origin)", strings.Join(daemonCorsOriginsRef, ","))
	if text == "" {
		daemonCorsOriginsRef = nil
	} else {
		origins = strings.Split(text, ",")
		for i = range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		daemonCorsOriginsRef = origins
	}

	daemonWebDirRef = askDefault(reader, "web client directory to serve at / (blank = api only)", daemonWebDirRef)
	if daemonWebDirRef != "" {
		info, err = os.Stat(daemonWebDirRef)
		if err != nil || !info.IsDir() {
			fmt.Printf("  warning: %s does not look like a directory yet\n", daemonWebDirRef)
		}
	}

	fmt.Println()
}

func confirmYesNo(reader *bufio.Reader, label string) bool {
	var line string

	var err error

	fmt.Printf("%s [y/N]: ", label)

	line, err = reader.ReadString('\n')
	if err != nil {
		return false
	}

	line = strings.ToLower(strings.TrimSpace(line))

	return line == "y" || line == "yes"
}

func daemonConfirmPlan(binary string, existed bool) bool {
	var reader *bufio.Reader

	reader = bufio.NewReader(os.Stdin)

	fmt.Println("about to install:")
	fmt.Printf("  binary    %s\n", binary)
	fmt.Printf("  data dir  %s\n", util.RootDir)
	fmt.Printf("  host      %s\n", daemonHostRef)
	fmt.Printf("  port      %d\n", daemonPortRef)

	if len(daemonCorsOriginsRef) > 0 {
		fmt.Printf("  cors      %s\n", strings.Join(daemonCorsOriginsRef, ", "))
	} else {
		fmt.Println("  cors      (none)")
	}

	if daemonWebDirRef != "" {
		fmt.Printf("  web-dir   %s\n", daemonWebDirRef)
	} else {
		fmt.Println("  web-dir   (none, api only)")
	}

	if existed {
		fmt.Println("  note      this replaces the already-installed service")
	}

	fmt.Println()

	return confirmYesNo(reader, "proceed")
}

func daemonInstallExecute(cmd *cobra.Command, args []string) error {
	var binary string
	var preset daemonPreset
	var existed bool

	var err error

	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return fmt.Errorf("mininaru daemon is not supported on %s", runtime.GOOS)
	}

	binary, err = daemonBinary()
	if err != nil {
		return err
	}

	if daemonWizardNeeded(cmd) {
		preset, existed, err = existingConfig(cmd.Context())
		if err != nil {
			return err
		}

		daemonWizard(preset, existed)

		if !daemonConfirmPlan(binary, existed) {
			fmt.Println("aborted")

			return nil
		}
	}

	switch runtime.GOOS {
	case "linux":
		return linuxDaemonInstall(cmd.Context(), binary)
	case "darwin":
		return darwinDaemonInstall(cmd.Context(), binary)
	case "windows":
		return windowsDaemonInstall(cmd.Context(), binary)
	}

	return nil
}

func daemonRestartExecute(cmd *cobra.Command, args []string) error {
	switch runtime.GOOS {
	case "linux":
		return linuxDaemonRestart(cmd.Context())
	case "darwin":
		return darwinDaemonRestart(cmd.Context())
	case "windows":
		return windowsDaemonRestart(cmd.Context())
	default:
		return fmt.Errorf("mininaru daemon is not supported on %s", runtime.GOOS)
	}
}

func daemonUninstallExecute(cmd *cobra.Command, args []string) error {
	switch runtime.GOOS {
	case "linux":
		return linuxDaemonUninstall(cmd.Context())
	case "darwin":
		return darwinDaemonUninstall(cmd.Context())
	case "windows":
		return windowsDaemonUninstall(cmd.Context())
	default:
		return fmt.Errorf("mininaru daemon is not supported on %s", runtime.GOOS)
	}
}
