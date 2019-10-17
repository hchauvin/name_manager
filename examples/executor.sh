#!/usr/bin/env bash

function name_manager {
  ./bin/name_manager "$@"
}

function execute_test {
  local test=$1

  local stack_name
  stack_name=$(name_manager acquire stack)
  set +e
  STACK_NAME=$stack_name eval "$test"
  rc=$?
  set -e
  name_manager release stack "$stack_name"
  return $rc
}

function execute_tests {
  local concurrency=$1
  shift
  local tests=("$@")

  local tests_length=${#tests[@]}
  local chunk_length=$(((tests_length + concurrency - 1) / concurrency))

  test_idx_start=0
  pids=""
  for ((chunk_idx = 0; chunk_idx < concurrency; chunk_idx++)); do
    _execute_test_chunk "${tests[@]:$test_idx_start:$chunk_length}" &
    pids+=" $!"
    test_idx_start=$((test_idx_start + chunk_length))
  done

  failed=false
  for p in $pids; do
    if ! wait $p; then
      failed=true
    fi
  done

  if $failed; then
    echo >&2 "ERRORS: Some tests failed"
    return 1
  fi
}

function _execute_test_chunk {
  local tests=("$@")

  local tests_length=${#tests[@]}

  for ((test_idx = 0; test_idx < tests_length; test_idx++)); do
    execute_test "${tests[$test_idx]}"
  done
}
