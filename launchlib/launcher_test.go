package launchlib

import (
	"testing"
	"os"
)

func TestGetJavaHome(t *testing.T) {
	originalJavaHome := os.Getenv("JAVA_HOME")
	setEnvOrFail("JAVA_HOME", "foo")

	javaHome := getJavaHome("")
	if javaHome != "foo" {
		t.Error("Expected JAVA_HOME='foo', found", javaHome)
	}
	javaHome = getJavaHome("explicit javahome")
	if javaHome != "explicit javahome" {
		t.Error("Expected JAVA_HOME='explicit javahome', found", javaHome)
	}

	setEnvOrFail("JAVA_HOME", originalJavaHome)
}

func setEnvOrFail(key string, value string) {
	err := os.Setenv(key, value)
	if err != nil {
		panic("Failed to set env var: " + key)
	}
}
