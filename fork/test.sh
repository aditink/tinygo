#!/bin/sh
# fork/test.sh TINYGOROOT [GO_BIN] — build (and, if mode=run, run) every fork/tests/<id>/ program
# with the given release root. Each test dir has a `mode` file: `ll` = must compile to LLVM IR
# (the program needs a later fork commit to link), `run` = must build, run, exit 0 and print a
# line starting with "fork-". An optional `env` file holds VAR=value lines exported for that
# test's build. Builds use -tags netgo (pure-Go resolver: no host -lresolv).
set -eu
root=${1:?usage: fork/test.sh TINYGOROOT [GO_BIN]}
gobin=${2:-}
here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
[ -n "$gobin" ] && PATH=$gobin:$PATH
export PATH TINYGOROOT=$root GOTOOLCHAIN=local GOENV=off
fail=0
for d in "$here"/tests/*/; do
  id=$(basename "$d"); mode=$(cat "$d/mode")
  envs=; [ -f "$d/env" ] && envs=$(cat "$d/env")
  for sch in tasks threads; do
    case $mode in
      ll)  (cd "$d" && env $envs "$root/bin/tinygo" build -tags netgo -scheduler=$sch -o "$out/$id-$sch.ll" .) \
             && echo "PASS $id ($sch): compiles to IR" || { echo "FAIL $id ($sch): build"; fail=1; } ;;
      run) if (cd "$d" && env $envs "$root/bin/tinygo" build -tags netgo -scheduler=$sch -o "$out/$id-$sch" .); then
             if r=$("$out/$id-$sch" 2>&1) && printf '%s\n' "$r" | grep -q '^fork-'; then echo "PASS $id ($sch): $(printf '%s\n' "$r" | grep '^fork-')"
             else echo "FAIL $id ($sch): run: $r"; fail=1; fi
           else echo "FAIL $id ($sch): build"; fail=1; fi ;;
      *) echo "FAIL $id: bad mode '$mode'"; fail=1 ;;
    esac
  done
done
exit $fail
