#! /usr/bin/env bash
set -eEo pipefail

# -E above makes this trap fire for failures inside functions and subshells too, not just at the top level.
trap 'echo -e "\033[33;5mBuild failed on build.sh:$LINENO\033[0m"' ERR

# Everything below runs from the root of the source tree, wherever this script lives, whatever the tree is named and
# wherever the script is run from.
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$ROOT"

# The go.mod for github.com/richardwilkes/unison must be here, not in a directory above. Workspace mode is turned off
# for the check so that the answer describes this directory alone rather than whatever go.work is in effect.
MODULE_PATH=github.com/richardwilkes/unison
if ! FOUND_MODULE=$(GOWORK=off go list -m -f '{{.Path}} {{.Dir}}' 2>/dev/null) ||
	[ "${FOUND_MODULE%% *}" != "$MODULE_PATH" ] ||
	[ "$(cd "${FOUND_MODULE#* }" && pwd -P)" != "$(pwd -P)" ]; then
	echo "$ROOT is not the root of the $MODULE_PATH module (go reports: ${FOUND_MODULE:-no module})" >&2
	exit 1
fi

while [ $# -gt 0 ]; do
	arg="$1"
	shift
	case "$arg" in
	--all | -a)
		BUILD_GO=1
		BUILD_GEN=1
		FMT=1
		LINT=1
		TEST=1
		RACE=-race
		SOMETHING=1
		;;
	--go | -g)
		BUILD_GO=1
		SOMETHING=1
		;;
	--gen | -G)
		BUILD_GEN=1
		SOMETHING=1
		;;
	--fmt | -f)
		FMT=1
		SOMETHING=1
		;;
	--lint | -l)
		LINT=1
		SOMETHING=1
		;;
	--test | -t)
		TEST=1
		SOMETHING=1
		;;
	--race | -r)
		TEST=1
		RACE=-race
		SOMETHING=1
		;;
	--target | -T | --target=*)
		if [ "$arg" == "${arg#--target=}" ]; then
			if [ $# -eq 0 ]; then
				echo "$arg requires an os/arch value, e.g. windows/amd64" >&2
				exit 1
			fi
			TARGET="$1"
			shift
		else
			TARGET="${arg#--target=}"
		fi
		if [ "$TARGET" == "${TARGET#*/}" ]; then
			echo "Invalid target: $TARGET (expected os/arch, e.g. windows/amd64)" >&2
			exit 1
		fi
		export GOOS="${TARGET%%/*}"
		export GOARCH="${TARGET#*/}"
		;;
	--help | -h)
		echo "$0 [options]"
		echo "  -a, --all            Equivalent to --gen --go --fmt --lint --race"
		echo "  -f, --fmt            Verify the source formatting (gofumpt)"
		echo "  -g, --go             Build the Go code and install upack"
		echo "  -G, --gen            Generate the source"
		echo "  -l, --lint           Run the linters"
		echo "  -r, --race           Run the tests with race-checking enabled"
		echo "  -t, --test           Run the tests"
		echo "  -T, --target OS/ARCH Build the Go code for another platform, e.g. windows/amd64. GOOS and GOARCH"
		echo "                       set in the environment work too. upack is always installed for this machine's"
		echo "                       own platform"
		echo "  -h, --help           This help text"
		exit 0
		;;
	*)
		echo "Invalid argument: $arg"
		exit 1
		;;
	esac
done

# A go.work in a parent directory, such as one tying the main checkout to sibling modules, is also found from a worktree
# kept inside the tree, yet doesn't list that worktree, so go can't build it. Unless GOWORK was set explicitly, module
# mode is used whenever the workspace in effect doesn't include this directory. A workspace is also how a locally
# modified dependency (such as canvas) gets built in, though, so dropping one that supplies or replaces a module this
# build needs would quietly build against the wrong code. In that case the build stops instead; GOWORK=off says that
# building against go.mod alone is intended.
check_workspace_dropped() {
	local work_file="$1" used replaced required path dir version needed=()
	used=$(cd "$(dirname "$work_file")" && GOWORK="$work_file" go list -m -f '{{.Path}} {{.Dir}}' 2>/dev/null || true)
	replaced=$(go work edit -json "$work_file" 2>/dev/null | grep -A1 '"Old"' |
		sed -n 's/.*"Path": "\([^"]*\)".*/\1/p' || true)
	if [ -n "$used" ] || [ -n "$replaced" ]; then
		if ! required=$(go list -m -f '{{if not .Main}}{{.Path}} {{.Version}}{{end}}' all); then
			echo -e "\033[31mUnable to list this build's dependencies to compare against $work_file. Set GOWORK=off to" \
				"build against go.mod alone.\033[0m" >&2
			exit 1
		fi
	fi
	while read -r path dir; do
		if [ -n "$path" ] && [ "$path" != "$MODULE_PATH" ]; then
			version=$(awk -v p="$path" '$1 == p { print $2 }' <<<"$required")
			if [ -n "$version" ]; then
				needed+=("$path: would use $version from go.mod rather than $dir")
			fi
		fi
	done <<<"$used"
	while read -r path; do
		if [ -n "$path" ]; then
			version=$(awk -v p="$path" '$1 == p { print $2 }' <<<"$required")
			if [ -n "$version" ]; then
				needed+=("$path: would use $version from go.mod, as its replace directive would not apply")
			fi
		fi
	done <<<"$replaced"
	if [ ${#needed[@]} -ne 0 ]; then
		echo -e "\033[31m$work_file supplies modules this build needs:\033[0m" >&2
		printf '  %s\n' "${needed[@]}" >&2
		echo -e "\033[31mAdd $ROOT to that workspace (or give it a go.work of its own) to build against them, or set" \
			"GOWORK=off to build against go.mod alone.\033[0m" >&2
		exit 1
	fi
	echo -e "\033[33mIgnoring $work_file, which doesn't include $ROOT and supplies nothing this build needs.\033[0m" >&2
}
# Output is captured before being searched, here and below, rather than piped into "grep -q": grep stops reading at the
# first match, which can kill the writer with SIGPIPE, and pipefail then reports the whole pipeline as failing.
if [ -z "${GOWORK+set}" ]; then
	WORK_FILE=$(go env GOWORK)
	if [ -n "$WORK_FILE" ]; then
		WORK_MODULE_DIRS=$(go list -m -f '{{.Dir}}' 2>/dev/null || true)
		if ! grep -Fqx -e "$ROOT" -e "$(pwd -P)" <<<"$WORK_MODULE_DIRS"; then
			export GOWORK=off
			check_workspace_dropped "$WORK_FILE"
		fi
	fi
fi

# The build machine and the target are told apart here. Only the Go code's build is done for the target: the enum
# generator, the tests and the installed packager all run on this machine, so from here on GOOS and GOARCH name the
# build machine's platform, and the build names the target explicitly. The linters cover every supported platform
# regardless.
HOST_OS=$(go env GOHOSTOS)
HOST_ARCH=$(go env GOHOSTARCH)
TARGET_OS=$(go env GOOS)
TARGET_ARCH=$(go env GOARCH)
SUPPORTED_TARGETS=$(go tool dist list)
if ! grep -Fqx "$TARGET_OS/$TARGET_ARCH" <<<"$SUPPORTED_TARGETS"; then
	echo "Unsupported target: $TARGET_OS/$TARGET_ARCH" >&2
	exit 1
fi
export GOOS="$HOST_OS"
export GOARCH="$HOST_ARCH"
if [ "$TARGET_OS/$TARGET_ARCH" != "$HOST_OS/$HOST_ARCH" ]; then
	echo -e "\033[33mCross-compiling for $TARGET_OS/$TARGET_ARCH on $HOST_OS/$HOST_ARCH\033[0m"
fi

# The module is 100% cgo-free and must stay that way: everything is built and tested with cgo disabled so any
# accidental reintroduction fails loudly. (The -race test run below is the one exception; see the comment there.)
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

# The tools are built for this machine rather than run with "go run", so that they are built once and never pick up a
# target's settings.
BUILD_TOOLS_DIR=$(mktemp -d)
trap 'rm -rf "$BUILD_TOOLS_DIR"' EXIT
tool() {
	local name="$1"
	shift
	if [ ! -e "$BUILD_TOOLS_DIR/$name$(go env GOEXE)" ]; then
		go build -o "$BUILD_TOOLS_DIR/$name$(go env GOEXE)" "./cmd/$name"
	fi
	"$BUILD_TOOLS_DIR/$name$(go env GOEXE)" "$@"
}

if [ "$SOMETHING"x != "1x" ]; then
	BUILD_GEN=1
	BUILD_GO=1
fi

if [ "$BUILD_GEN"x == "1x" ]; then
	echo -e "\033[33mGenerating...\033[0m"
	tool enumgen -root "$ROOT"
fi

if [ "$BUILD_GO"x == "1x" ]; then
	echo -e "\033[33mBuilding the Go code...\033[0m"
	GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" go build -v ./...
fi

# Both the formatting check and the linters come out of golangci-lint, so whichever of them runs first installs it.
# Set GOLANGCI_LINT to the path of a binary to use it as is.
ensure_golangci_lint() {
	if [ -n "$GOLANGCI_LINT" ]; then
		return
	fi
	GOLANGCI_LINT_VERSION=$(curl --head -s https://github.com/golangci/golangci-lint/releases/latest | grep -i location: | sed 's/^.*v//' | tr -d '\r\n')
	TOOLS_DIR=$(go env GOPATH)/bin
	if [ ! -e "$TOOLS_DIR/golangci-lint" ] || [ "$("$TOOLS_DIR/golangci-lint" version 2>&1 | awk '{ print $4 }' || true)x" != "${GOLANGCI_LINT_VERSION}x" ]; then
		echo -e "\033[33mInstalling version $GOLANGCI_LINT_VERSION of golangci-lint into $TOOLS_DIR...\033[0m"
		mkdir -p "$TOOLS_DIR"
		curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/main/install.sh | sh -s -- -b "$TOOLS_DIR" v$GOLANGCI_LINT_VERSION
	fi
	GOLANGCI_LINT="$TOOLS_DIR/golangci-lint"
}

# `golangci-lint run` also enforces the `formatters` section of .golangci.yml, but only for the files it loads and not
# for paths excluded from linting, such as internal/w32. `golangci-lint fmt` ignores build constraints and those
# exclusions, so this single pass covers every file. It is also the one that prints the actual diff rather than a single
# "not properly formatted" issue per file.
if [ "$FMT"x == "1x" ]; then
	ensure_golangci_lint
	echo -e "\033[33mChecking the formatting of the Go code...\033[0m"
	if ! "$GOLANGCI_LINT" fmt --diff; then
		echo -e "\033[31mRun 'golangci-lint fmt' to apply the formatting shown above.\033[0m" >&2
		exit 1
	fi
	echo "0 issues."
fi

if [ "$LINT"x == "1x" ]; then
	ensure_golangci_lint
	# Lint for every supported platform, not just the host, so problems in platform-specific files (e.g.
	# *_windows.go) are caught no matter where the script is run. Each platform is linted twice, once for the
	# default build and once with GOEXPERIMENT=simd, because the goexperiment.simd-tagged sources (internal/pixconv,
	# internal/f32) are excluded from the default build and would otherwise never be analyzed.
	for LINT_OS in darwin linux windows; do
		for LINT_EXP in "" simd; do
			echo -e "\033[33mLinting for $LINT_OS${LINT_EXP:+ (GOEXPERIMENT=$LINT_EXP)}...\033[0m"
			GOOS=$LINT_OS GOEXPERIMENT=$LINT_EXP "$GOLANGCI_LINT" run
		done
	done
fi

if [ "$TEST"x == "1x" ]; then
	TEST_CGO=0
	if [ -n "$RACE" ]; then
		echo -e "\033[33mTesting with -race enabled...\033[0m"
		if [ "$HOST_OS" != "darwin" ]; then
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

# The packager is installed last, so that a failure in any step above leaves the previously installed one in place.
if [ "$BUILD_GO"x == "1x" ]; then
	echo -e "\033[33mInstalling upack...\033[0m"
	go install -v ./cmd/upack
fi
