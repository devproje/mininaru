// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

const (
	daemonUnitName    = "mininaru.service"
	daemonLaunchLabel = "net.projecttl.mininaru"
	daemonTaskName    = "mininaru"

	daemonEnvBegin = "# >>> mininaru env >>>"
	daemonEnvEnd   = "# <<< mininaru env <<<"
)

var (
	daemonHostRef string
	daemonPortRef uint16
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

func linuxUnit(binary, dataDir, host string, port uint16) string {
	return fmt.Sprintf(`[Unit]
Description=mininaru HTTP API server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
UMask=0077
Environment=NARU_PATH=%s
Environment=MININARU_NO_UPDATE_CHECK=1
ExecStart=%s serve --host %s --port %d
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, dataDir, binary, host, port)
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

	err = util.WriteFileAtomic(unitPath, []byte(linuxUnit(binary, util.RootDir, daemonHostRef, daemonPortRef)), 0600)
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

func darwinPlist(binary, dataDir, host string, port uint16) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>serve</string>
		<string>--host</string>
		<string>%s</string>
		<string>--port</string>
		<string>%d</string>
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
`, daemonLaunchLabel, binary, host, port, dataDir)
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

	err = util.WriteFileAtomic(plistPath, []byte(darwinPlist(binary, util.RootDir, daemonHostRef, daemonPortRef)), 0644)
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

func windowsTaskAction(binary string) string {
	return fmt.Sprintf(`"%s" serve --host %s --port %d`, binary, daemonHostRef, daemonPortRef)
}

func windowsTaskExists(ctx context.Context) bool {
	return exec.CommandContext(ctx, "schtasks", "/Query", "/TN", daemonTaskName).Run() == nil
}

func windowsDaemonInstall(ctx context.Context, binary string) error {
	var err error

	err = run(ctx, "setx", "NARU_PATH", util.RootDir)
	if err != nil {
		return err
	}

	err = run(ctx, "schtasks", "/Create", "/TN", daemonTaskName,
		"/TR", windowsTaskAction(binary), "/SC", "ONLOGON", "/RL", "LIMITED", "/F")
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

func daemonInstallExecute(cmd *cobra.Command, args []string) error {
	var binary string

	var err error

	binary, err = daemonBinary()
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "linux":
		return linuxDaemonInstall(cmd.Context(), binary)
	case "darwin":
		return darwinDaemonInstall(cmd.Context(), binary)
	case "windows":
		return windowsDaemonInstall(cmd.Context(), binary)
	default:
		return fmt.Errorf("mininaru daemon is not supported on %s", runtime.GOOS)
	}
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
