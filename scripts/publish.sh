#!/bin/bash

# Publishes the distribution archives for the products that are
# provided as arguments. If called without arguments, publishes
# all of the products. Assumes that the artifacts for current
# version of the products to publish are present in the
# `build/distributions/` directory. If called with the `-l` flag,
# the products are published to the local Maven directory in `$HOME/.m2`.
# Otherwise, the products are published to Bintray using cURL and
# the following environment variables must be set:
#
# GROUP_ID: The Maven group under which to publish artifacts
# GROUP_PATH: The URL path corresponding to the above group (i.e., replace '.' with '/')
# BINTRAY_USERNAME: The username for the Bintray user.
# BINTRAY_PASSWORD: The password for the Bintray user.

BINTRAY_URL='https://api.bintray.com/content'
BINTRAY_ORG='palantir'
BINTRAY_REPO='releases'

set -eu -o pipefail
source scripts/script_functions.sh || source script_functions.sh
source scripts/product.properties || source product.properties

function get_pom() {
    local product=$1
    local version=$2

    # template string for POM
    local pomTemplate=\
'<?xml version="1.0" encoding="UTF-8"?>
<project xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd" xmlns="http://maven.apache.org/POM/4.0.0"
        xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <modelVersion>4.0.0</modelVersion>
  <groupId>{{GROUP_ID}}</groupId>
  <artifactId>{{PRODUCT}}</artifactId>
  <version>{{VERSION}}</version>
  <packaging>tgz</packaging>
</project>'

    # replace {{PRODUCT}} and {{VERSION}} with variable values, {{GROUP_ID}} with global constant
    local pom=$(echo "$pomTemplate" | sed "s/{{PRODUCT}}/$product/g;s/{{VERSION}}/$version/g;s/{{GROUP_ID}}/$GROUP_ID/g")
    echo "$pom"
}

function publish_version() {
    local publish_url=$1

    echo "Publishing artifacts in package $publish_url"
    result=`curl -s -X POST  \
         -u "$BINTRAY_USERNAME:$BINTRAY_PASSWORD" \
         "${publish_url}"`
    if [[ $result != *"files"* ]]
    then
        exit_with_message "Failed to publish ${publish_url}"
    fi
}

function upload_file() {
    local file_path=$1
    local upload_url=$2
    if [ ! -f "$file_path" ]; then
        exit_with_message "Artifact not found at $file_path"
    fi

    echo "Publishing $(basename $file_path) to ${upload_url}"
    result=`curl -s -T ${file_path} \
         -u "$BINTRAY_USERNAME:$BINTRAY_PASSWORD" \
         "${upload_url}"`
    if [[ $result != *"success"* ]]
    then
        exit_with_message "Failed to upload to ${upload_url}"
    fi
}

LOCAL_M2="$HOME/.m2"

LOCAL=false
while getopts ":l" opt; do
    case $opt in
        l)
            LOCAL=true
            shift $((OPTIND-1))
            ;;
        \?)
            exit_with_message "Invalid option: -$OPTARG"
            ;;
  esac
done

# if this is a non-local publish, ensure that proper environment variables are set
if [ $LOCAL != true ]; then
    verify_env_variables_set BINTRAY_URL BINTRAY_USERNAME BINTRAY_PASSWORD 
fi
if [ -n "${ALMANAC_PUBLISH:-}" ]; then
    verify_env_variables_set ALMANAC_ACCESS_ID ALMANAC_SECRET_KEY ALMANAC_URL
fi

VERSION="$(git_version)"

PRODUCTS=("${@:-}")
if [ -z "$PRODUCTS" ]; then
    # if arguments are empty, default to get_products
    PRODUCTS=$(get_products)
fi

if [ "$LOCAL" = true ]; then
    REPO="repository"
else
    REPO=$BINTRAY_ORG/$BINTRAY_REPO
fi

# create temporary directory for POM files
POM_TEMP_DIR=$(mktemp -d build/pomTempDir.XXXXXX)
# clean up temporary directory on script termination
trap 'rm -rf "$POM_TEMP_DIR"' EXIT

for PRODUCT in $PRODUCTS; do
    VERSION_PATH="${REPO}/${PRODUCT}/${VERSION}"
    PRODUCT_PATH="${VERSION_PATH}/${GROUP_PATH}"

    FILE="$PRODUCT-$VERSION.tgz"
    FILE_PATH="dist/$PRODUCT/build/distributions/$FILE"

    # create POM for artifact
    POM="$PRODUCT-$VERSION.pom"
    POM_PATH="${POM_TEMP_DIR}/$POM"
    echo "$(get_pom $PRODUCT $VERSION)" > "$POM_PATH"

    if [ $LOCAL != true ]; then
        # upload artifact
        FILE_URL="${BINTRAY_URL}/${PRODUCT_PATH}/${FILE}"
        upload_file "$FILE_PATH" "$FILE_URL"

        # upload POM
        POM_URL="${BINTRAY_URL}/${PRODUCT_PATH}/${POM}"
        upload_file "$POM_PATH" "$POM_URL"

        # publish all files
        PUBLISH_URL="${BINTRAY_URL}/${VERSION_PATH}/publish"
        publish_version "$PUBLISH_URL"
    else
        LOCAL_REPO_DIRECTORY="${LOCAL_M2}/${PRODUCT_PATH}"
        mkdir -p "$LOCAL_REPO_DIRECTORY"
        cp -v "$FILE_PATH" "$LOCAL_REPO_DIRECTORY"
        cp -v "$POM_PATH" "$LOCAL_REPO_DIRECTORY"
    fi
done
