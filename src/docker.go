package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

const (
	RuntimeName   = "nvidia"
	RuntimeBinary = "nvidia-container-runtime"

	DefaultConfig       = "/etc/docker/daemon.json"
	DefaultSocket       = "/var/run/docker.sock"
	DefaultSetAsDefault = true

	ReloadBackoff     = 5 * time.Second
	MaxReloadAttempts = 6

	DefaultDockerRuntime  = "runc"
	SocketMessageToGetPID = "GET /info HTTP/1.0\r\n\r\n"
)

var runtimeDirnameArg string
var configFlag string
var socketFlag string
var setAsDefaultFlag bool

func main() {
	// Create the top-level CLI
	c := cli.NewApp()
	c.Name = "docker"
	c.Usage = "Update docker config with the nvidia runtime"
	c.Version = "0.1.0"

	// Create the 'setup' subcommand
	setup := cli.Command{}
	setup.Name = "setup"
	setup.Usage = "Trigger docker config to be updated"
	setup.ArgsUsage = "<runtime_dirname>"
	setup.Action = func(c *cli.Context) error {
		return Setup(c)
	}

	// Create the 'cleanup' subcommand
	cleanup := cli.Command{}
	cleanup.Name = "cleanup"
	cleanup.Usage = "Trigger any updates made to docker config to be undone"
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
			Usage:       "Path to docker config file",
			Value:       DefaultConfig,
			Destination: &configFlag,
			EnvVars:     []string{"DOCKER_CONFIG"},
		},
		&cli.StringFlag{
			Name:        "socket",
			Aliases:     []string{"s"},
			Usage:       "Path to the docker socket file",
			Value:       DefaultSocket,
			Destination: &socketFlag,
			EnvVars:     []string{"DOCKER_SOCKET"},
		},
		// The flags below are only used by the 'setup' command.
		&cli.BoolFlag{
			Name:        "set-as-default",
			Aliases:     []string{"d"},
			Usage:       "Set nvidia as the default runtime",
			Value:       DefaultSetAsDefault,
			Destination: &setAsDefaultFlag,
			EnvVars:     []string{"DOCKER_SET_AS_DEFAULT"},
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

// Setup updates docker configuration to include the nvidia runtime and reloads it
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

	err = UpdateConfig(config)
	if err != nil {
		return fmt.Errorf("unable to update config: %v", err)
	}

	err = FlushConfig(config)
	if err != nil {
		return fmt.Errorf("unable to flush config: %v", err)
	}

	err = SignalDocker()
	if err != nil {
		return fmt.Errorf("unable to signal docker: %v", err)
	}

	log.Infof("Completed 'setup' for %v", c.App.Name)

	return nil
}

// Setup reverts docker configuration to remove the nvidia runtime and reloads it
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

	err = RevertConfig(config)
	if err != nil {
		return fmt.Errorf("unable to update config: %v", err)
	}

	err = FlushConfig(config)
	if err != nil {
		return fmt.Errorf("unable to flush config: %v", err)
	}

	err = SignalDocker()
	if err != nil {
		return fmt.Errorf("unable to signal docker: %v", err)
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

// LoadConfig loads the docker config from disk
func LoadConfig() (map[string]interface{}, error) {
	log.Infof("Loading config: %v", configFlag)

	info, err := os.Stat(configFlag)
	if os.IsExist(err) && info.IsDir() {
		return nil, fmt.Errorf("config file is a directory")
	}

	config := make(map[string]interface{})

	if os.IsNotExist(err) {
		log.Infof("Config file does not exist, creating new one")
		return config, nil
	}

	readBytes, err := ioutil.ReadFile(configFlag)
	if err != nil {
		return nil, fmt.Errorf("unable to read config: %v", err)
	}

	reader := bytes.NewReader(readBytes)
	if err := json.NewDecoder(reader).Decode(&config); err != nil {
		return nil, err
	}

	log.Infof("Successfully loaded config")
	return config, nil
}

// UpdateConfig updates the docker config to include the nvidia runtime
func UpdateConfig(config map[string]interface{}) error {
	runtimePath := filepath.Join(runtimeDirnameArg, RuntimeBinary)

	if setAsDefaultFlag {
		config["default-runtime"] = RuntimeName
	}

	runtimes := make(map[string]interface{})
	if _, exists := config["runtimes"]; exists {
		runtimes = config["runtimes"].(map[string]interface{})
	}
	runtimes[RuntimeName] = map[string]interface{}{"path": runtimePath, "args": []string{}}

	config["runtimes"] = runtimes
	return nil
}

//RevertConfig reverts the docker config to remove the nvidia runtime
func RevertConfig(config map[string]interface{}) error {
	if _, exists := config["default-runtime"]; exists {
		if config["default-runtime"] == RuntimeName {
			config["default-runtime"] = DefaultDockerRuntime
		}
	}

	if _, exists := config["runtimes"]; exists {
		runtimes := config["runtimes"].(map[string]interface{})
		delete(runtimes, RuntimeName)

		if len(runtimes) == 0 {
			delete(config, "runtimes")
		}
	}
	return nil
}

// FlushConfig flushes the updated/reverted config out to disk
func FlushConfig(config map[string]interface{}) error {
	log.Infof("Flushing config")

	output, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("unable to convert to JSON: %v", err)
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

		_, err = f.WriteString(string(output))
		if err != nil {
			return fmt.Errorf("unable to write output: %v", err)
		}
	}

	log.Infof("Successfully flushed config")

	return nil
}

// SignalDocker sends a SIGHUP signal to docker daemon
func SignalDocker() error {
	log.Infof("Sending SIGHUP signal to docker")

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
			return fmt.Errorf("unable to send SIGHUP to 'docker' process: %v", err)
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
		log.Warnf("Error signaling docker, attempt %v/%v: %v", i+1, MaxReloadAttempts, err)
		time.Sleep(ReloadBackoff)
	}
	if err != nil {
		log.Warnf("Max retries reached %v/%v, aborting", MaxReloadAttempts, MaxReloadAttempts)
		return err
	}

	log.Infof("Successfully signaled docker")

	return nil
}
