package lib

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var files = []string{LauncherStaticFile, LauncherCustomFile, OutputFile, Pidfile}

func setup(t *testing.T) {
	for _, file := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(file), 0777))
	}

	require.NoError(t, os.Link("testdata/launcher-static-null.yml", LauncherStaticFile))
	require.NoError(t, os.Link("testdata/launcher-custom-null.yml", LauncherCustomFile))
}

func teardown(t *testing.T) {
	for _, file := range files {
		require.NoError(t, os.RemoveAll(strings.Split(file, "/")[0]))
	}
}

func writePid(t *testing.T, pid int) {
	require.NoError(t, ioutil.WriteFile(Pidfile, []byte(strconv.Itoa(pid)), 0644))
}

func readPid(t *testing.T) int {
	pidBytes, err := ioutil.ReadFile(Pidfile)
	require.NoError(t, err)
	pid, err := strconv.Atoi(string(pidBytes))
	require.NoError(t, err)
	return pid
}
