#!/bin/bash

# Builds the products that are provided as arguments. If called
# without arguments, builds all of the products. The build output
# for a product is in the `./build/$PRODUCT` directory.

set -eu -o pipefail
source scripts/script_functions.sh || source script_functions.sh

VERSION="$(git_version)"
VERSION_LD_FLAG="$(go_version_ld_flag)"

PRODUCTS=("${@:-}")
if [ -z "$PRODUCTS" ]; then
    # if arguments are empty, default to get_products
    PRODUCTS=$(get_products)
fi
echo "products: $PRODUCTS"

OS_ARCH="$(uname -s | awk '{print tolower($0)}')-amd64"

for PRODUCT in $PRODUCTS; do
    echo "Building $PRODUCT..."
    go build \
        -o "./dist/$PRODUCT/build/$PRODUCT" \
        -ldflags "$VERSION_LD_FLAG" \
        "./$PRODUCT"
    echo "Done building $PRODUCT"
done
