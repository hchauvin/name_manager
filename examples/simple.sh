#!/usr/bin/env bash

set -eou pipefail

source "examples/executor.sh"

TESTS=(
  "echo Test 1 with stack \$STACK_NAME"
  "echo Test 2 with stack \$STACK_NAME"
  "echo Test 3 with stack \$STACK_NAME"
  "echo Test 4 with stack \$STACK_NAME"
  "echo Test 5 with stack \$STACK_NAME"
)
execute_tests 2 "${TESTS[@]}"

echo ""
echo "Stacks:"
name_manager list
