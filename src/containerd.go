package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	toml "github.com/pelletier/go-toml"
	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/mod/semver"
)

const (
	RuntimeTypeV1 = "io.containerd.runtime.v1.linux"
	RuntimeTypeV2 = "io.containerd.runc.v1"
	RuntimeBinary = "nvidia-container-runtime"

	DefaultConfig       = "/etc/containerd/config.toml"
	DefaultSocket       = "/run/containerd/containerd.sock"
	DefaultRuntimeClass = "nvidia"
	DefaultSetAsDefault = true

	ReloadBackoff     = 5 * time.Second
	MaxReloadAttempts = 6

	SocketMessageToGetPID = ""
)

var runtimeDirnameArg string
var configFlag string
var socketFlag string
var runtimeClassFlag string
var setAsDefaultFlag bool

func main() {
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
		return Setup(c)
	}

	// Create the 'cleanup' subcommand
	cleanup := cli.Command{}
	cleanup.Name = "cleanup"
	cleanup.Usage = "Trigger any updates made to a containerd config to be undone"
	cleanup.ArgsUsage = "<runtime_dirname>"
	cleanup.Action = func(c *cli.Context) error {
		return Cleanup(c)
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
			Value:       DefaultConfig,
			Destination: &configFlag,
			EnvVars:     []string{"CONTAINERD_CONFIG"},
		},
		&cli.StringFlag{
			Name:        "socket",
			Aliases:     []string{"s"},
			Usage:       "Path to the containerd socket file",
			Value:       DefaultSocket,
			Destination: &socketFlag,
			EnvVars:     []string{"CONTAINERD_SOCKET"},
		},
		&cli.StringFlag{
			Name:        "runtime-class",
			Aliases:     []string{"r"},
			Usage:       "The name of the runtime class to set for the nvidia-container-runtime",
			Value:       DefaultRuntimeClass,
			Destination: &runtimeClassFlag,
			EnvVars:     []string{"CONTAINERD_RUNTIME_CLASS"},
		},
		// The flags below are only used by the 'setup' command.
		&cli.BoolFlag{
			Name:        "set-as-default",
			Aliases:     []string{"d"},
			Usage:       "Set nvidia-container-runtime as the default runtime",
			Value:       DefaultSetAsDefault,
			Destination: &setAsDefaultFlag,
			EnvVars:     []string{"CONTAINERD_SET_AS_DEFAULT"},
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
func Setup(c *cli.Context) error {
	log.Infof("Starting 'setup' for %v", c.App.Name)

	err := ParseArgs(c)
	if err != nil {
		return fmt.Errorf("unable to parse args: %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("unable to load config: %v", err)
	}

	containerdVersion, err := GetContainerdVersion(c.Context)
	if err != nil {
		return fmt.Errorf("unable to get containerd version", err)
	}

	version, err := ParseVersion(config, containerdVersion)
	if err != nil {
		return fmt.Errorf("unable to parse version: %v", err)
	}

	err = UpdateConfig(config, version, containerdVersion)
	if err != nil {
		return fmt.Errorf("unable to update config: %v", err)
	}

	err = FlushConfig(config)
	if err != nil {
		return fmt.Errorf("unable to flush config: %v", err)
	}

	err = SignalContainerd()
	if err != nil {
		return fmt.Errorf("unable to signal containerd: %v", err)
	}

	log.Infof("Completed 'setup' for %v", c.App.Name)

	return nil
}

// Setup reverts a containerd configuration to remove the nvidia-containerd-runtime and reloads it
func Cleanup(c *cli.Context) error {
	log.Infof("Starting 'cleanup' for %v", c.App.Name)

	err := ParseArgs(c)
	if err != nil {
		return fmt.Errorf("unable to parse args: %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("unable to load config: %v", err)
	}

	containerdVersion, err := GetContainerdVersion(c.Context)
	if err != nil {
		return fmt.Errorf("unable to get containerd version", err)
	}

	version, err := ParseVersion(config, containerdVersion)
	if err != nil {
		return fmt.Errorf("unable to parse version: %v", err)
	}

	err = RevertConfig(config, version)
	if err != nil {
		return fmt.Errorf("unable to update config: %v", err)
	}

	err = FlushConfig(config)
	if err != nil {
		return fmt.Errorf("unable to flush config: %v", err)
	}

	err = SignalContainerd()
	if err != nil {
		return fmt.Errorf("unable to signal containerd: %v", err)
	}

	log.Infof("Completed 'cleanup' for %v", c.App.Name)

	return nil
}

// ParseArgs parses the command line arguments to the CLI
func ParseArgs(c *cli.Context) error {
	args := c.Args()

	log.Infof("Parsing arguments: %v", args.Slice())
	if args.Len() != 1 {
		return fmt.Errorf("incorrect number of arguments")
	}
	runtimeDirnameArg = args.Get(0)
	log.Infof("Successfully parsed arguments")

	return nil
}

// LoadConfig loads the containerd config from disk
func LoadConfig() (*toml.Tree, error) {
	log.Infof("Loading config: %v", configFlag)

	info, err := os.Stat(configFlag)
	if os.IsExist(err) && info.IsDir() {
		return nil, fmt.Errorf("config file is a directory")
	}

	configFile := configFlag
	if os.IsNotExist(err) {
		configFile = "/dev/null"
		log.Infof("Config file does not exist, creating new one")
	}

	config, err := toml.LoadFile(configFile)
	if err != nil {
		return nil, err
	}

	log.Infof("Successfully loaded config")

	return config, nil
}

// ParseVersion parses the version field out of the containerd config
func ParseVersion(config *toml.Tree, containerdVersion string) (int, error) {
	var defaultVersion int
	switch semver.Compare(containerdVersion, "v1.3") {
	case -1:
		defaultVersion = 1
	default:
		defaultVersion = 2
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
func UpdateConfig(config *toml.Tree, version int, containerdVersion string) error {
	var err error

	log.Infof("Updating config")
	switch version {
	case 1:
		err = UpdateV1Config(config, containerdVersion)
	case 2:
		err = UpdateV2Config(config)
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
func RevertConfig(config *toml.Tree, version int) error {
	var err error

	log.Infof("Reverting config")
	switch version {
	case 1:
		err = RevertV1Config(config)
	case 2:
		err = RevertV2Config(config)
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
func UpdateV1Config(config *toml.Tree, containerdVersion string) error {
	runtimePath := filepath.Join(runtimeDirnameArg, RuntimeBinary)

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
		runtimeClassFlag,
	}
	runtimeClassOptionsPath := []string{
		"plugins",
		"cri",
		"containerd",
		"runtimes",
		runtimeClassFlag,
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
		config.SetPath(append(runtimeClassPath, "runtime_type"), RuntimeTypeV1)
		config.SetPath(append(runtimeClassPath, "runtime_root"), "")
		config.SetPath(append(runtimeClassPath, "runtime_engine"), "")
		config.SetPath(append(runtimeClassPath, "privileged_without_host_devices"), false)
	}
	config.SetPath(append(runtimeClassOptionsPath, "Runtime"), runtimePath)

	if setAsDefaultFlag {
		if config.GetPath(defaultRuntimePath) == nil {
			config.SetPath(append(defaultRuntimePath, "runtime_type"), RuntimeTypeV1)
			config.SetPath(append(defaultRuntimePath, "runtime_root"), "")
			config.SetPath(append(defaultRuntimePath, "runtime_engine"), "")
			config.SetPath(append(defaultRuntimePath, "privileged_without_host_devices"), false)
		}
		config.SetPath(append(defaultRuntimeOptionsPath, "Runtime"), runtimePath)
		if semver.Compare(containerdVersion, "1.3") >= 0 {
			config.SetPath(defaultRuntimeNamePath, runtimeClassFlag)
		}
	}

	return nil
}

// RevertV1Config performs a revert specific to v1 of the containerd config
func RevertV1Config(config *toml.Tree) error {
	runtimeClassPath := []string{
		"plugins",
		"cri",
		"containerd",
		"runtimes",
		runtimeClassFlag,
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
		if RuntimeBinary == path.Base(runtime) {
			config.DeletePath(append(defaultRuntimeOptionsPath, "Runtime"))
		}
	}

	if defaultRuntimeName, ok := config.GetPath(defaultRuntimeNamePath).(string); ok {
		if RuntimeBinary == defaultRuntimeName {
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
func UpdateV2Config(config *toml.Tree) error {
	runtimePath := filepath.Join(runtimeDirnameArg, RuntimeBinary)

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
		runtimeClassFlag,
	}
	runtimeClassOptionsPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
		"runtimes",
		runtimeClassFlag,
		"options",
	}

	switch runc := config.GetPath(runcPath).(type) {
	case *toml.Tree:
		runc, _ = toml.Load(runc.String())
		config.SetPath(runtimeClassPath, runc)
	default:
		config.SetPath(append(runtimeClassPath, "runtime_type"), RuntimeTypeV2)
		config.SetPath(append(runtimeClassPath, "runtime_root"), "")
		config.SetPath(append(runtimeClassPath, "runtime_engine"), "")
		config.SetPath(append(runtimeClassPath, "privileged_without_host_devices"), false)
	}
	config.SetPath(append(runtimeClassOptionsPath, "BinaryName"), runtimePath)

	if setAsDefaultFlag {
		config.SetPath(append(containerdPath, "default_runtime_name"), runtimeClassFlag)
	}

	return nil
}

// RevertV2Config performs a revert specific to v2 of the containerd config
func RevertV2Config(config *toml.Tree) error {
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
		runtimeClassFlag,
	}

	config.DeletePath(runtimeClassPath)
	if runtime, ok := config.GetPath(append(containerdPath, "default_runtime_name")).(string); ok {
		if runtimeClassFlag == runtime {
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
func FlushConfig(config *toml.Tree) error {
	log.Infof("Flushing config")

	output, err := config.ToTomlString()
	if err != nil {
		return fmt.Errorf("unable to convert to TOML: %v", err)
	}

	switch len(output) {
	case 0:
		err := os.Remove(configFlag)
		if err != nil {
			fmt.Errorf("unable to remove empty file: %v", err)
		}
		log.Infof("Config empty, removing file")
	default:
		f, err := os.Create(configFlag)
		if err != nil {
			return fmt.Errorf("unable to open for writing: %v", configFlag, err)
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

// SignalContainerd sends a SIGHUP signal to the containerd daemon
func SignalContainerd() error {
	log.Infof("Sending SIGHUP signal to containerd")

	// Wrap the logic to perform the SIGHUP in a function so we can retry it on failure
	retriable := func() error {
		conn, err := net.Dial("unix", socketFlag)
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

		_, _, err = conn.(*net.UnixConn).WriteMsgUnix([]byte(SocketMessageToGetPID), nil, nil)
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

	// Try to send a SIGHUP up to MaxReloadAttempts times
	var err error
	for i := 0; i < MaxReloadAttempts; i++ {
		err = retriable()
		if err == nil {
			break
		}
		if i == MaxReloadAttempts-1 {
			break
		}
		log.Warnf("Error signaling containerd, attempt %v/%v: %v", i+1, MaxReloadAttempts, err)
		time.Sleep(ReloadBackoff)
	}
	if err != nil {
		log.Warnf("Max retries reached %v/%v, aborting", MaxReloadAttempts, MaxReloadAttempts)
		return err
	}

	log.Infof("Successfully signaled containerd")

	return nil
}

// GetContainerdVersion returns the version of containerd running
func GetContainerdVersion(ctx context.Context) (string, error) {
	client, err := containerd.New(socketFlag)
	if err != nil {
		return "", err
	}
	defer client.Close()

	version, err := client.Version(ctx)
	if err != nil {
		return "", err
	}

	log.Infof("Containerd version is %s", version.Version)

	return version.Version, nil
}
