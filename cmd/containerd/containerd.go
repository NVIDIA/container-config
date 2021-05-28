/**
# Copyright (c) 2020-2021, NVIDIA CORPORATION.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/plugin"
	toml "github.com/pelletier/go-toml"
	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/mod/semver"
)

const (
	restartModeSignal  = "signal"
	restartModeSystemd = "systemd"
	restartModeNone    = "NONE"

	runtimeBinary = "nvidia-container-runtime"

	defaultConfig        = "/etc/containerd/config.toml"
	defaultSocket        = "/run/containerd/containerd.sock"
	defaultRuntimeClass  = "nvidia"
	defaultRuntmeType    = plugin.RuntimeRuncV2
	defaultSetAsDefault  = true
	defaultRestartMode   = restartModeSignal
	defaultHostRootMount = "/host"

	containerdVersion1dot3 = "v1.3"

	reloadBackoff     = 5 * time.Second
	maxReloadAttempts = 6

	socketMessageToGetPID = ""
)

// containerdVersion allows for methods that allow for better readability under version
// comparisons.
type containerdVersion string

// options stores the configuration from the command line or environment variables
type options struct {
	config        string
	socket        string
	runtimeClass  string
	runtimeType   string
	setAsDefault  bool
	restartMode   string
	hostRootMount string
	runtimeDir    string
}

func main() {
	options := options{}

	// Create the top-level CLI
	c := cli.NewApp()
	c.Name = "containerd"
	c.Usage = "Update a containerd config with the nvidia-container-runtime"
	c.Version = "0.1.0"

	// Create the 'setup' subcommand
	setup := cli.Command{}
	setup.Name = "setup"
	setup.Usage = "Trigger a containerd config to be updated"
	setup.ArgsUsage = "<runtime_dirname>"
	setup.Action = func(c *cli.Context) error {
		return Setup(c, &options)
	}

	// Create the 'cleanup' subcommand
	cleanup := cli.Command{}
	cleanup.Name = "cleanup"
	cleanup.Usage = "Trigger any updates made to a containerd config to be undone"
	cleanup.ArgsUsage = "<runtime_dirname>"
	cleanup.Action = func(c *cli.Context) error {
		return Cleanup(c, &options)
	}

	// Register the subcommands with the top-level CLI
	c.Commands = []*cli.Command{
		&setup,
		&cleanup,
	}

	// Setup common flags across both subcommands. All subcommands get the same
	// set of flags even if they don't use some of them. This is so that we
	// only require the user to specify one set of flags for both 'startup'
	// and 'cleanup' to simplify things.
	commonFlags := []cli.Flag{
		&cli.StringFlag{
			Name:        "config",
			Aliases:     []string{"c"},
			Usage:       "Path to the containerd config file",
			Value:       defaultConfig,
			Destination: &options.config,
			EnvVars:     []string{"CONTAINERD_CONFIG"},
		},
		&cli.StringFlag{
			Name:        "socket",
			Aliases:     []string{"s"},
			Usage:       "Path to the containerd socket file",
			Value:       defaultSocket,
			Destination: &options.socket,
			EnvVars:     []string{"CONTAINERD_SOCKET"},
		},
		&cli.StringFlag{
			Name:        "runtime-class",
			Aliases:     []string{"r"},
			Usage:       "The name of the runtime class to set for the nvidia-container-runtime",
			Value:       defaultRuntimeClass,
			Destination: &options.runtimeClass,
			EnvVars:     []string{"CONTAINERD_RUNTIME_CLASS"},
		},
		&cli.StringFlag{
			Name:        "runtime-type",
			Usage:       "The runtime_type to use for the configured runtime classes",
			Value:       defaultRuntmeType,
			Destination: &options.runtimeType,
			EnvVars:     []string{"CONTAINERD_RUNTIME_TYPE"},
		},
		// The flags below are only used by the 'setup' command.
		&cli.BoolFlag{
			Name:        "set-as-default",
			Aliases:     []string{"d"},
			Usage:       "Set nvidia-container-runtime as the default runtime",
			Value:       defaultSetAsDefault,
			Destination: &options.setAsDefault,
			EnvVars:     []string{"CONTAINERD_SET_AS_DEFAULT"},
			Hidden:      true,
		},
		&cli.StringFlag{
			Name:        "restart-mode",
			Usage:       "Specify how containerd should be restarted; [signal | systemd]",
			Value:       defaultRestartMode,
			Destination: &options.restartMode,
			EnvVars:     []string{"CONTAINERD_RESTART_MODE"},
		},
		&cli.StringFlag{
			Name:        "host-root",
			Usage:       "Specify the path to the host root to be used when restarting containerd using systemd",
			Value:       defaultHostRootMount,
			Destination: &options.hostRootMount,
			EnvVars:     []string{"HOST_ROOT_MOUNT"},
		},
	}

	// Update the subcommand flags with the common subcommand flags
	setup.Flags = append([]cli.Flag{}, commonFlags...)
	cleanup.Flags = append([]cli.Flag{}, commonFlags...)

	// Run the top-level CLI
	if err := c.Run(os.Args); err != nil {
		log.Fatal(fmt.Errorf("Error: %v", err))
	}
}

// Setup updates a containerd configuration to include the nvidia-containerd-runtime and reloads it
func Setup(c *cli.Context, o *options) error {
	log.Infof("Starting 'setup' for %v", c.App.Name)

	runtimeDir, err := ParseArgs(c)
	if err != nil {
		return fmt.Errorf("unable to parse args: %v", err)
	}
	o.runtimeDir = runtimeDir

	cfg, err := LoadConfig(o.config)
	if err != nil {
		return fmt.Errorf("unable to load config: %v", err)
	}

	containerdVersion, err := getContainerdVersion(c.Context, o.socket)
	if err != nil {
		return fmt.Errorf("unable to get containerd version: %v", err)
	}

	version, err := ParseVersion(cfg, containerdVersion)
	if err != nil {
		return fmt.Errorf("unable to parse version: %v", err)
	}

	err = UpdateConfig(cfg, o, version, containerdVersion)
	if err != nil {
		return fmt.Errorf("unable to update config: %v", err)
	}

	err = FlushConfig(o.config, cfg)
	if err != nil {
		return fmt.Errorf("unable to flush config: %v", err)
	}

	err = RestartContainerd(o)
	if err != nil {
		return fmt.Errorf("unable to restart containerd: %v", err)
	}

	log.Infof("Completed 'setup' for %v", c.App.Name)

	return nil
}

// Cleanup reverts a containerd configuration to remove the nvidia-containerd-runtime and reloads it
func Cleanup(c *cli.Context, o *options) error {
	log.Infof("Starting 'cleanup' for %v", c.App.Name)

	_, err := ParseArgs(c)
	if err != nil {
		return fmt.Errorf("unable to parse args: %v", err)
	}

	cfg, err := LoadConfig(o.config)
	if err != nil {
		return fmt.Errorf("unable to load config: %v", err)
	}

	containerdVersion, err := getContainerdVersion(c.Context, o.socket)
	if err != nil {
		return fmt.Errorf("unable to get containerd version: %v", err)
	}

	version, err := ParseVersion(cfg, containerdVersion)
	if err != nil {
		return fmt.Errorf("unable to parse version: %v", err)
	}

	err = RevertConfig(cfg, o, version)
	if err != nil {
		return fmt.Errorf("unable to update config: %v", err)
	}

	err = FlushConfig(o.config, cfg)
	if err != nil {
		return fmt.Errorf("unable to flush config: %v", err)
	}

	err = RestartContainerd(o)
	if err != nil {
		return fmt.Errorf("unable to restart containerd: %v", err)
	}

	log.Infof("Completed 'cleanup' for %v", c.App.Name)

	return nil
}

// ParseArgs parses the command line arguments to the CLI
func ParseArgs(c *cli.Context) (string, error) {
	args := c.Args()

	log.Infof("Parsing arguments: %v", args.Slice())
	if args.Len() != 1 {
		return "", fmt.Errorf("incorrect number of arguments")
	}
	runtimeDir := args.Get(0)
	log.Infof("Successfully parsed arguments")

	return runtimeDir, nil
}

// LoadConfig loads the containerd config from disk
func LoadConfig(config string) (*toml.Tree, error) {
	log.Infof("Loading config: %v", config)

	info, err := os.Stat(config)
	if os.IsExist(err) && info.IsDir() {
		return nil, fmt.Errorf("config file is a directory")
	}

	configFile := config
	if os.IsNotExist(err) {
		configFile = "/dev/null"
		log.Infof("Config file does not exist, creating new one")
	}

	cfg, err := toml.LoadFile(configFile)
	if err != nil {
		return nil, err
	}

	log.Infof("Successfully loaded config")

	return cfg, nil
}

// ParseVersion parses the version field out of the containerd config
func ParseVersion(config *toml.Tree, containerdVersion containerdVersion) (int, error) {
	var defaultVersion int
	if containerdVersion.atLeast(containerdVersion1dot3) {
		defaultVersion = 2
	} else {
		defaultVersion = 1
	}

	var version int
	switch v := config.Get("version").(type) {
	case nil:
		switch len(config.Keys()) {
		case 0: // No config exists, or the config file is empty, use version inferred from containerd
			version = defaultVersion
		default: // A config file exists, has content, and no version is set
			version = 1
		}
	case int64:
		version = int(v)
	default:
		return -1, fmt.Errorf("unsupported type for version field: %v", v)
	}
	log.Infof("Config version: %v", version)

	return version, nil
}

// UpdateConfig updates the containerd config to include the nvidia-container-runtime
func UpdateConfig(config *toml.Tree, o *options, version int, containerdVersion containerdVersion) error {
	var err error

	log.Infof("Updating config")
	switch version {
	case 1:
		err = UpdateV1Config(config, o, containerdVersion)
	case 2:
		err = UpdateV2Config(config, o)
	default:
		err = fmt.Errorf("unsupported containerd config version: %v", version)
	}
	if err != nil {
		return err
	}
	log.Infof("Successfully updated config")

	return nil
}

// RevertConfig reverts the containerd config to remove the nvidia-container-runtime
func RevertConfig(config *toml.Tree, o *options, version int) error {
	var err error

	log.Infof("Reverting config")
	switch version {
	case 1:
		err = RevertV1Config(config, o)
	case 2:
		err = RevertV2Config(config, o)
	default:
		err = fmt.Errorf("unsupported containerd config version: %v", version)
	}
	if err != nil {
		return err
	}
	log.Infof("Successfully reverted config")

	return nil
}

// UpdateV1Config performs an update specific to v1 of the containerd config
func UpdateV1Config(config *toml.Tree, o *options, containerdVersion containerdVersion) error {
	runtimePath := filepath.Join(o.runtimeDir, runtimeBinary)

	// We ensure that the version is set to 1. This handles the case where the config was empty and
	// the config version was determined from the containerd version.
	config.Set("version", int64(1))

	runcPath := []string{
		"plugins",
		"cri",
		"containerd",
		"runtimes",
		"runc",
	}
	runtimeClassPath := []string{
		"plugins",
		"cri",
		"containerd",
		"runtimes",
		o.runtimeClass,
	}
	runtimeClassOptionsPath := []string{
		"plugins",
		"cri",
		"containerd",
		"runtimes",
		o.runtimeClass,
		"options",
	}
	defaultRuntimePath := []string{
		"plugins",
		"cri",
		"containerd",
		"default_runtime",
	}
	defaultRuntimeOptionsPath := []string{
		"plugins",
		"cri",
		"containerd",
		"default_runtime",
		"options",
	}
	defaultRuntimeNamePath := []string{
		"plugins",
		"cri",
		"containerd",
		"default_runtime_name",
	}

	switch runc := config.GetPath(runcPath).(type) {
	case *toml.Tree:
		runc, _ = toml.Load(runc.String())
		config.SetPath(runtimeClassPath, runc)
	default:
		config.SetPath(append(runtimeClassPath, "runtime_type"), o.runtimeType)
		config.SetPath(append(runtimeClassPath, "runtime_root"), "")
		config.SetPath(append(runtimeClassPath, "runtime_engine"), "")
		config.SetPath(append(runtimeClassPath, "privileged_without_host_devices"), false)
	}
	config.SetPath(append(runtimeClassOptionsPath, "Runtime"), runtimePath)

	if !o.setAsDefault {
		return nil
	}

	if containerdVersion.atLeast(containerdVersion1dot3) {
		config.SetPath(defaultRuntimeNamePath, o.runtimeClass)
		if config.GetPath(defaultRuntimePath) != nil {
			log.Warnf("The setting of default_runtime (%v) in containerd is deprecated", defaultRuntimePath)
		}
		return nil
	}

	log.Warnf("Support for containerd version %v is deprecated", containerdVersion1dot3)
	if config.GetPath(defaultRuntimePath) == nil {
		config.SetPath(append(defaultRuntimePath, "runtime_type"), o.runtimeType)
		config.SetPath(append(defaultRuntimePath, "runtime_root"), "")
		config.SetPath(append(defaultRuntimePath, "runtime_engine"), "")
		config.SetPath(append(defaultRuntimePath, "privileged_without_host_devices"), false)
	}
	config.SetPath(append(defaultRuntimeOptionsPath, "Runtime"), runtimePath)

	return nil
}

// RevertV1Config performs a revert specific to v1 of the containerd config
func RevertV1Config(config *toml.Tree, o *options) error {
	runtimeClassPath := []string{
		"plugins",
		"cri",
		"containerd",
		"runtimes",
		o.runtimeClass,
	}
	defaultRuntimePath := []string{
		"plugins",
		"cri",
		"containerd",
		"default_runtime",
	}
	defaultRuntimeOptionsPath := []string{
		"plugins",
		"cri",
		"containerd",
		"default_runtime",
		"options",
	}
	defaultRuntimeNamePath := []string{
		"plugins",
		"cri",
		"containerd",
		"default_runtime_name",
	}

	config.DeletePath(runtimeClassPath)
	if runtime, ok := config.GetPath(append(defaultRuntimeOptionsPath, "Runtime")).(string); ok {
		if runtimeBinary == path.Base(runtime) {
			config.DeletePath(append(defaultRuntimeOptionsPath, "Runtime"))
		}
	}

	if defaultRuntimeName, ok := config.GetPath(defaultRuntimeNamePath).(string); ok {
		if o.runtimeClass == defaultRuntimeName {
			config.DeletePath(defaultRuntimeNamePath)
		}
	}

	for i := 0; i < len(runtimeClassPath); i++ {
		if runtimes, ok := config.GetPath(runtimeClassPath[:len(runtimeClassPath)-i]).(*toml.Tree); ok {
			if len(runtimes.Keys()) == 0 {
				config.DeletePath(runtimeClassPath[:len(runtimeClassPath)-i])
			}
		}
	}

	if options, ok := config.GetPath(defaultRuntimeOptionsPath).(*toml.Tree); ok {
		if len(options.Keys()) == 0 {
			config.DeletePath(defaultRuntimeOptionsPath)
		}
	}

	if runtime, ok := config.GetPath(defaultRuntimePath).(*toml.Tree); ok {
		fields := []string{"runtime_type", "runtime_root", "runtime_engine", "privileged_without_host_devices"}
		if len(runtime.Keys()) <= len(fields) {
			matches := []string{}
			for _, f := range fields {
				e := runtime.Get(f)
				if e != nil {
					matches = append(matches, f)
				}
			}
			if len(matches) == len(runtime.Keys()) {
				for _, m := range matches {
					runtime.Delete(m)
				}
			}
		}
	}

	for i := 0; i < len(defaultRuntimePath); i++ {
		if runtimes, ok := config.GetPath(defaultRuntimePath[:len(defaultRuntimePath)-i]).(*toml.Tree); ok {
			if len(runtimes.Keys()) == 0 {
				config.DeletePath(defaultRuntimePath[:len(defaultRuntimePath)-i])
			}
		}
	}

	if len(config.Keys()) == 1 && config.Keys()[0] == "version" {
		config.Delete("version")
	}

	return nil
}

// UpdateV2Config performs an update specific to v2 of the containerd config
func UpdateV2Config(config *toml.Tree, o *options) error {
	runtimePath := filepath.Join(o.runtimeDir, runtimeBinary)

	// We ensure that the version is set to 2. This handles the case where the config was empty and
	// the config version was determined from the containerd version.
	config.Set("version", int64(2))

	containerdPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
	}
	runcPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
		"runtimes",
		"runc",
	}
	runtimeClassPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
		"runtimes",
		o.runtimeClass,
	}
	runtimeClassOptionsPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
		"runtimes",
		o.runtimeClass,
		"options",
	}

	switch runc := config.GetPath(runcPath).(type) {
	case *toml.Tree:
		runc, _ = toml.Load(runc.String())
		config.SetPath(runtimeClassPath, runc)
	default:
		config.SetPath(append(runtimeClassPath, "runtime_type"), o.runtimeType)
		config.SetPath(append(runtimeClassPath, "runtime_root"), "")
		config.SetPath(append(runtimeClassPath, "runtime_engine"), "")
		config.SetPath(append(runtimeClassPath, "privileged_without_host_devices"), false)
	}
	config.SetPath(append(runtimeClassOptionsPath, "BinaryName"), runtimePath)

	if o.setAsDefault {
		config.SetPath(append(containerdPath, "default_runtime_name"), o.runtimeClass)
	}

	return nil
}

// RevertV2Config performs a revert specific to v2 of the containerd config
func RevertV2Config(config *toml.Tree, o *options) error {
	containerdPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
	}
	runtimeClassPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
		"runtimes",
		o.runtimeClass,
	}

	config.DeletePath(runtimeClassPath)
	if runtime, ok := config.GetPath(append(containerdPath, "default_runtime_name")).(string); ok {
		if o.runtimeClass == runtime {
			config.DeletePath(append(containerdPath, "default_runtime_name"))
		}
	}

	for i := 0; i < len(runtimeClassPath); i++ {
		if runtimes, ok := config.GetPath(runtimeClassPath[:len(runtimeClassPath)-i]).(*toml.Tree); ok {
			if len(runtimes.Keys()) == 0 {
				config.DeletePath(runtimeClassPath[:len(runtimeClassPath)-i])
			}
		}
	}

	if len(config.Keys()) == 1 && config.Keys()[0] == "version" {
		config.Delete("version")
	}

	return nil
}

// FlushConfig flushes the updated/reverted config out to disk
func FlushConfig(config string, cfg *toml.Tree) error {
	log.Infof("Flushing config")

	output, err := cfg.ToTomlString()
	if err != nil {
		return fmt.Errorf("unable to convert to TOML: %v", err)
	}

	switch len(output) {
	case 0:
		err := os.Remove(config)
		if err != nil {
			return fmt.Errorf("unable to remove empty file: %v", err)
		}
		log.Infof("Config empty, removing file")
	default:
		f, err := os.Create(config)
		if err != nil {
			return fmt.Errorf("unable to open '%v' for writing: %v", config, err)
		}
		defer f.Close()

		_, err = f.WriteString(output)
		if err != nil {
			return fmt.Errorf("unable to write output: %v", err)
		}
	}

	log.Infof("Successfully flushed config")

	return nil
}

// RestartContainerd restarts containerd depending on the value of restartModeFlag
func RestartContainerd(o *options) error {
	switch o.restartMode {
	case restartModeNone:
		log.Warnf("Skipping sending signal to containerd due to --restart-mode=%v", o.restartMode)
		return nil
	case restartModeSignal:
		err := SignalContainerd(o)
		if err != nil {
			return fmt.Errorf("unable to signal containerd: %v", err)
		}
	case restartModeSystemd:
		return RestartContainerdSystemd(o.hostRootMount)
	default:
		return fmt.Errorf("Invalid restart mode specified: %v", o.restartMode)
	}

	return nil
}

// SignalContainerd sends a SIGHUP signal to the containerd daemon
func SignalContainerd(o *options) error {
	log.Infof("Sending SIGHUP signal to containerd")

	// Wrap the logic to perform the SIGHUP in a function so we can retry it on failure
	retriable := func() error {
		conn, err := net.Dial("unix", o.socket)
		if err != nil {
			return fmt.Errorf("unable to dial: %v", err)
		}
		defer conn.Close()

		sconn, err := conn.(*net.UnixConn).SyscallConn()
		if err != nil {
			return fmt.Errorf("unable to get syscall connection: %v", err)
		}

		err1 := sconn.Control(func(fd uintptr) {
			err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_PASSCRED, 1)
		})
		if err1 != nil {
			return fmt.Errorf("unable to issue call on socket fd: %v", err1)
		}
		if err != nil {
			return fmt.Errorf("unable to SetsockoptInt on socket fd: %v", err)
		}

		_, _, err = conn.(*net.UnixConn).WriteMsgUnix([]byte(socketMessageToGetPID), nil, nil)
		if err != nil {
			return fmt.Errorf("unable to WriteMsgUnix on socket fd: %v", err)
		}

		oob := make([]byte, 1024)
		_, oobn, _, _, err := conn.(*net.UnixConn).ReadMsgUnix(nil, oob)
		if err != nil {
			return fmt.Errorf("unable to ReadMsgUnix on socket fd: %v", err)
		}

		oob = oob[:oobn]
		scm, err := syscall.ParseSocketControlMessage(oob)
		if err != nil {
			return fmt.Errorf("unable to ParseSocketControlMessage from message received on socket fd: %v", err)
		}

		ucred, err := syscall.ParseUnixCredentials(&scm[0])
		if err != nil {
			return fmt.Errorf("unable to ParseUnixCredentials from message received on socket fd: %v", err)
		}

		err = syscall.Kill(int(ucred.Pid), syscall.SIGHUP)
		if err != nil {
			return fmt.Errorf("unable to send SIGHUP to 'containerd' process: %v", err)
		}

		return nil
	}

	// Try to send a SIGHUP up to maxReloadAttempts times
	var err error
	for i := 0; i < maxReloadAttempts; i++ {
		err = retriable()
		if err == nil {
			break
		}
		if i == maxReloadAttempts-1 {
			break
		}
		log.Warnf("Error signaling containerd, attempt %v/%v: %v", i+1, maxReloadAttempts, err)
		time.Sleep(reloadBackoff)
	}
	if err != nil {
		log.Warnf("Max retries reached %v/%v, aborting", maxReloadAttempts, maxReloadAttempts)
		return err
	}

	log.Infof("Successfully signaled containerd")

	return nil
}

// RestartContainerdSystemd restarts containerd using systemctl
func RestartContainerdSystemd(hostRootMount string) error {
	log.Infof("Restarting containerd using systemd and host root mounted at %v", hostRootMount)

	command := "chroot"
	args := []string{hostRootMount, "systemctl", "restart", "containerd"}

	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error restarting containerd using systemd: %v", err)
	}

	return nil
}

// getContainerdVersion returns the version of containerd running
func getContainerdVersion(ctx context.Context, socket string) (containerdVersion, error) {
	client, err := containerd.New(socket)
	if err != nil {
		return "", err
	}
	defer client.Close()

	version, err := client.Version(ctx)
	if err != nil {
		return "", err
	}

	containerdVersion, err := newContainerdVersion(version.Version)
	if err != nil {
		return "", fmt.Errorf("error retrieving containerd version: %v", containerdVersion)
	}

	log.Infof("Containerd version is %v", containerdVersion)
	return containerdVersion, nil
}

// newContainerdVersion creates a containerdVersion from the specified version string.
func newContainerdVersion(version string) (containerdVersion, error) {
	if semver.IsValid(version) {
		return containerdVersion(version), nil
	}

	if version != "" && version[0] != 'v' && semver.IsValid("v"+version) {
		return containerdVersion("v" + version), nil
	}

	return "", fmt.Errorf("%v is an invalid semantic version", version)
}

func (v containerdVersion) atLeast(version string) bool {
	return semver.Compare(string(v), version) >= 0
}
