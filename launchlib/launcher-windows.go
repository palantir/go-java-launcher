//go:build windows

package launchlib

const (
	javaExecutablePath = "bin/java.exe"
	// pathBlackListRegex matches characters disallowed in Windows paths we allow for program arguments
	// Windows paths need : for drive letters, \ for path separators, and spaces for paths like "Program Files"
	pathBlackListRegex = `[^\w.\/\\_\-: ]`
)
