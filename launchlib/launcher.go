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
	"os"
	"path"
	"regexp"
	"strings"
	"syscall"
)

const (
	TemplateDelimsOpen  = "{{"
	TemplateDelimsClose = "}}"
	// ExecPathBlackListRegex matches characters disallowed in paths we allow to be passed to exec()
	ExecPathBlackListRegex = `[^\w.\/_\-]`
)

func LaunchWithConfig(staticConfigFile, customConfigFile string) error {
	staticConfig, staticConfigErr := GetStaticConfigFromFile(staticConfigFile)
	if staticConfigErr != nil {
		return staticConfigErr
	}

	customConfig, customConfigErr := GetCustomConfigFromFile(customConfigFile)
	if customConfigErr != nil {
		return customConfigErr
	}

	launchErr := Launch(&staticConfig, &customConfig)
	if launchErr != nil {
		return launchErr
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

type processExecutor interface {
	Exec(executable string, args []string, env []string) error
}

type syscallProcessExecutor struct{}

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

func Launch(staticConfig *StaticLauncherConfig, customConfig *CustomLauncherConfig) error {
	fmt.Printf("Launching with static configuration %v and custom configuration %v\n", *staticConfig, *customConfig)

	workingDir := getWorkingDir()
	fmt.Println("Working directory:", workingDir)

	var args []string
	var executable string
	var executableErr error

	if staticConfig.ConfigType == "java" {
		javaHome, javaHomeErr := getJavaHome(staticConfig.JavaConfig.JavaHome)
		if javaHomeErr != nil {
			return javaHomeErr
		}
		fmt.Println("Using JAVA_HOME:", javaHome)

		classpath := joinClasspathEntries(absolutizeClasspathEntries(workingDir, staticConfig.JavaConfig.Classpath))
		fmt.Println("Classpath:", classpath)

		executable, executableErr = verifyPathIsSafeForExec(path.Join(javaHome, "/bin/java"))
		if executableErr != nil {
			return executableErr
		}
		args = append(args, executable) // 0th argument is the command itself
		args = append(args, staticConfig.JavaConfig.JvmOpts...)
		args = append(args, customConfig.JvmOpts...)
		args = append(args, "-classpath", classpath)
		args = append(args, staticConfig.JavaConfig.MainClass)
	} else if staticConfig.ConfigType == "executable" {
		executable, executableErr = verifyPathIsSafeForExec(staticConfig.Executable)
		if executableErr != nil {
			return executableErr
		}
		args = append(args, executable) // 0th argument is the command itself
	} else {
		return fmt.Errorf("You can't launch type %v, this should have errored in config validation", staticConfig.ConfigType)
	}

	args = append(args, staticConfig.Args...)
	fmt.Printf("Argument list to executable binary: %v\n\n", args)

	env := replaceEnvironmentVariables(merge(staticConfig.Env, customConfig.Env))

	execWithChecks(executable, args, env, &syscallProcessExecutor{})
	return nil
}

func execWithChecks(executable string, args []string, customEnv map[string]string, p processExecutor) {
	env := os.Environ()
	for key, value := range customEnv {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	execErr := p.Exec(executable, args, env)
	if execErr != nil {
		if os.IsNotExist(execErr) {
			fmt.Println("Executable not found at:", executable)
		}
		panic(execErr)
	}
}

func (s *syscallProcessExecutor) Exec(executable string, args, env []string) error {
	return syscall.Exec(executable, args, env)
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
