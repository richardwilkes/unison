#! /usr/bin/env bash

set -eo pipefail

trap 'echo -e "\033[33;5mBuild failed on build.sh:$LINENO\033[0m"' ERR

for arg in "$@"; do
	case "$arg" in
	--all | -a)
		LINT=1
		TEST=1
		RACE=-race
		;;
	--lint | -l)
		LINT=1
		;;
	--race | -r)
		TEST=1
		RACE=-race
		;;
	--test | -t)
		TEST=1
		;;
	--help | -h)
		echo "$0 [options]"
		echo "  -a, --all  Equivalent to --lint --race"
		echo "  -l, --lint Run the linters"
		echo "  -r, --race Run the tests with race-checking enabled"
		echo "  -t, --test Run the tests"
		echo "  -h, --help This help text"
		exit 0
		;;
	*)
		echo "Invalid argument: $arg"
		exit 1
		;;
	esac
done

# The module is 100% cgo-free and must stay that way: everything is built and tested with cgo disabled so any
# accidental reintroduction fails loudly. (The -race test run below is the one exception; see the comment there.)
# CGO_ENABLED is restored to its prior state on exit so the setting cannot leak out of this script.
ORIG_CGO_ENABLED_SET=${CGO_ENABLED+set}
ORIG_CGO_ENABLED=${CGO_ENABLED-}
restore_cgo_enabled() {
	if [ "$ORIG_CGO_ENABLED_SET" = "set" ]; then
		export CGO_ENABLED="$ORIG_CGO_ENABLED"
	else
		unset CGO_ENABLED
	fi
}
trap restore_cgo_enabled EXIT
export CGO_ENABLED=0

# Guard against cgo creeping back in. A stray cgo file would not necessarily break the CGO_ENABLED=0 build (build
# constraints just exclude it), so check for import "C" explicitly, in both its single and grouped import forms.
echo -e "\033[33mVerifying the module is cgo-free...\033[0m"
CGO_USERS=$(grep -rlE --include='*.go' -e '^import[[:space:]]+"C"' -e '^[[:space:]]*"C"[[:space:]]*(//.*)?$' . || true)
if [ -n "$CGO_USERS" ]; then
	echo -e "\033[31mcgo is not permitted in this module, but these files import \"C\":\033[0m"
	echo "$CGO_USERS"
	exit 1
fi

# Generate the source
echo -e "\033[33mGenerating...\033[0m"
go generate ./cmd/enumgen/main.go

# Build the Go code
echo -e "\033[33mBuilding Go code...\033[0m"
go build -v ./...

# Run the linters
if [ "$LINT"x == "1x" ]; then
	GOLANGCI_LINT_VERSION=$(curl --head -s https://github.com/golangci/golangci-lint/releases/latest | grep -i location: | sed 's/^.*v//' | tr -d '\r\n')
	TOOLS_DIR=$(go env GOPATH)/bin
	if [ ! -e "$TOOLS_DIR/golangci-lint" ] || [ "$("$TOOLS_DIR/golangci-lint" version 2>&1 | awk '{ print $4 }' || true)x" != "${GOLANGCI_LINT_VERSION}x" ]; then
		echo -e "\033[33mInstalling version $GOLANGCI_LINT_VERSION of golangci-lint into $TOOLS_DIR...\033[0m"
		mkdir -p "$TOOLS_DIR"
		curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/main/install.sh | sh -s -- -b "$TOOLS_DIR" v$GOLANGCI_LINT_VERSION
	fi
	# Lint for every supported platform, not just the host, so problems in platform-specific files (e.g.
	# *_windows.go) are caught no matter where the script is run.
	for TARGET_GOOS in darwin linux windows; do
		echo -e "\033[33mLinting for $TARGET_GOOS...\033[0m"
		GOOS=$TARGET_GOOS "$TOOLS_DIR/golangci-lint" run
	done
fi

# Run the tests
if [ "$TEST"x == "1x" ]; then
	TEST_CGO=0
	if [ -n "$RACE" ]; then
		echo -e "\033[33mTesting with -race enabled...\033[0m"
		if [ "$(go env GOOS)" != "darwin" ]; then
			# Go's prebuilt race runtime itself requires cgo everywhere except macOS ("go: -race requires cgo").
			# The module still contains no cgo (enforced by the guard above); this only changes how the test
			# binaries link the race runtime.
			TEST_CGO=1
		fi
	else
		echo -e "\033[33mTesting...\033[0m"
	fi
	# The "|| true" keeps pipefail from failing the build if grep filters out every line of output.
	CGO_ENABLED=$TEST_CGO go test $RACE ./... | { grep -v "no test files" || true; }
fi

# Install the packager
echo -e "\033[33mInstalling upack...\033[0m"
go install -v ./cmd/upack
