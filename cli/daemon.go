// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

const daemonUnitName = "mininaru.service"

var (
	daemonEnvFileRef    string
	daemonWorkingDirRef string
)

var daemonConfig *cobra.Command = &cobra.Command{
	Use:   "daemon",
	Short: "manage the systemd user daemon",
	Long: `Run ` + "`mininaru serve`" + ` in the background as a systemd user unit.

The unit reads its API key from an environment file that must exist beforehand
and be readable only by you. Linux only.`,
	Example: `  mininaru daemon install
  mininaru daemon reload
  mininaru daemon uninstall`,
}

var daemonInstall *cobra.Command = &cobra.Command{
	Use:   "install",
	Short: "install and start the systemd user daemon",
	Long: `Write the systemd user unit, enable it and start it.

Running this again overwrites an existing unit with the current binary path and
working directory. User units stop at logout unless lingering is enabled.`,
	Example: `  mininaru daemon install
  mininaru daemon install --env-file ~/.config/mininaru/env`,
	Args: usageArgs(cobra.NoArgs),
	RunE: daemonInstallExecute,
}

var daemonReload *cobra.Command = &cobra.Command{
	Use:     "reload",
	Aliases: []string{"restart"},
	Short:   "restart the systemd user daemon",
	Long: `Restart the daemon so it picks up the current configuration.

Providers, agents, bots and mcp servers are read once at startup, so a restart
is what applies changes made with the other subcommands. The unit file is read
from disk again first, which also picks up a unit rewritten by a newer binary.`,
	Example: `  mininaru daemon reload`,
	Args:    usageArgs(cobra.NoArgs),
	RunE:    daemonReloadExecute,
}

var daemonUninstall *cobra.Command = &cobra.Command{
	Use:     "uninstall",
	Aliases: []string{"remove"},
	Short:   "stop and remove the systemd user daemon",
	Args:    usageArgs(cobra.NoArgs),
	RunE:    daemonUninstallExecute,
}

func daemonPaths() (string, string, error) {
	var configDir string
	var unitDir string
	var envFile string

	var err error

	configDir, err = os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	unitDir = filepath.Join(configDir, "systemd", "user")
	envFile = daemonEnvFileRef
	if envFile == "" {
		envFile = filepath.Join(configDir, "mininaru", "env")
	}
	envFile, err = filepath.Abs(envFile)
	if err != nil {
		return "", "", err
	}

	return filepath.Join(unitDir, daemonUnitName), envFile, nil
}

func daemonEnvValid(path string) error {
	var info os.FileInfo
	var file *os.File
	var scanner *bufio.Scanner
	var line string
	var value string
	var found bool

	var err error

	info, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("daemon environment file is required: create %s with mode 0600 and set %s", path, apiKeyEnv)
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("daemon environment file is not a regular file: %s", path)
	}
	if info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("daemon environment file must not be accessible by group or others: chmod 600 %s", path)
	}

	file, err = os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		line = strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		value, found = strings.CutPrefix(line, apiKeyEnv+"=")
		if found && strings.Trim(strings.TrimSpace(value), "\"'") != "" {
			return nil
		}
	}
	if err = scanner.Err(); err != nil {
		return err
	}

	return fmt.Errorf("daemon environment file does not define %s", apiKeyEnv)
}

func daemonUnit(binary, dataDir, workingDir, envFile string) string {
	return `[Unit]
Description=mininaru HTTP API server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
UMask=0077
WorkingDirectory=` + workingDir + `
Environment=` + "NARU_PATH=" + dataDir + `
EnvironmentFile=` + envFile + `
ExecStart=` + binary + ` serve
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`
}

func systemctlUser(ctx context.Context, args ...string) error {
	var command *exec.Cmd
	var output []byte

	var err error

	command = exec.CommandContext(ctx, "systemctl", append([]string{"--user"}, args...)...)
	output, err = command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl --user %s failed: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return nil
}

func daemonInstallExecute(cmd *cobra.Command, args []string) error {
	var unitPath string
	var envFile string
	var binary string
	var workingDir string

	var err error

	if _, err = exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl is required: %w", err)
	}
	unitPath, envFile, err = daemonPaths()
	if err != nil {
		return err
	}
	if err = daemonEnvValid(envFile); err != nil {
		return err
	}
	binary, err = os.Executable()
	if err != nil {
		return err
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return err
	}
	workingDir = daemonWorkingDirRef
	if workingDir == "" {
		workingDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	workingDir, err = filepath.Abs(workingDir)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(unitPath), 0700); err != nil {
		return err
	}
	if err = util.WriteFileAtomic(unitPath, []byte(daemonUnit(binary, util.RootDir, workingDir, envFile)), 0600); err != nil {
		return err
	}
	if err = systemctlUser(cmd.Context(), "daemon-reload"); err != nil {
		return err
	}
	if err = systemctlUser(cmd.Context(), "enable", "--now", daemonUnitName); err != nil {
		uiNote("the unit was written to %s, remove it with `mininaru daemon uninstall`", unitPath)

		return err
	}

	uiOk("installed and started %s", daemonUnitName)
	uiNote("unit:   %s", unitPath)
	uiNote("env:    %s", envFile)
	uiNote("status: systemctl --user status %s", daemonUnitName)
	uiNote("logs:   journalctl --user -u %s -f", daemonUnitName)

	if !daemonLingering(cmd.Context()) {
		uiNote("the daemon stops when you log out, run `loginctl enable-linger` to keep it running")
	}

	return nil
}

func daemonLingering(ctx context.Context) bool {
	var out []byte

	var err error

	out, err = exec.CommandContext(ctx, "loginctl", "show-user", strconv.Itoa(os.Getuid()), "--property=Linger").Output()
	if err != nil {
		return true
	}

	return strings.Contains(string(out), "Linger=yes")
}

func daemonActiveState(ctx context.Context) string {
	var out []byte

	out, _ = exec.CommandContext(ctx, "systemctl", "--user", "is-active", daemonUnitName).Output()

	return strings.TrimSpace(string(out))
}

func daemonInstalled() (bool, error) {
	var unitPath string

	var err error

	unitPath, _, err = daemonPaths()
	if err != nil {
		return false, err
	}

	_, err = os.Stat(unitPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func daemonRestart(ctx context.Context) error {
	var state string

	var err error

	if _, err = exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl is required: %w", err)
	}
	if err = systemctlUser(ctx, "daemon-reload"); err != nil {
		return err
	}
	if err = systemctlUser(ctx, "restart", daemonUnitName); err != nil {
		return err
	}

	state = daemonActiveState(ctx)
	if state != "active" && state != "activating" {
		uiNote("logs: journalctl --user -u %s -n 50", daemonUnitName)

		return fmt.Errorf("%s is %s after the restart", daemonUnitName, state)
	}

	return nil
}

func daemonReloadExecute(cmd *cobra.Command, args []string) error {
	var installed bool

	var err error

	if _, err = exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl is required: %w", err)
	}
	installed, err = daemonInstalled()
	if err != nil {
		return err
	}
	if !installed {
		return configErrorf("%s is not installed, run `mininaru daemon install` first", daemonUnitName)
	}
	if err = daemonRestart(cmd.Context()); err != nil {
		return err
	}

	uiOk("restarted %s", daemonUnitName)

	return nil
}

func daemonUninstallExecute(cmd *cobra.Command, args []string) error {
	var unitPath string
	var installed bool

	var err error

	unitPath, _, err = daemonPaths()
	if err != nil {
		return err
	}
	if _, err = exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl is required: %w", err)
	}
	if _, err = os.Stat(unitPath); err == nil {
		installed = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if installed {
		if err = systemctlUser(cmd.Context(), "disable", "--now", daemonUnitName); err != nil {
			return err
		}
	}
	if err = os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = systemctlUser(cmd.Context(), "daemon-reload"); err != nil {
		return err
	}

	if !installed {
		uiNote("%s was not installed", daemonUnitName)

		return nil
	}

	uiOk("removed %s", daemonUnitName)

	return nil
}

func init() {
	daemonInstall.Flags().StringVar(&daemonEnvFileRef, "env-file", "", "environment file containing MININARU_API_KEY")
	daemonInstall.Flags().StringVar(&daemonWorkingDirRef, "working-directory", "", "working directory exposed to built-in tools")
	daemonConfig.AddCommand(daemonInstall, daemonReload, daemonUninstall)
}
