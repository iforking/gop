//go:build ignore
// +build ignore

/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

func checkPathExist(path string, isDir bool) bool {
	stat, err := os.Stat(path)
	isExists := !os.IsNotExist(err)
	if isDir {
		return isExists && stat.IsDir()
	}
	return isExists && !stat.IsDir()
}

func trimRight(s string) string {
	return strings.TrimRight(s, " \t\r\n")
}

// Path returns single path to check
type Path struct {
	path  string
	isDir bool
}

func (p *Path) checkExists(rootDir string) bool {
	absPath := filepath.Join(rootDir, p.path)
	return checkPathExist(absPath, p.isDir)
}

func getGopRoot() string {
	pwd, _ := os.Getwd()

	pathsToCheck := []Path{
		{path: "cmd/gop", isDir: true},
		{path: "builtin", isDir: true},
		{path: "go.mod", isDir: false},
		{path: "go.sum", isDir: false},
	}

	for _, path := range pathsToCheck {
		if !path.checkExists(pwd) {
			println("Error: This script should be run at the root directory of gop repository.")
			os.Exit(1)
		}
	}
	return pwd
}

var gopRoot = getGopRoot()
var initCommandExecuteEnv = os.Environ()
var commandExecuteEnv = initCommandExecuteEnv

// Always put `gop` command as the first item, as it will be referenced by below code.
var gopBinFiles = []string{"gop", "gopfmt"}
var versionFile = filepath.Join(gopRoot, "VERSION")

const (
	inWindows = (runtime.GOOS == "windows")
)

func init() {
	if inWindows {
		for index, file := range gopBinFiles {
			file += ".exe"
			gopBinFiles[index] = file
		}
	}
}

func execCommand(command string, arg ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(command, arg...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = commandExecuteEnv
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func getGitBranch() string {
	branch, _, err := execCommand("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	return trimRight(branch)
}

func checkoutBranch(branch string) (string, error) {
	_, stderr, err := execCommand("git", "checkout", branch)
	return stderr, err
}

func isGitRepo() bool {
	gitDir, _, err := execCommand("git", "rev-parse", "--git-dir")
	if err != nil {
		return false
	}
	return checkPathExist(filepath.Join(gopRoot, trimRight(gitDir)), true)
}

func getBuildDateTime() string {
	now := time.Now()
	return now.Format("2006-01-02_15-04-05")
}

func getBuildVer() string {
	tagRet, tagErr, err := execCommand("git", "describe", "--tags")
	if err != nil || tagErr != "" {
		return ""
	}
	return trimRight(tagRet)
}

func getGopBuildFlags() string {
	defaultGopRoot := gopRoot
	if gopRootFinal := os.Getenv("GOPROOT_FINAL"); gopRootFinal != "" {
		defaultGopRoot = gopRootFinal
	}
	buildFlags := fmt.Sprintf("-X \"github.com/goplus/gop/env.defaultGopRoot=%s\"", defaultGopRoot)
	buildFlags += fmt.Sprintf(" -X \"github.com/goplus/gop/env.buildDate=%s\"", getBuildDateTime())

	version := findGopVersion()
	buildFlags += fmt.Sprintf(" -X \"github.com/goplus/gop/env.buildVersion=%s\"", version)

	return buildFlags
}

func detectGopBinPath() string {
	return filepath.Join(gopRoot, "bin")
}

func detectGoBinPath() string {
	goBin, ok := os.LookupEnv("GOBIN")
	if ok {
		return goBin
	}

	goPath, ok := os.LookupEnv("GOPATH")
	if ok {
		list := filepath.SplitList(goPath)
		if len(list) > 0 {
			// Put in first directory of $GOPATH.
			return filepath.Join(list[0], "bin")
		}
	}

	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "go", "bin")
}

func linkGoplusToLocalBin() string {
	println("Start Linking.")

	gopBinPath := detectGopBinPath()
	goBinPath := detectGoBinPath()
	if !checkPathExist(gopBinPath, true) {
		log.Fatalf("Error: %s is not existed, you should build Go+ before linking.\n", gopBinPath)
	}
	if !checkPathExist(goBinPath, true) {
		if err := os.MkdirAll(goBinPath, 0755); err != nil {
			fmt.Printf("Error: target directory %s is not existed and we can't create one.\n", goBinPath)
			log.Fatalln(err)
		}
	}

	for _, file := range gopBinFiles {
		sourceFile := filepath.Join(gopBinPath, file)
		if !checkPathExist(sourceFile, false) {
			log.Fatalf("Error: %s is not existed, you should build Go+ before linking.\n", sourceFile)
		}
		targetLink := filepath.Join(goBinPath, file)
		if checkPathExist(targetLink, false) {
			// Delete existed one
			if err := os.Remove(targetLink); err != nil {
				log.Fatalln(err)
			}
		}
		if err := os.Symlink(sourceFile, targetLink); err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("Link %s to %s successfully.\n", sourceFile, targetLink)
	}

	println("End linking.")
	return goBinPath
}

func buildGoplusTools(useGoProxy bool) {
	commandsDir := filepath.Join(gopRoot, "cmd")
	buildFlags := getGopBuildFlags()

	if useGoProxy {
		println("Info: we will use goproxy.cn as a Go proxy to accelerate installing process.")
		commandExecuteEnv = append(commandExecuteEnv,
			"GOPROXY=https://goproxy.cn,direct",
		)
	}

	// Install Go+ binary files under current ./bin directory.
	gopBinPath := detectGopBinPath()
	if err := os.Mkdir(gopBinPath, 0755); err != nil && !os.IsExist(err) {
		println("Error: Go+ can't create ./bin directory to put build assets.")
		log.Fatalln(err)
	}

	println("Installing Go+ tools...\n")
	os.Chdir(commandsDir)
	buildOutput, buildErr, err := execCommand("go", "build", "-o", gopBinPath, "-v", "-ldflags", buildFlags, "./...")
	print(buildErr)
	if err != nil {
		log.Fatalln(err)
	}
	print(buildOutput)

	// Clear gop run cache
	cleanGopRunCache()

	installPath := linkGoplusToLocalBin()

	println("\nGo+ tools installed successfully!")

	if _, _, err := execCommand("gop", "version"); err != nil {
		showHelpPostInstall(installPath)
	}
}

func showHelpPostInstall(installPath string) {
	println("\nNEXT STEP:")
	println("\nWe just installed Go+ into the directory: ", installPath)
	message := `
To setup a better Go+ development environment,
we recommend you add the above install directory into your PATH environment variable.
	`
	println(message)
}

func runTestcases() {
	println("Start running testcases.")
	os.Chdir(gopRoot)

	coverage := "-coverprofile=coverage.txt"
	gopCommand := filepath.Join(detectGopBinPath(), gopBinFiles[0])
	if !checkPathExist(gopCommand, false) {
		println("Error: Go+ must be installed before running testcases.")
		os.Exit(1)
	}

	testOutput, testErr, err := execCommand(gopCommand, "test", coverage, "-covermode=atomic", "./...")
	println(testOutput)
	println(testErr)
	if err != nil {
		println(err.Error())
	}

	println("End running testcases.")
}

func clean() {
	gopBinPath := detectGopBinPath()
	goBinPath := detectGoBinPath()

	// Clean links
	for _, file := range gopBinFiles {
		targetLink := filepath.Join(goBinPath, file)
		if checkPathExist(targetLink, false) {
			if err := os.Remove(targetLink); err != nil {
				log.Fatalln(err)
			}
		}
	}

	// Clean build binary files
	if checkPathExist(gopBinPath, true) {
		if err := os.RemoveAll(gopBinPath); err != nil {
			log.Fatalln(err)
		}
	}

	cleanGopRunCache()
}

func cleanGopRunCache() {
	homeDir, _ := os.UserHomeDir()
	runCacheDir := filepath.Join(homeDir, ".gop", "run")
	files := []string{"go.mod", "go.sum"}
	for _, file := range files {
		fullPath := filepath.Join(runCacheDir, file)
		if checkPathExist(fullPath, false) {
			if err := os.Remove(fullPath); err != nil {
				log.Fatalln(err)
			}
		}
	}
}

func uninstall() {
	println("Uninstalling Go+ and related tools.")
	clean()
	println("Go+ and related tools uninstalled successfully.")
}

func isInChina() bool {
	const prefix = "LANG=\""
	out, errMsg, err := execCommand("locale")
	if err != nil || errMsg != "" {
		return false
	}
	if strings.HasPrefix(out, prefix) {
		out = out[len(prefix):]
		return strings.HasPrefix(out, "zh_CN") || strings.HasPrefix(out, "zh_HK")
	}
	return false
}

// findGopVersion returns current version of gop
func findGopVersion() string {
	// Read version from VERSION file
	if checkPathExist(versionFile, false) {
		data, err := os.ReadFile(versionFile)
		if err == nil {
			version := trimRight(string(data))
			return version
		}
	}

	// Read version from git repo
	if !isGitRepo() {
		log.Fatal("Error: must be a git repo or a VERSION file existed.")
	}
	version := getBuildVer() // Closet tag on git log
	return version
}

// releaseNewVersion tags the repo with provided new tag, and writes new tag into VERSION file.
func releaseNewVersion(tag string) {
	if !isGitRepo() {
		log.Fatal("Error: Releasing a new version could only be operated under a git repo.")
	}
	println("Start releasing new version")

	version := tag

	re := regexp.MustCompile(`^v\d+?\.\d+?`)
	releaseBranch := re.FindString(version)
	sourceBranch := getGitBranch()

	if releaseBranch == "" {
		log.Fatal("Error: A valid version should be has form: vx.y.z")
	}

	// Checkout to release breanch
	if stderr, err := checkoutBranch(releaseBranch); err != nil {
		log.Fatalf("Error: checkout to release branch: %s failed with error: %v.", releaseBranch, stderr)
	}

	fmt.Printf("\nReleasing new version: %s\n\n", version)

	// Cache new version
	if err := os.WriteFile(versionFile, []byte(version), 0644); err != nil {
		log.Fatalf("Error: cache new version with error: %v\n", err)
	}

	// Tag the source code
	if _, stderr, err := execCommand("git", "tag", version); err != nil {
		log.Fatalf("Error: tag the source code with error: %v\n", stderr)
	}

	// Checkout back to source branch
	if stderr, err := checkoutBranch(sourceBranch); err != nil {
		log.Fatalf("Error: checkout to source branch: %s failed with error: %v.", sourceBranch, stderr)
	}

	println("End releasing new version:", tag)
}

func main() {
	isInstall := flag.Bool("install", false, "Install Go+")
	isTest := flag.Bool("test", false, "Run testcases")
	isUninstall := flag.Bool("uninstall", false, "Uninstall Go+")
	isGoProxy := flag.Bool("proxy", false, "Set GOPROXY for people in China")
	isAutoProxy := flag.Bool("autoproxy", false, "Check to set GOPROXY automatically")
	tag := flag.String("tag", "", "Release an new version with specified tag")

	flag.Parse()

	useGoProxy := *isGoProxy
	if !useGoProxy && *isAutoProxy {
		useGoProxy = isInChina()
	}
	flagActionMap := map[*bool]func(){
		isInstall:   func() { buildGoplusTools(useGoProxy) },
		isUninstall: uninstall,
		isTest:      runTestcases,
	}

	// Sort flags, for example: install flag should be checked earlier than test flag.
	flags := []*bool{isInstall, isTest, isUninstall}
	hasActionDone := false

	if *tag != "" {
		releaseNewVersion(*tag)
		hasActionDone = true
	}

	for _, flag := range flags {
		if *flag {
			flagActionMap[flag]()
			hasActionDone = true
		}
	}

	if !hasActionDone {
		println("Usage:\n")
		flag.PrintDefaults()
	}
}
