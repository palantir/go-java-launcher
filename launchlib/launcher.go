package launchlib

import (
	"fmt"
	"os"
	"path"
	"strings"
	"syscall"
)

// Returns explicitJavaHome if it is not the empty string, or the value of the JAVA_HOME environment variable otherwise.
// Panics if neither of them is set.
func getJavaHome(explicitJavaHome string) string {
	if explicitJavaHome != "" {
		return explicitJavaHome
	}

	javaHome := os.Getenv("JAVA_HOME")
	if len(javaHome) == 0 {
		panic("JAVA_HOME environment variable not set")
	}
	return javaHome
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

func Launch(staticConfig *StaticLauncherConfig, customConfig *CustomLauncherConfig) {
	fmt.Printf("Launching with static configuration %v and custom configuration %v\n", *staticConfig, *customConfig)

	workingDir := getWorkingDir()
	fmt.Println("Working directory:", workingDir)

	javaHome := getJavaHome(staticConfig.JavaHome)
	fmt.Println("Using JAVA_HOME:", javaHome)
	javaCommand := path.Join(javaHome, "/bin/java")

	classpath := joinClasspathEntries(absolutizeClasspathEntries(workingDir, staticConfig.Classpath))
	fmt.Println("Classpath:", classpath)

	var args []string
	args = append(args, javaCommand) // 0th argument is the command itself
	args = append(args, staticConfig.JvmOpts...)
	args = append(args, customConfig.JvmOpts...)
	args = append(args, "-classpath", classpath)
	args = append(args, staticConfig.MainClass)
	args = append(args, staticConfig.Args...)
	fmt.Println("Argument list to Java binary:", args)

	env := os.Environ()
	execErr := syscall.Exec(javaCommand, args, env)
	if execErr != nil {
		panic(execErr)
	}
}
