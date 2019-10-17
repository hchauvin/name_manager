#!/usr/bin/env bash

set -eou pipefail

source "examples/executor.sh"

function docker_compose_test {
  local idx=$1
  local stack_name=$2

  local docker_compose_project_name=stack_$stack_name
  docker-compose -p "$docker_compose_project_name" -f examples/docker-compose.yml up -d
  
  local address
  address=$(docker-compose -p "$docker_compose_project_name" -f examples/docker-compose.yml port server 80)
  echo "<Actual test placeholder: $idx for stack $stack_name; server address: $address>"
}

TESTS=(
  "docker_compose_test 1 \$STACK_NAME"
  "docker_compose_test 2 \$STACK_NAME"
  "docker_compose_test 3 \$STACK_NAME"
  "docker_compose_test 4 \$STACK_NAME"
  "docker_compose_test 5 \$STACK_NAME"
)
execute_tests 3 "${TESTS[@]}"

echo ""
echo "Stacks:"
name_manager list
