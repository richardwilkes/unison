#! /usr/bin/env bash

set -eo pipefail

trap 'echo -e "\033[33;5mBuild failed on build.sh:$LINENO\033[0m"' ERR

GOLANGCI_LINT_VERSION=1.54.0

for arg in "$@"
do
  case "$arg" in
    --all|-a) LINT=1; TEST=1; RACE=-race ;;
    --lint|-l) LINT=1 ;;
    --race|-r) TEST=1; RACE=-race ;;
    --test|-t) TEST=1 ;;
    --help|-h)
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

# Build the Go code
echo -e "\033[33mBuilding Go code...\033[0m"
go build -v ./...

# Run the tests
if [ "$TEST"x == "1x" ]; then
  if [ -n "$RACE" ]; then
    echo -e "\033[33mTesting with -race enabled...\033[0m"
  else
    echo -e "\033[33mTesting...\033[0m"
  fi
  go test $RACE ./...
fi

# Run the linters
if [ "$LINT"x == "1x" ]; then
  TOOLS_DIR=$PWD/tools
  if [ ! -e "$TOOLS_DIR/golangci-lint" ] || [ "$("$TOOLS_DIR/golangci-lint" version 2>&1 | awk '{ print $4 }' || true)x" != "${GOLANGCI_LINT_VERSION}x" ]; then
    echo -e "\033[33mInstalling version $GOLANGCI_LINT_VERSION of golangci-lint into $TOOLS_DIR...\033[0m"
    mkdir -p "$TOOLS_DIR"
    curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$TOOLS_DIR" v$GOLANGCI_LINT_VERSION
  fi
  echo -e "\033[33mRunning Go linters...\033[0m"
  $TOOLS_DIR/golangci-lint run
fi
