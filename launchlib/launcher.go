// Copyright 2016 Palantir Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package launchlib

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

const (
	TemplateDelimsOpen  = "{{"
	TemplateDelimsClose = "}}"
	// ExecPathBlackListRegex matches characters disallowed in paths we allow to be passed to exec()
	ExecPathBlackListRegex = `[^\w.\/_\-]`
)

type ProcCmd struct {
	Name        string
	Cmd         *exec.Cmd
	OutFileName string
}

func CompileCmdsFromConfigFiles() (procCmds []ProcCmd, rErr error) {
	primaryStdout, err := os.Create(PrimaryOutputFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create primary output file: "+PrimaryOutputFile)
	}
	defer func() {
		if cErr := primaryStdout.Close(); rErr == nil && cErr != nil {
			rErr = errors.Wrap(err, "failed to close primary output file: "+PrimaryOutputFile)
		}
	}()
	staticConfig, customConfig, err := GetConfigsFromFiles(LauncherStaticFile, LauncherCustomFile, primaryStdout)

	if err != nil {
		return nil, err
	}
	return CompileCmdsFromConfig(&staticConfig, &customConfig, primaryStdout)
}

func CompileCmdsFromConfig(staticConfig *PrimaryStaticLauncherConfig, customConfig *PrimaryCustomLauncherConfig,
	primaryStdout io.Writer) (procCmds []ProcCmd, rErr error) {
	procCmds = make([]ProcCmd, 0, 1+len(staticConfig.SubProcesses))

	primaryCmd, err := CompileCmdFromConfig(&staticConfig.StaticLauncherConfig, &customConfig.CustomLauncherConfig,
		primaryStdout)
	if err != nil {
		return nil, err
	}
	procCmds = append(procCmds, ProcCmd{"primary", primaryCmd, PrimaryOutputFile})
	for name, subProcStatic := range staticConfig.SubProcesses {
		subProcCustom, ok := customConfig.SubProcesses[name]
		if !ok {
			return nil, errors.Errorf("no custom launcher config exists for subProcess config '%s'", name)
		}
		subProcOutFileName := fmt.Sprintf(OutputFileFormat, name+"-")
		subProcStdout, err := os.Create(subProcOutFileName)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create subprocess output file: "+subProcOutFileName)
		}
		defer func() {
			if cErr := subProcStdout.Close(); rErr == nil && cErr != nil {
				rErr = errors.Wrap(err, "failed to close subprocess output file: "+subProcOutFileName)
			}
		}()
		subProcCmd, err := CompileCmdFromConfig(&subProcStatic, &subProcCustom, subProcStdout)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to compile command for subProcess config '%s'", name)
		}
		procCmds = append(procCmds, ProcCmd{name, subProcCmd, subProcOutFileName})
	}
	return procCmds, nil
}

func CompileCmdFromConfig(staticConfig *StaticLauncherConfig, customConfig *CustomLauncherConfig,
	stdout io.Writer) (*exec.Cmd, error) {
	fmt.Fprintf(stdout, "Launching with static configuration %v and custom configuration %v\n", *staticConfig,
		*customConfig)

	workingDir := getWorkingDir()
	fmt.Fprintln(stdout, "Working directory:", workingDir)

	var args []string
	var executable string
	var executableErr error

	if staticConfig.Type == "java" {
		javaHome, javaHomeErr := getJavaHome(staticConfig.JavaConfig.JavaHome)
		if javaHomeErr != nil {
			return nil, javaHomeErr
		}
		fmt.Fprintln(stdout, "Using JAVA_HOME:", javaHome)

		classpath := joinClasspathEntries(absolutizeClasspathEntries(workingDir, staticConfig.JavaConfig.Classpath))
		fmt.Fprintln(stdout, "Classpath:", classpath)

		executable, executableErr = verifyPathIsSafeForExec(path.Join(javaHome, "/bin/java"))
		if executableErr != nil {
			return nil, executableErr
		}
		args = append(args, executable) // 0th argument is the command itself
		args = append(args, staticConfig.JavaConfig.JvmOpts...)
		args = append(args, customConfig.JvmOpts...)
		args = append(args, "-classpath", classpath)
		args = append(args, staticConfig.JavaConfig.MainClass)
	} else if staticConfig.Type == "executable" {
		executable, executableErr = verifyPathIsSafeForExec(staticConfig.Executable)
		if executableErr != nil {
			return nil, executableErr
		}
		args = append(args, executable) // 0th argument is the command itself
	} else {
		return nil, fmt.Errorf("You can't launch type %v, this should have errored in config validation", staticConfig.Type)
	}

	args = append(args, staticConfig.Args...)
	fmt.Fprintf(stdout, "Argument list to executable binary: %v\n\n", args)

	env := replaceEnvironmentVariables(merge(staticConfig.Env, customConfig.Env))

	return createCmd(executable, args, env)
}

func MkDirs(dirs []string, stdout io.Writer) error {
	isDirMatcher := regexp.MustCompile(`^[A-Za-z0-9]+(/[A-Za-z0-9]+)*$`).MatchString
	for _, dir := range dirs {
		if !isDirMatcher(dir) {
			return fmt.Errorf("Cannot create directory with non [A-Za-z0-9] characters: %s", dir)
		}

		fmt.Fprintf(stdout, "Creating directory: %s\n", dir)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

// Returns true iff the given path is safe to be passed to exec(): must not contain funky characters and be a valid file.
func verifyPathIsSafeForExec(execPath string) (string, error) {
	if unsafe, err := regexp.MatchString(ExecPathBlackListRegex, execPath); err != nil {
		return "", err
	} else if unsafe {
		return "", fmt.Errorf("Unsafe execution path: %q ", execPath)
	} else if _, statErr := os.Stat(execPath); statErr != nil {
		return "", statErr
	}

	return execPath, nil
}

// Returns explicitJavaHome if it is not the empty string, or the value of the JAVA_HOME environment variable otherwise.
// Panics if neither of them is set.
func getJavaHome(explicitJavaHome string) (string, error) {
	if explicitJavaHome != "" {
		return explicitJavaHome, nil
	}

	javaHome := os.Getenv("JAVA_HOME")
	if len(javaHome) == 0 {
		return "", fmt.Errorf("JAVA_HOME environment variable not set")
	}
	return javaHome, nil
}

func getWorkingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}

// Prepends each of the given classpath entries with the given working directory.
func absolutizeClasspathEntries(workingDir string, relativeClasspathEntries []string) []string {
	absoluteClasspathEntries := make([]string, len(relativeClasspathEntries))
	for i, entry := range relativeClasspathEntries {
		absoluteClasspathEntries[i] = path.Join(workingDir, entry)
	}
	return absoluteClasspathEntries
}

func joinClasspathEntries(classpathEntries []string) string {
	return strings.Join(classpathEntries, ":")
}

func createCmd(executable string, args []string, customEnv map[string]string) (*exec.Cmd, error) {
	env := os.Environ()
	for key, value := range customEnv {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	cmd := &exec.Cmd{
		Path: executable,
		Args: args,
		Env:  env,
	}

	return cmd, nil
}

// Performs replacement of all replaceable values in env, returning a new
// map, with the same keys as env, but possibly changed values.
func replaceEnvironmentVariables(env map[string]string) map[string]string {
	replacer := createReplacer()

	returnMap := make(map[string]string)
	for key, value := range env {
		returnMap[key] = replacer.Replace(value)
	}

	return returnMap
}

// copy all the keys and values from overrideMap into origMap. If a key already
// exists in origMap, its value is overridden.
func merge(origMap, overrideMap map[string]string) map[string]string {
	if len(overrideMap) == 0 {
		return origMap
	}

	returnMap := make(map[string]string)
	for key, value := range origMap {
		returnMap[key] = value
	}
	for key, value := range overrideMap {
		returnMap[key] = value
	}
	return returnMap
}

func createReplacer() *strings.Replacer {
	return strings.NewReplacer(
		delim("CWD"), getWorkingDir(),
	)
}

func delim(str string) string {
	return fmt.Sprintf("%s%s%s", TemplateDelimsOpen, str, TemplateDelimsClose)
}
