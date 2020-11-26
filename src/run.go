package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

const (
	DefaultNoDaemon    = false
	DefaultToolkitArgs = ""
	DefaultRuntime     = "docker"
	DefaultRuntimeArgs = ""
)

var AvailableRuntimes = map[string]struct{}{"docker": {}, "crio": {}, "containerd": {}}

var destinationArg string
var noDaemonFlag bool
var toolkitArgsFlag string
var runtimeFlag string
var runtimeArgsFlag string

func main() {
	// Create the top-level CLI
	c := cli.NewApp()
	c.Name = "nvidia-toolkit"
	c.Usage = "Install the nvidia-container-toolkit for use by a given runtime"
	c.UsageText = "DESTINATION [-n | --no-daemon] [-t | --toolkit-args] [-r | --runtime] [-u | --runtime-args]"
	c.Description = "DESTINATION points to the host path underneath which the nvidia-container-toolkit should be installed.\nIt will be installed at ${DESTINATION}/toolkit"
	c.Version = "0.1.0"
	c.Action = Run

	// Setup flags for the CLI
	c.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:        "no-daemon",
			Aliases:     []string{"n"},
			Usage:       "terminate immediatly after setting up the runtime. Note that no cleanup will be performed",
			Value:       DefaultNoDaemon,
			Destination: &noDaemonFlag,
			EnvVars:     []string{"NO_DAEMON"},
		},
		&cli.StringFlag{
			Name:        "toolkit-args",
			Aliases:     []string{"t"},
			Usage:       "arguments to pass to the underlying 'toolkit' command",
			Value:       DefaultToolkitArgs,
			Destination: &toolkitArgsFlag,
			EnvVars:     []string{"TOOLKIT_ARGS"},
		},
		&cli.StringFlag{
			Name:        "runtime",
			Aliases:     []string{"r"},
			Usage:       "the runtime to setup on this node. One of {'docker', 'crio', 'containerd'}",
			Value:       DefaultRuntime,
			Destination: &runtimeFlag,
			EnvVars:     []string{"RUNTIME"},
		},
		&cli.StringFlag{
			Name:        "runtime-args",
			Aliases:     []string{"u"},
			Usage:       "arguments to pass to 'docker', 'crio', or 'containerd' setup command",
			Value:       DefaultRuntimeArgs,
			Destination: &runtimeArgsFlag,
			EnvVars:     []string{"RUNTIME_ARGS"},
		},
	}

	// Run the CLI
	if err := c.Run(os.Args); err != nil {
		log.Fatal(fmt.Errorf("Error: %v", err))
	}
}

// Run runs the core logic of the CLI
func Run(c *cli.Context) error {
	log.Infof("Starting %v", c.App.Name)

	err := VerifyFlags()
	if err != nil {
		return fmt.Errorf("unable to verify flags: %v", err)
	}

	err = ParseArgs(c)
	if err != nil {
		return fmt.Errorf("unable to parse arguments: %v", err)
	}

	log.Infof("Completed %v", c.App.Name)

	return nil
}

func VerifyFlags() error {
	log.Infof("Verifying Flags")
	if _, exists := AvailableRuntimes[runtimeFlag]; !exists {
		return fmt.Errorf("unknown runtime: %v", runtimeFlag)
	}
	return nil
}

func ParseArgs(c *cli.Context) error {
	args := c.Args()

	log.Infof("Parsing arguments: %v", args.Slice())
	if args.Len() != 1 {
		return fmt.Errorf("incorrect number of arguments")
	}
	destinationArg = args.Get(0)

	return nil
}
