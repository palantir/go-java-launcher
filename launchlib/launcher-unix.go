//go:build unix

package launchlib

const (
	javaExecutablePath = "bin/java"
	// pathBlackListRegex matches characters disallowed in Unix paths we allow for program arguments
	pathBlackListRegex = `[^\w.\/_\-]`
)
