package main

import "testing"

func TestMainMethod(t *testing.T) {
	LaunchWithConfig("test_resources/launcher-static.yml", "test_resources/launcher-custom.yml")
}

func TestMainMethodWithoutCustomConfig(t *testing.T) {
	LaunchWithConfig("test_resources/launcher-static.yml", "foo")
}
