package lib

import (
	"io/ioutil"
	"strconv"
	"os"
	"path/filepath"
	"strings"
)

var files = []string{LauncherStaticFile, LauncherCustomFile, OutputFile, Pidfile}

func setup() {
	Setup("testdata/launcher-static-null.yml", "testdata/launcher-custom-null.yml")
}

func Setup(launcherStaticFilePath string, launcherCustomFilePath string) {
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file), 0777); err != nil {
			panic(err)
		}
	}

	if err := os.Link(launcherStaticFilePath, LauncherStaticFile); err != nil {
		panic(err)
	}
	if err := os.Link(launcherCustomFilePath, LauncherCustomFile); err != nil {
		panic(err)
	}
}

func Teardown() {
	for _, file := range files {
		if err := os.RemoveAll(strings.Split(file, "/")[0]); err != nil {
			panic(err)
		}
	}
}

func WritePid(pid int) {
	if err := ioutil.WriteFile(Pidfile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		panic(err)
	}
}

func ReadPid() int {
	pidBytes, err := ioutil.ReadFile(Pidfile)
	if err != nil {
		panic(err)
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		panic(err)
	}
	return pid
}
