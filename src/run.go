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
	return nil
}
