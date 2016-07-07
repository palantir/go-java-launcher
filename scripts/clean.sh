#!/bin/bash

set -eu -o pipefail
source scripts/script_functions.sh || source script_functions.sh

project_root="$( git rev-parse --show-toplevel )"

PRODUCTS="${@:-""}"
if [ -z "$PRODUCTS" ]; then
    # if arguments are empty, default to get_products
    PRODUCTS=$(get_products)
fi

for PRODUCT in $PRODUCTS; do
    rm -rf "./dist/$PRODUCT/build"
done
