#!/bin/bash

# Verifies that the project is valid by running checks such as
# `errcheck`, `outparamcheck` and `go tool vet` on all packages
# returned by the `get_packages` function. If all checks succeed,
# runs the Go tests for the packages as well. If the `-c` flag is
# specified, tests are run in coverage mode and a coverage report
# is written to the file `coverage.out`.

set -eu -o pipefail
source scripts/script_functions.sh || source script_functions.sh

# Verifies that every package provided has at least one file that matches
# the pattern "*_test.go". Important to ensure that coverage is reported
# correctly. If any packages do not have tests, the name is echoed and the
# script is terminated with an exit code of 1.
function verify_tests_exist() {
    local packages=$@

    local fail=
    for pkg in $packages; do
        if [ -z "$(find "$pkg" -maxdepth 1 -type f -name '*_test.go')" ]; then
            echo "Missing placeholder_test.go in $pkg"
            fail=true
        fi
    done

    if [ "$fail" = true ]; then
        exit 1
    fi
}

# if -c flag is specified, run tests with coverage
COVERAGE=false
while getopts ":c" opt; do
    case $opt in
        c)
            COVERAGE=true
            ;;
        \?)
            exit_with_message "Invalid option: -$OPTARG"
            ;;
    esac
done

PACKAGES=$(get_packages)

verify_tests_exist $PACKAGES

echo "Running errcheck..."
if ! errcheck $PACKAGES; then
    exit_with_message "errcheck failed: did not compile or error return values need to be checked"
fi

# TODO: publish outparamcheck as separate package/project
# echo "Running outparamcheck..."
# if ! go run ./outparamcheck/main.go $PACKAGES; then
#     exit_with_message "outparamcheck failed"
# fi

echo "Running go tool vet..."
if ! go tool vet $PACKAGES; then
    exit_with_message "go tool vet failed"
fi

echo "Running deadcode..."
if ! deadcode $PACKAGES; then
    exit_with_message "found dead code"
fi

echo "Running ineffassign..."
for pkg in $PACKAGES; do
    if ! ineffassign $pkg; then
        exit_with_message "found ineffectual assignment"
    fi
done

echo "Running varcheck..."
if ! varcheck $PACKAGES; then
    exit_with_message "found unused global variable or constant"
fi

echo "Running unconvert..."
if ! unconvert $PACKAGES; then
    exit_with_message "there are unnecessary conversions"
fi

echo "Running golint..."
if ! scripts/lint.sh; then
    exit_with_message "golint failed"
fi

if [ $COVERAGE = true ]; then
    echo "Running gotestcover..."
    CMD="gotestcover -v -covermode=count -coverprofile=coverage.out"
else
    echo "Running go test..."
    CMD="go test -v"
fi

$CMD $PACKAGES | tee gotest.out
