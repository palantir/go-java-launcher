#!/bin/bash

source scripts/product.properties || source product.properties

# Echoes the products that are part of the project.
function get_products() {
    echo "$ALL_PRODUCTS"
}

# Echoes all of the packages that are part of the project (excluding vendored code and mock code).
function get_packages() {
    local packages=$(
        find . -mindepth 2 -type f -name '*.go' -not \( -path './vendor/*' -o -path '*/mocks/*' \) |
        xargs -n1 dirname |
        sort -u)

    echo "$packages"
}

# Echoes the version string, which is the result of
# `git describe --tags` with the first 'v' removed
# If there are any uncommitted changes in the repository,
# ".dirty" is appended to the version. If the script is
# not run in a Git repository, exits the script.
function git_version() {
  local version=$(git describe --tags | sed 's/^v//')

  if [ -z "$version" ]; then
      exit_with_message "Unable to determine version using git describe --tags"
  fi

  if [ -n "$(git status --porcelain)" ]; then
      version="${version}.dirty"
  fi

  echo "$version"
}

# returns 0 if current version is a snapshot version
# (defined as a version that contains the character "-");
# returns 1 otherwise.
function is_snapshot_version() {
    [[ $(git_version) =~ - ]]
}

# Echoes the value for "-ldflags" that should be used for the Go compiler.
function go_version_ld_flag() {
    local version=$(git_version)
    echo "-X version.version=$version"
}

# Echoes the provided messages and exits with exit code 1.
function exit_with_message() {
    echo "$@" >&2
    exit 1
}

# Verifies that the variables with the provided names
# are set. If any are not set, the variable names are
# echoed and the script exits with an exit code of 1.
function verify_env_variables_set() {
    local required_variables=("$@")
    local missing_variables=()

    for curr_variable in $required_variables; do
        if [ -z "${!curr_variable:-}" ]; then
            missing_variables+=("$curr_variable")
        fi
    done

    if [[ ${#missing_variables[@]} > 0 ]]; then
        exit_with_message "Required environment variables not set: ${missing_variables[@]}"
    fi
}
