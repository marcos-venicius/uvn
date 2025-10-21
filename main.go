package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const VERSION = "1.1.1"
const EMPTY_STRING = "<empty>"

const (
	vpnFilePathConfigKey     = "vpn_file_path"
	vpnAuthFilePathConfigKey = "vpn_auth_file_path"
)

var userConfigurationPath string = os.ExpandEnv("$HOME/.uvn.conf")

type UserConfigurations struct {
	vpnPath         string
	vpnAuthFilePath string
}

type VPNManager struct {
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	stdout    bytes.Buffer
	stderr    bytes.Buffer
	upChan    chan bool
	arguments Arguments
}

func doesPathExists(path string) (bool, error) {
	_, err := os.Stat(path)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func LoadUserConfigurations() (*UserConfigurations, error) {
	userHomeDir, err := os.UserHomeDir()

	if err != nil {
		return nil, err
	}

	ok, err := doesPathExists(userConfigurationPath)

	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("please create a config file at %s and add respective configurations. see --help for more details", userConfigurationPath)
	}

	data, err := os.ReadFile(userConfigurationPath)

	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")

	configs := UserConfigurations{
		vpnPath:         EMPTY_STRING,
		vpnAuthFilePath: EMPTY_STRING,
	}

	for index, line := range lines {
		lineNumber := index + 1
		line = strings.TrimSpace(line)

		if strings.TrimSpace(line) == "" {
			continue
		}

		var index = strings.Index(line, "=")

		if index < 0 {
			return nil, fmt.Errorf("line %d: invalid config line: %s", lineNumber, line)
		}

		key := strings.TrimSpace(line[:index])

		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNumber)
		}

		if index+1 >= len(line) {
			return nil, fmt.Errorf("line %d: %s does not have a value", lineNumber, key)
		}

		value := strings.TrimSpace(line[index+1:])

		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			value = value[1 : len(value)-1]
		}

		if value == "" {
			return nil, fmt.Errorf("line %d: %s does not have a value", lineNumber, key)
		}

		if key == vpnFilePathConfigKey || key == vpnAuthFilePathConfigKey {
			value = strings.Replace(value, "~", userHomeDir, 1)
		}

		switch key {
		case vpnFilePathConfigKey:
			configs.vpnPath = value
		case vpnAuthFilePathConfigKey:
			configs.vpnAuthFilePath = value
		}
	}

	if configs.vpnPath == EMPTY_STRING {
		return nil, fmt.Errorf("missing %s configuration", vpnFilePathConfigKey)
	}

	return &configs, nil
}

func NewVPNManager(arguments Arguments) *VPNManager {
	return &VPNManager{
		upChan:    make(chan bool, 1),
		arguments: arguments,
	}
}

func (v *VPNManager) Start(vpnPath string, userConfigurations *UserConfigurations) error {
	command := make([]string, 0, 5)

	command = append(command, "openvpn")
	command = append(command, "--config")
	command = append(command, userConfigurations.vpnPath)

	if userConfigurations.vpnAuthFilePath != EMPTY_STRING {
		command = append(command, "--auth-user-pass")
		command = append(command, userConfigurations.vpnAuthFilePath)
	}

	ctx, cancel := context.WithCancel(context.Background())

	v.cancel = cancel

	v.cmd = exec.CommandContext(ctx, "sudo", command...)

	stdoutPipe, err := v.cmd.StdoutPipe()

	if err != nil {
		return err
	}

	stderrPipe, err := v.cmd.StderrPipe()

	if err != nil {
		return err
	}

	if err := v.cmd.Start(); err != nil {
		return err
	}

	go v.monitorOutput(stdoutPipe, stderrPipe)

	go func() {
		v.cmd.Wait()
		close(v.upChan)
	}()

	return nil
}

func (v *VPNManager) monitorOutput(stdoutPipe, stderrPipe io.ReadCloser) {
	stdout := bufio.NewScanner(stdoutPipe)
	stderr := bufio.NewScanner(stderrPipe)

	for stdout.Scan() {
		line := stdout.Text()

		if v.arguments.verbose {
			fmt.Println("[VPN STDOUT]", line)
		}

		if strings.Contains(line, "Initialization Sequence Completed") {
			v.upChan <- true
		}
	}

	for stderr.Scan() {
		line := stderr.Text()

		if v.arguments.verbose {
			fmt.Println("[VPN STDERR]", line)
		}
	}
}

func (v *VPNManager) WaitUntilUp(timeout time.Duration) bool {
	select {
	case <-v.upChan:
		if v.arguments.verbose {
			fmt.Println("+ VPN is up and running!!")
		}
		return true
	case <-time.After(timeout):
		fmt.Println("- Timeout waiting for VPN to connect.")
		return false
	}
}

func (v *VPNManager) Stop() error {
	if v.cancel != nil {
		v.cancel()
	}

	if v.cmd.Process != nil {
		return v.cmd.Process.Kill()
	}

	return nil
}

func Usage(programName string) {
	fmt.Fprintf(os.Stderr, "usage: %s <command to run inside vpn>\n", programName)
	fmt.Fprintf(os.Stderr, "  You need to have a configuration file at %s\n", userConfigurationPath)
	fmt.Fprintf(os.Stderr, "  in this file, you should set up \"%s\" which is a string with the absolute path to your vpn configuration file\n", vpnFilePathConfigKey)
	fmt.Fprintf(os.Stderr, "  you also can setup \"%s\" which is a string with the absolute path to your vpn auth-user-pass configuration file\n", vpnAuthFilePathConfigKey)
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  -h  --help        show this message\n")
	fmt.Fprintf(os.Stderr, "  -v  --verbose     verbose mode\n")
	fmt.Fprintf(os.Stderr, "      --version     show current version\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  this program will get the VPN up and then run the command passed as arguments\n")
	fmt.Fprintf(os.Stderr, "  for example \"%s git push --force\" will get the VPN up and running then, execute \"git push --force\" from the directory your are running this program\n", programName)
}

type Arguments struct {
	input       []string
	programName string
	verbose     bool
}

func ParseArguments() Arguments {
	args := os.Args
	index := 0

	shift := func() *string {
		if index+1 > len(args) {
			return nil
		}

		value := args[index]

		index += 1

		return &value
	}

	programName := *shift()

	arg := shift()

	arguments := Arguments{
		input:       make([]string, 0, 10),
		verbose:     false,
		programName: programName,
	}

	for arg != nil {
		if len(arguments.input) == 0 {
			switch *arg {
			case "-h", "--help":
				Usage(programName)
				os.Exit(0)
			case "-v", "--verbose":
				arguments.verbose = true
			case "--version":
				fmt.Fprintln(os.Stdout, VERSION)
				os.Exit(0)
			default:
				arguments.input = append(arguments.input, *arg)
			}
		} else {
			arguments.input = append(arguments.input, *arg)
		}

		arg = shift()
	}

	return arguments
}

func main() {
	arguments := ParseArguments()

	if len(arguments.input) == 0 {
		Usage(arguments.programName)

		os.Exit(1)
	}

	configs, err := LoadUserConfigurations()

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	vpn := NewVPNManager(arguments)

	if err := vpn.Start(configs.vpnPath, configs); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start VPN due to: %s\n", err)
		os.Exit(1)
	}

	if !vpn.WaitUntilUp(15 * time.Second) {
		fmt.Fprintf(os.Stderr, "VPN did not start in time\n")
		if err := vpn.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "error stopping VPN: %s\n", err)
		}
		os.Exit(1)
	}

	defer func() {
		if arguments.verbose {
			fmt.Fprintf(os.Stderr, "+ shutting down VPN...\n")
		}

		if err := vpn.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "error stopping VPN: %s\n", err)
		} else {
			if arguments.verbose {
				fmt.Fprintf(os.Stderr, "+ success\n")
			}
		}
	}()

	cmd := exec.Command(arguments.input[0], arguments.input[1:]...)

	cmd.Dir, err = os.Getwd()

	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting current working dir: %s\n", err)
		os.Exit(1)
	}

	stdoutPipe, err := cmd.StdoutPipe()

	if err != nil {
		fmt.Fprintf(os.Stderr, "error capturing stdout: %s\n", err)
		os.Exit(1)
	}

	stderrPipe, err := cmd.StderrPipe()

	if err != nil {
		fmt.Fprintf(os.Stderr, "error capturing stderr: %s\n", err)
		os.Exit(1)
	}

	if arguments.verbose {
		fmt.Fprintf(os.Stderr, "+ running %+v at %s\n", arguments.input, cmd.Dir)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error running %+v: %s\n", arguments.input, err)
		os.Exit(1)
	}

	go func() {
		stdout := bufio.NewScanner(stdoutPipe)
		stderr := bufio.NewScanner(stderrPipe)

		if arguments.verbose {
			fmt.Println()
			fmt.Println()
		}

		for stdout.Scan() {
			line := stdout.Text()

			fmt.Fprintf(os.Stdout, "%s\n", line)
		}

		for stderr.Scan() {
			line := stderr.Text()

			fmt.Fprintf(os.Stderr, "%s\n", line)
		}

		if arguments.verbose {
			fmt.Println()
			fmt.Println()
		}
	}()

	cmd.Wait()
}
