#!/bin/bash

# Creates the distribution archives for the products that are
# provided as arguments. If called without arguments, creates
# distributions for all of the products. The distribution
# artifacts are written to the `dist/<product>/build/distributions`
# directory.

set -eux -o pipefail
source scripts/script_functions.sh || source script_functions.sh

project_root="$( git rev-parse --show-toplevel )"

build_products() {
    local products=$1

    local version_ld_flag="$(go_version_ld_flag)"
    local cwd=$(pwd)

    local product_paths=()
    for product in $products; do
        product_paths+=("./$product")
    done

    echo "Building $products..."
    gox -verbose \
        -os="linux darwin" \
        -arch="amd64" \
        -output "dist/{{.Dir}}/build/{{.Dir}}-{{.OS}}-{{.Arch}}" \
        -ldflags "$version_ld_flag" \
        ${product_paths[@]}
    # `gox -verbose` includes newline at the beginning of its output,
    # so visual balance is nicer when newline is included after as well
    echo ""
    echo "Done building $products"
}

set_manifest_product_name_and_version() {
    local dist_path=$1
    local product=$2
    local version=$3

    local manifest="$dist_path/deployment/manifest.yml"
    # Not using 'sed -i' because it behaves differently on Mac
    sed -e "s/@productName@/$product/" -e "s/@productVersion@/$version/" \
        "$manifest" > "${manifest}.tmp"
    mv "$manifest"{.tmp,}
}

layout_product() {
    local product=$1
    local version=$2

    local dist=$product-$version
    local product_path=$project_root/dist/$product
    mkdir -p "$project_root/dist/$product/build/$dist"

    # TODO(rfink) Not needed?
    # cp -r $product_path/buildSrc/* "$product_path/build/$dist"
    find "$product_path/build/$dist" -name .gitkeep -delete

    # TODO(rfink) Not needed?
    # set_manifest_product_name_and_version "$product_path/build/$dist" "$product" "$version"

    for TARGET in linux-amd64 darwin-amd64; do
        mkdir -p "$product_path/build/$dist/service/bin/$TARGET/"
        cp "$product_path/build/$product-$TARGET" \
            "$product_path/build/$dist/service/bin/$TARGET/$product"
    done

    rm -f "$product_path/build/$product-latest"
    ln -s "$product_path/build/$dist" "$product_path/build/$product-latest"
}

tar_product() {
    local product=$1
    local version=$2

    local dist=$product-$version
    local product_path="./dist/$product"

    echo "Packaging $dist.tgz..."
    tar -C $product_path/build/ -czf "$product_path/build/distributions/$dist.tgz" \
        "$dist"
}

PRODUCTS="${@:-""}"
if [ -z "$PRODUCTS" ]; then
    # if arguments are empty, default to get_products
    PRODUCTS=$(get_products)
fi

for PRODUCT in $PRODUCTS; do
    mkdir -p "$project_root/dist/$PRODUCT/build/distributions"
done

# build product dists
build_products "$PRODUCTS"

VERSION="$(git_version)"

# layout product dists
for PRODUCT in $PRODUCTS; do
    layout_product $PRODUCT $VERSION
    tar_product $PRODUCT $VERSION
done

echo "Artifacts available in dist directory"
