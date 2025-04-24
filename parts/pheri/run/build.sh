#!/bin/bash
set -e
# Environment
## Part Environment
export SNAPCRAFT_ARCH_BUILD_FOR="amd64"
export SNAPCRAFT_ARCH_BUILD_ON="amd64"
export SNAPCRAFT_ARCH_TRIPLET_BUILD_FOR="x86_64-linux-gnu"
export SNAPCRAFT_ARCH_TRIPLET_BUILD_ON="x86_64-linux-gnu"
export SNAPCRAFT_ARCH_TRIPLET="x86_64-linux-gnu"
export SNAPCRAFT_EXTENSIONS_DIR="/snap/snapcraft/14408/share/snapcraft/extensions"
export SNAPCRAFT_PARALLEL_BUILD_COUNT="3"
export SNAPCRAFT_PRIME="/home/tensormoney.com/public_html/pheri/prime"
export SNAPCRAFT_PROJECT_NAME="pheri"
export SNAPCRAFT_PROJECT_VERSION="0.1.0"
export SNAPCRAFT_PROJECT_DIR="/home/tensormoney.com/public_html/pheri"
export SNAPCRAFT_PROJECT_GRADE="stable"
export SNAPCRAFT_STAGE="/home/tensormoney.com/public_html/pheri/stage"
export SNAPCRAFT_TARGET_ARCH="amd64"
export SNAPCRAFT_CONTENT_DIRS=""
export SNAPCRAFT_PART_SRC="/home/tensormoney.com/public_html/pheri/parts/pheri/src"
export SNAPCRAFT_PART_SRC_WORK="/home/tensormoney.com/public_html/pheri/parts/pheri/src/"
export SNAPCRAFT_PART_BUILD="/home/tensormoney.com/public_html/pheri/parts/pheri/build"
export SNAPCRAFT_PART_BUILD_WORK="/home/tensormoney.com/public_html/pheri/parts/pheri/build/"
export SNAPCRAFT_PART_INSTALL="/home/tensormoney.com/public_html/pheri/parts/pheri/install"
## Plugin Environment
export SNAPCRAFT_GO_LDFLAGS="-ldflags -linkmode=external"
export CGO_ENABLED="1"
export GOBIN="${SNAPCRAFT_PART_INSTALL}/bin"
## User Environment

set -xeuo pipefail
go mod download
go install -p "${SNAPCRAFT_PARALLEL_BUILD_COUNT}"  ${SNAPCRAFT_GO_LDFLAGS} ./...
