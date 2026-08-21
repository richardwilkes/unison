#! /usr/bin/env bash

# Collects the benchmark and gate data needed to settle the goexperiment.simd dispatch preferences on this machine.
# Run it from the repo root on the `simd` branch. It writes everything into ./simd-bench-results and packs that
# directory into simd-bench-results.tgz.
#
# A full run takes a couple of minutes on current hardware and stays well under ten on older silicon. Use --smoke to
# shake out the plumbing in a few seconds; a smoke run produces no usable timings.

set -eo pipefail

trap 'echo -e "\033[33;5msimd-bench failed on simd-bench.sh:$LINENO\033[0m"' ERR

SMOKE=
for arg in "$@"; do
	case "$arg" in
	--smoke | -s)
		SMOKE=1
		;;
	--help | -h)
		echo "$0 [options]"
		echo "  -s, --smoke Run every step with minimal iteration counts to verify the script itself"
		echo "  -h, --help  This help text"
		exit 0
		;;
	*)
		echo "Invalid argument: $arg" >&2
		exit 1
		;;
	esac
done

OUT=simd-bench-results
rm -rf "$OUT" simd-bench-results.tgz
mkdir -p "$OUT"
echo '*' >"$OUT/.gitignore"

echo "== system info"
{
	if [ -n "$SMOKE" ]; then
		echo "SMOKE RUN — iteration counts were cut to the minimum; the timings here are not data."
	fi
	date
	go version
	echo "GOOS=$(go env GOOS) GOARCH=$(go env GOARCH)"
	if [ "$(uname -s)" = "Darwin" ]; then
		sysctl -n machdep.cpu.brand_string 2>/dev/null || true
	else
		grep -m1 'model name' /proc/cpuinfo 2>/dev/null || true
	fi
	git rev-parse HEAD
} >"$OUT/sysinfo.txt"
cat "$OUT/sysinfo.txt"

# The wiring tests show which kernels this CPU supports and which ones init actually dispatched: a PASS means the
# preference table matched the hardware, a SKIP names the gate that declined.
echo "== gates"
GOEXPERIMENT=simd go test ./internal/pixconv/ ./internal/f32/ -run 'SIMDWiring' -v -count=1 >"$OUT/gates.txt" 2>&1 || {
	echo "wiring tests failed; see $OUT/gates.txt" >&2
	exit 1
}
grep -E -- '--- (PASS|SKIP|FAIL)' "$OUT/gates.txt" || true

# Bit-exactness on this hardware before any timing. The root package is in here because it is what actually consumes
# the kernels: text layout sums rune widths with internal/f32 and the presentation paths convert frames with
# internal/pixconv.
echo "== correctness (GOEXPERIMENT=simd)"
GOEXPERIMENT=simd go test ./internal/pixconv/ ./internal/f32/ ./ -count=1 >"$OUT/correctness.txt" 2>&1 || {
	echo "correctness failed; see $OUT/correctness.txt" >&2
	exit 1
}
cat "$OUT/correctness.txt"

# Groups that produced benchmark data, in run order, for the benchstat pass below. (Not named GROUPS: bash owns that
# name and silently discards assignments to it.)
BENCH_GROUPS=()

bench() {
	local name=$1 pkg=$2 pattern=$3 count=$4 benchtime=$5
	if [ -n "$SMOKE" ]; then
		count=1
		benchtime=10x
	fi
	echo "== bench $name (default)"
	go test "$pkg" -run XXX -bench "$pattern" -count "$count" -benchtime "$benchtime" >"$OUT/${name}_default.txt" 2>&1 || {
		echo "bench $name failed; see $OUT/${name}_default.txt" >&2
		exit 1
	}
	echo "== bench $name (GOEXPERIMENT=simd)"
	GOEXPERIMENT=simd go test "$pkg" -run XXX -bench "$pattern" -count "$count" -benchtime "$benchtime" \
		>"$OUT/${name}_simd.txt" 2>&1 || {
		echo "bench $name failed; see $OUT/${name}_simd.txt" >&2
		exit 1
	}
	# `go test -bench` succeeds when the pattern matches nothing at all, so a group whose benchmarks are not in this
	# tree leaves two passing-but-empty logs behind. Detect that here and drop the group rather than handing benchstat
	# a file with no measurements in it.
	if grep -qE '^Benchmark' "$OUT/${name}_default.txt" && grep -qE '^Benchmark' "$OUT/${name}_simd.txt"; then
		BENCH_GROUPS+=("$name")
	else
		echo "   no benchmarks matched '$pattern' in $pkg; skipping this group"
	fi
}

# The iteration counts are fixed rather than time-based so both builds measure exactly the same work. One pixconv
# iteration converts a whole 1920x1080 frame (~2M pixels, 1-2ms), so a couple of hundred of them is already a stable
# sample; the f32 sums are nanoseconds to sub-microsecond and need the large counts to rise above timer and loop
# overhead. The text benchmarks are the exception: they are added separately, so they get a time bound instead of an
# iteration count that could be wildly wrong for them.
bench pixconv ./internal/pixconv/ 'BenchmarkSwizzleRB$|BenchmarkSwizzleRBBytes$|BenchmarkRGBAToBGRABytes$|BenchmarkRGBAToARGBBytes$|BenchmarkPremulBGRA$|BenchmarkPremulARGB$' 10 200x
bench sumshort ./internal/f32/ 'BenchmarkSum/n=(8|64)$' 10 50000x
bench sumlong ./internal/f32/ 'BenchmarkSum/n=1024$' 10 3000x
bench text ./ 'BenchmarkTextCacheWidths$|BenchmarkTextPositionForRuneIndex$' 10 300ms

# Render benchstat comparisons when the tool is available; the raw files above are the data of record either way.
if command -v benchstat >/dev/null 2>&1; then
	BS=benchstat
else
	TOOLS_DIR=$(go env GOPATH)/bin
	if [ ! -x "$TOOLS_DIR/benchstat" ]; then
		echo "== installing benchstat into $TOOLS_DIR"
		go install golang.org/x/perf/cmd/benchstat@latest || true
	fi
	if [ -x "$TOOLS_DIR/benchstat" ]; then
		BS="$TOOLS_DIR/benchstat"
	else
		BS=""
	fi
fi
if [ ${#BENCH_GROUPS[@]} -eq 0 ]; then
	echo "no benchmarks ran; nothing to compare" >&2
elif [ -n "$BS" ]; then
	for name in "${BENCH_GROUPS[@]}"; do
		"$BS" "$OUT/${name}_default.txt" "$OUT/${name}_simd.txt" >"$OUT/${name}_benchstat.txt" 2>&1 || true
	done
	echo "== benchstat summaries written"
else
	echo "benchstat not found; raw files are enough — or: go install golang.org/x/perf/cmd/benchstat@latest"
fi

tar czf simd-bench-results.tgz "$OUT"
echo
if [ -n "$SMOKE" ]; then
	echo "Smoke run complete. The timings in $OUT are not data; re-run without --smoke to collect real numbers."
else
	echo "Done. Send back simd-bench-results.tgz (or the $OUT directory)."
fi
