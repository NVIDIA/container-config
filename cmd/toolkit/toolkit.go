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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	// DefaultNvidiaDriverRoot specifies the default NVIDIA driver run directory
	DefaultNvidiaDriverRoot = "/run/nvidia/driver"

	nvidiaContainerCliSource         = "/usr/bin/nvidia-container-cli"
	nvidiaContainerRuntimeHookSource = "/usr/bin/nvidia-container-toolkit"
	nvidiaContainerRuntimeSource     = "/usr/bin/nvidia-container-runtime"

	nvidiaContainerToolkitConfigSource = "/etc/nvidia-container-runtime/config.toml"
	configFilename                     = "config.toml"
)

var toolkitDirArg string
var nvidiaDriverRootFlag string

func main() {
	// Create the top-level CLI
	c := cli.NewApp()
	c.Name = "toolkit"
	c.Usage = "Manage the NVIDIA container toolkit"
	c.Version = "0.1.0"

	// Create the 'install' subcommand
	install := cli.Command{}
	install.Name = "install"
	install.Usage = "Install the components of the NVIDIA container toolkit"
	install.ArgsUsage = "<toolkit_directory>"
	install.Before = parseArgs
	install.Action = Install

	// Create the 'delete' command
	delete := cli.Command{}
	delete.Name = "delete"
	delete.Usage = "Delete the NVIDIA container toolkit"
	delete.ArgsUsage = "<toolkit_directory>"
	delete.Before = parseArgs
	delete.Action = Delete

	// Register the subcommand with the top-level CLI
	c.Commands = []*cli.Command{
		&install,
		&delete,
	}

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:        "nvidia-driver-root",
			Value:       DefaultNvidiaDriverRoot,
			Destination: &nvidiaDriverRootFlag,
			EnvVars:     []string{"NVIDIA_DRIVER_ROOT"},
		},
	}

	// Update the subcommand flags with the common subcommand flags
	install.Flags = append([]cli.Flag{}, flags...)

	// Run the top-level CLI
	if err := c.Run(os.Args); err != nil {
		log.Fatal(fmt.Errorf("error: %v", err))
	}
}

// parseArgs parses the command line arguments to the CLI
func parseArgs(c *cli.Context) error {
	args := c.Args()

	log.Infof("Parsing arguments: %v", args.Slice())
	if c.NArg() != 1 {
		return fmt.Errorf("incorrect number of arguments")
	}
	toolkitDirArg = args.Get(0)
	log.Infof("Successfully parsed arguments")

	return nil
}

// Delete removes the NVIDIA container toolkit
func Delete(cli *cli.Context) error {
	log.Infof("Deleting NVIDIA container toolkit from '%v'", toolkitDirArg)
	err := os.RemoveAll(toolkitDirArg)
	if err != nil {
		return fmt.Errorf("error deleting toolkit directory: %v", err)
	}
	return nil
}

// Install installs the components of the NVIDIA container toolkit.
// Any existing installation is removed.
func Install(cli *cli.Context) error {
	log.Infof("Installing NVIDIA container toolkit to '%v'", toolkitDirArg)

	log.Infof("Removing existing NVIDIA container toolkit installation")
	err := os.RemoveAll(toolkitDirArg)
	if err != nil {
		return fmt.Errorf("error removing toolkit directory: %v", err)
	}

	toolkitConfigDir := filepath.Join(toolkitDirArg, ".config", "nvidia-container-runtime")
	toolkitConfigPath := filepath.Join(toolkitConfigDir, configFilename)

	err = createDirectories(toolkitDirArg, toolkitConfigDir)
	if err != nil {
		return fmt.Errorf("could not create required directories: %v", err)
	}

	err = installContainerLibrary(toolkitDirArg)
	if err != nil {
		return fmt.Errorf("error installing NVIDIA container library: %v", err)
	}

	_, err = installContainerRuntime(toolkitDirArg)
	if err != nil {
		return fmt.Errorf("error installing NVIDIA container runtime: %v", err)
	}

	nvidiaContainerCliExecutable, err := installContainerCLI(toolkitDirArg)
	if err != nil {
		return fmt.Errorf("error installing NVIDIA container CLI: %v", err)
	}

	_, err = installRuntimeHook(toolkitDirArg, toolkitConfigPath)
	if err != nil {
		return fmt.Errorf("error installing NVIDIA container runtime hook: %v", err)
	}

	err = installToolkitConfig(toolkitConfigPath, nvidiaDriverRootFlag, nvidiaContainerCliExecutable)
	if err != nil {
		return fmt.Errorf("error installing NVIDIA container toolkit config: %v", err)
	}

	return nil
}

// installContainerLibrary locates and installs the libnvidia-container.so.1 library.
// A predefined set of library candidates are considered, with the first one
// resulting in success being installed to the toolkit folder. The install process
// resolves the symlink for the library and copies the versioned library itself.
func installContainerLibrary(toolkitDir string) error {
	log.Infof("Installing NVIDIA container library to '%v'", toolkitDir)

	candidates := []string{
		"/usr/lib64/libnvidia-container.so.1",
		"/usr/lib/x86_64-linux-gnu/libnvidia-container.so.1",
	}
	for _, l := range candidates {
		log.Infof("Checking library candidate '%v'", l)

		libraryCandidate, err := resolveLink(l)
		if err != nil {
			log.Infof("Skipping library candidate '%v': %v", l, err)
			continue
		}

		installedLibPath, err := installFileToFolder(toolkitDir, libraryCandidate)
		if err != nil {
			log.Infof("Skipping library candidate '%v': %v", l, err)
			continue
		}
		log.Infof("Installed '%v' to '%v'", l, installedLibPath)

		const libName = "libnvidia-container.so.1"
		if filepath.Base(installedLibPath) == libName {
			return nil
		}

		err = installSymlink(toolkitDir, libName, installedLibPath)
		if err != nil {
			return fmt.Errorf("error installing symlink for NVIDIA container library: %v", err)
		}
		return nil
	}

	return fmt.Errorf("error locating NVIDIA container library")
}

// installToolkitConfig installs the config file for the NVIDIA container toolkit ensuring
// that the settings are updated to match the desired install and nvidia driver directories.
func installToolkitConfig(toolkitConfigPath string, nvidiaDriverDir string, nvidiaContainerCliExecutablePath string) error {
	log.Infof("Installing NVIDIA container toolkit config '%v'", toolkitConfigPath)

	config, err := toml.LoadFile(nvidiaContainerToolkitConfigSource)
	if err != nil {
		return fmt.Errorf("could not open source config file: %v", err)
	}

	targetConfig, err := os.Create(toolkitConfigPath)
	if err != nil {
		return fmt.Errorf("could not create target config file: %v", err)
	}
	defer targetConfig.Close()

	nvidiaContainerCliKey := func(p string) []string {
		return []string{"nvidia-container-cli", p}
	}

	// Read the ldconfig path from the config as this may differ per platform
	// On ubuntu-based systems this ends in `.real`
	ldconfigPath := fmt.Sprintf("%s", config.GetPath(nvidiaContainerCliKey("ldconfig")))

	// Use the driver run root as the root:
	driverLdconfigPath := "@" + filepath.Join(nvidiaDriverDir, strings.TrimPrefix(ldconfigPath, "@/"))

	config.SetPath(nvidiaContainerCliKey("root"), nvidiaDriverDir)
	config.SetPath(nvidiaContainerCliKey("path"), nvidiaContainerCliExecutablePath)
	config.SetPath(nvidiaContainerCliKey("ldconfig"), driverLdconfigPath)

	_, err = config.WriteTo(targetConfig)
	if err != nil {
		return fmt.Errorf("error writing config: %v", err)
	}
	return nil
}

// installContainerRuntime sets up the NVIDIA container runtime, copying the executable
// and implementing the required wrapper
func installContainerRuntime(toolkitDir string) (string, error) {
	log.Infof("Installing NVIDIA container runtime from '%v'", nvidiaContainerRuntimeSource)

	preLines := []string{
		"",
		"cat /proc/modules | grep -e \"^nvidia \" >/dev/null 2>&1",
		"if [ \"${?}\" != \"0\" ]; then",
		"	echo \"nvidia driver modules are not yet loaded, invoking runc directly\"",
		"	exec runc \"$@\"",
		"fi",
		"",
	}
	env := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(toolkitDir, ".config"),
	}
	installedPath, err := installExecutable(toolkitDir, nvidiaContainerRuntimeSource, env, preLines, nil)
	if err != nil {
		return "", fmt.Errorf("error installing NVIDIA container runtime: %v", err)
	}
	return installedPath, nil
}

// installContainerCLI sets up the NVIDIA container CLI executable, copying the executable
// and implementing the required wrapper
func installContainerCLI(toolkitDir string) (string, error) {
	log.Infof("Installing NVIDIA container CLI from '%v'", nvidiaContainerCliSource)

	env := map[string]string{
		"LD_LIBRARY_PATH": toolkitDir,
	}
	installedPath, err := installExecutable(toolkitDir, nvidiaContainerCliSource, env, nil, nil)
	if err != nil {
		return "", fmt.Errorf("error installing NVIDIA container CLI: %v", err)
	}
	return installedPath, nil
}

// installRuntimeHook sets up the NVIDIA runtime hook, copying the executable
// and implementing the required wrapper
func installRuntimeHook(toolkitDir string, configFilePath string) (string, error) {
	log.Infof("Installing NVIDIA container runtime hook from '%v'", nvidiaContainerRuntimeHookSource)

	env := map[string]string{}
	argLines := []string{
		fmt.Sprintf("-config \"%s\"", configFilePath),
	}
	installedPath, err := installExecutable(toolkitDir, nvidiaContainerRuntimeHookSource, env, nil, argLines)
	if err != nil {
		return "", fmt.Errorf("error installing NVIDIA container runtime hook: %v", err)
	}

	err = installSymlink(toolkitDir, "nvidia-container-runtime-hook", installedPath)
	if err != nil {
		return "", fmt.Errorf("error installing symlink to NVIDIA container runtime hook: %v", err)
	}

	return installedPath, nil
}

// installExecutable installs an executable component of the NVIDIA container toolkit. The source executable
// is copied to a `.real` file and a wapper is created to set up the environment as required.
func installExecutable(toolkitDir string, sourceExecutable string, env map[string]string, preLines []string, argLines []string) (string, error) {
	log.Infof("Installing executable '%v'", sourceExecutable)

	dotRealFilename, err := installDotRealFile(toolkitDir, sourceExecutable)
	if err != nil {
		return "", fmt.Errorf("error installing .real file: %v", err)
	}
	log.Infof("Created '%v'", dotRealFilename)

	wrapperFilename, err := wrapExecutable(toolkitDir, sourceExecutable, dotRealFilename, env, preLines, argLines)
	if err != nil {
		return "", fmt.Errorf("error wrapping '%v': %v", dotRealFilename, err)
	}
	log.Infof("Created wrapper '%v'", wrapperFilename)

	return wrapperFilename, nil
}

func installDotRealFile(destFolder string, sourceExecutable string) (string, error) {
	executableDotReal := filepath.Base(sourceExecutable) + ".real"
	return installFileToFolderWithName(destFolder, executableDotReal, sourceExecutable)
}

func wrapExecutable(destFolder, executable string, dotRealFilename string, env map[string]string, preLines []string, argLines []string) (string, error) {
	wrapperPath := getInstalledPath(destFolder, executable)
	wrapper, err := os.Create(wrapperPath)
	if err != nil {
		return "", fmt.Errorf("error creating executable wrapper: %v", err)
	}
	defer wrapper.Close()

	// Add the shebang
	fmt.Fprintln(wrapper, "#! /bin/sh")

	// Add the preceding lines if any
	for _, line := range preLines {
		fmt.Fprintf(wrapper, "%s\n", line)
	}

	// Update the path to include the destination folder
	path, specified := env["PATH"]
	if !specified {
		path = "$PATH"
	}
	env["PATH"] = strings.Join([]string{destFolder, path}, ":")

	for e, v := range env {
		fmt.Fprintf(wrapper, "%s=%s \\\n", e, v)
	}
	// Add the call to the target executable
	fmt.Fprintf(wrapper, "%s \\\n", dotRealFilename)

	// Insert additional lines in the `arg` list
	for _, line := range argLines {
		fmt.Fprintf(wrapper, "\t%s \\\n", line)
	}
	// Add the script arguments "$@"
	fmt.Fprintln(wrapper, "\t\"$@\"")

	err = ensureExecutable(wrapperPath)
	if err != nil {
		return "", fmt.Errorf("error making wrapper executable: %v", err)
	}

	return wrapperPath, nil
}

// installSymlink creates a symlink in the toolkitDirectory that points to the specified target.
// Note: The target is assumed to be local to the toolkit directory
func installSymlink(toolkitDir string, link string, target string) error {
	symlinkPath := filepath.Join(toolkitDir, link)
	targetPath := filepath.Base(target)
	log.Infof("Creating symlink '%v' -> '%v'", symlinkPath, targetPath)

	err := os.Symlink(targetPath, symlinkPath)
	if err != nil {
		return fmt.Errorf("error creating symlink '%v' => '%v': %v", symlinkPath, targetPath, err)
	}
	return nil
}

// installFileToFolder copies a source file to a destination folder.
// The path of the input file is ignored.
// e.g. installFileToFolder("/some/path/file.txt", "/output/path")
// will result in a file "/output/path/file.txt" being generated
func installFileToFolder(destFolder string, src string) (string, error) {
	name := filepath.Base(src)
	return installFileToFolderWithName(destFolder, name, src)
}

// cp src destFolder/name
func installFileToFolderWithName(destFolder string, name, src string) (string, error) {
	dest := filepath.Join(destFolder, name)
	err := installFile(dest, src)
	if err != nil {
		return "", fmt.Errorf("error copying '%v' to '%v': %v", src, dest, err)
	}
	return dest, nil
}

// installFile copies a file from src to dest and maintains
// file modes
func installFile(dest string, src string) error {
	log.Infof("Installing '%v' to '%v'", src, dest)

	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening source: %v", err)
	}
	defer source.Close()

	destination, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("error creating destination: %v", err)
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return fmt.Errorf("error copying file: %v", err)
	}

	err = applyModeFromSource(dest, src)
	if err != nil {
		return fmt.Errorf("error setting destination file mode: %v", err)
	}
	return nil
}

// applyModeFromSource sets the file mode for a destination file
// to match that of a specified source file
func applyModeFromSource(dest string, src string) error {
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("error getting file info for '%v': %v", src, err)
	}
	err = os.Chmod(dest, sourceInfo.Mode())
	if err != nil {
		return fmt.Errorf("error setting mode for '%v': %v", dest, err)
	}
	return nil
}

// resolveLink finds the target of a symlink or the file itself in the
// case of a regular file.
// This is equivalent to running `readlink -f ${l}`
func resolveLink(l string) (string, error) {
	resolved, err := filepath.EvalSymlinks(l)
	if err != nil {
		return "", fmt.Errorf("error resolving link '%v': %v", l, err)
	}
	if l != resolved {
		log.Infof("Resolved link: '%v' => '%v'", l, resolved)
	}
	return resolved, nil
}

// getInstalledPath returns the path when file src is installed the specified
// folder.
func getInstalledPath(destFolder string, src string) string {
	filename := filepath.Base(src)
	return filepath.Join(destFolder, filename)
}

// ensureExecutable is equivalent to running chmod +x on the specified file
func ensureExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error getting file info for '%v': %v", path, err)
	}
	executableMode := info.Mode() | 0111
	err = os.Chmod(path, executableMode)
	if err != nil {
		return fmt.Errorf("error setting executable mode for '%v': %v", path, err)
	}
	return nil
}

func createDirectories(dir ...string) error {
	for _, d := range dir {
		log.Infof("Creating directory '%v'", d)
		err := os.MkdirAll(d, 0755)
		if err != nil {
			return fmt.Errorf("error creating directory: %v", err)
		}
	}
	return nil
}
