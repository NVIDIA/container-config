/**
# Copyright (c) 2021, NVIDIA CORPORATION.  All rights reserved.
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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

const (
	DefaultHooksDir     = "/usr/share/containers/oci/hooks.d"
	DefaultHookFilename = "oci-nvidia-hook.json"
	CRIOCommandName     = "crio"
)

var hooksDirFlag string
var hookFilenameFlag string
var tooklitDirArg string

func main() {
	// Create the top-level CLI
	c := cli.NewApp()
	c.Name = CRIOCommandName
	c.Usage = "Update cri-o hooks to include the NVIDIA runtime hook"
	c.ArgsUsage = "<toolkit_dirname>"
	c.Version = "0.1.0"

	// Create the 'setup' subcommand
	setup := cli.Command{}
	setup.Name = "setup"
	setup.Usage = "Create the cri-o hook required to run NVIDIA GPU containers"
	setup.ArgsUsage = "<toolkit_dirname>"
	setup.Action = Setup
	setup.Before = ParseArgs

	// Create the 'cleanup' subcommand
	cleanup := cli.Command{}
	cleanup.Name = "cleanup"
	cleanup.Usage = "Remove the NVIDIA cri-o hook"
	cleanup.Action = Cleanup

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
			Name:        "hooks-dir",
			Aliases:     []string{"d"},
			Usage:       "path to the cri-o hooks directory",
			Value:       DefaultHooksDir,
			Destination: &hooksDirFlag,
			EnvVars:     []string{"CRIO_HOOKS_DIR"},
			DefaultText: DefaultHooksDir,
		},
		&cli.StringFlag{
			Name:        "hook-filename",
			Aliases:     []string{"f"},
			Usage:       "filename of the cri-o hook that will be created / removed in the hooks directory",
			Value:       DefaultHookFilename,
			Destination: &hookFilenameFlag,
			EnvVars:     []string{"CRIO_HOOK_FILENAME"},
			DefaultText: DefaultHookFilename,
		},
	}

	// Update the subcommand flags with the common subcommand flags
	setup.Flags = append([]cli.Flag{}, commonFlags...)
	cleanup.Flags = append([]cli.Flag{}, commonFlags...)

	// Run the top-level CLI
	if err := c.Run(os.Args); err != nil {
		log.Fatal(fmt.Errorf("error: %v", err))
	}
}

// Setup installs the prestart hook required to launch GPU-enabled containers
func Setup(c *cli.Context) error {
	log.Infof("Starting 'setup' for %v", c.App.Name)

	err := os.MkdirAll(hooksDirFlag, 0755)
	if err != nil {
		return fmt.Errorf("error creating hooks directory %v: %v", hooksDirFlag, err)
	}

	hookPath := getHookPath(hooksDirFlag, hookFilenameFlag)
	err = createHook(tooklitDirArg, hookPath)
	if err != nil {
		return fmt.Errorf("error creating hook: %v", err)
	}

	return nil
}

// Cleanup removes the specified prestart hook
func Cleanup(c *cli.Context) error {
	log.Infof("Starting 'cleanup' for %v", c.App.Name)

	hookPath := getHookPath(hooksDirFlag, hookFilenameFlag)
	err := os.Remove(hookPath)
	if err != nil {
		return fmt.Errorf("error removing hook '%v': %v", hookPath, err)
	}

	return nil
}

// ParseArgs parses the command line arguments to the CLI
func ParseArgs(c *cli.Context) error {
	args := c.Args()

	log.Infof("Parsing arguments: %v", args.Slice())
	if c.NArg() != 1 {
		return fmt.Errorf("incorrect number of arguments")
	}
	tooklitDirArg = args.Get(0)
	log.Infof("Successfully parsed arguments")

	return nil
}

func createHook(toolkitDir string, hookPath string) error {
	hook, err := os.Create(hookPath)
	if err != nil {
		return fmt.Errorf("error creating hook file '%v': %v", hookPath, err)
	}
	defer hook.Close()

	hookTemplatePath, err := getHookTemplatePath()
	if err != nil {
		return fmt.Errorf("error getting hook template path: %v", err)
	}
	hookTemplate, err := os.Open(hookTemplatePath)
	if err != nil {
		return fmt.Errorf("error opening hook template '%v': %v", hookTemplatePath, err)
	}
	defer hookTemplate.Close()

	scanner := bufio.NewScanner(hookTemplate)
	for scanner.Scan() {
		line := scanner.Text()
		// sed -i "s#@DESTINATION@#${destination}#"
		line = strings.ReplaceAll(line, "@DESTINATION@", toolkitDir)
		_, err := hook.WriteString(line)
		if err != nil {
			return fmt.Errorf("error writing to hook file: %v", err)
		}
	}
	return nil
}

func getHookTemplatePath() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("error determining path of '%v' executable: %v", CRIOCommandName, err)
	}
	baseDir := filepath.Dir(ex)
	return filepath.Join(baseDir, DefaultHookFilename), nil
}

func getHookPath(hooksDir string, hookFilename string) string {
	return filepath.Join(hooksDir, hookFilename)
}
