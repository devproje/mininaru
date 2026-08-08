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
}

var daemonInstall *cobra.Command = &cobra.Command{
	Use:   "install",
	Short: "install and start the systemd user daemon",
	RunE:  daemonInstallExecute,
}

var daemonUninstall *cobra.Command = &cobra.Command{
	Use:   "uninstall",
	Short: "stop and remove the systemd user daemon",
	RunE:  daemonUninstallExecute,
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
WorkingDirectory=` + strconv.Quote(workingDir) + `
Environment=` + strconv.Quote("NARU_PATH="+dataDir) + `
EnvironmentFile=` + strconv.Quote(envFile) + `
ExecStart=` + strconv.Quote(binary) + ` serve
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
		return err
	}

	fmt.Printf("installed and started %s\n", daemonUnitName)
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

	fmt.Printf("removed %s\n", daemonUnitName)
	return nil
}

func init() {
	daemonInstall.Flags().StringVar(&daemonEnvFileRef, "env-file", "", "environment file containing MININARU_API_KEY")
	daemonInstall.Flags().StringVar(&daemonWorkingDirRef, "working-directory", "", "working directory exposed to built-in tools")
	daemonConfig.AddCommand(daemonInstall, daemonUninstall)
}
