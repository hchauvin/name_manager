# `name_manager`: Utility to manage shared test resources with a global lock

Tests should ideally set up and tear down all the resources they use.
However, sometimes these resources are too expensive and they must be shared.
For instance, instead of using a brand new database for each test, one can
reuse the same database and truncate the tables between the tests.  If
one wants to run tests concurrently, two by two, one can create two databases
and reuse them in the same way.  `name_manager` can be used to share such
test resources between processes and to avoid race conditions where two
concurrent tests end up using the same shared resource.

`name_manager` has an obvious application for the concurrent allocation of
shared test resources, as explained above, but its aim is more generic:
it is a distributed system, with a global lock, to retrieve unique names while
ensuring that previously acquired names, now released, are reused as much
as possible.

`name_manager` can be either consumed as a Go package through a standalone
Command-Line Interface (CLI).

## Example use

By default, the `name_manager` uses the local file system as an
inter-process lock.

As an example use, let's pretend we have a bunch of end-to-end tests on
a webapp.  These tests run on a full app that is deployed with
`docker-compose`.  Deploying with `docker-compose` is expensive, so we
start by using a single docker-compose project and executing the tests
serially:

```bash
# Creates the shared resource
docker-compose -p stack -f docker-compose.yml up -d

DOCKER_COMPOSE_PROJECT_NAME=stack test_1
DOCKER_COMPOSE_PROJECT_NAME=stack test_2
DOCKER_COMPOSE_PROJECT_NAME=stack test_3
DOCKER_COMPOSE_PROJECT_NAME=stack test_4

# Destroys the shared resource
docker-compose -f docker-compose.yml down
```

Note that, because the tests clean up the shared resource before use,
`docker-compose down` is not necessary between invocations of the shell
script above and will be omitted from now on.

If the tests all take the same time to complete, and we have enough
system resources, we can more or less halve the execution time, outside of
the setup/teardown steps, by providing two stacks and executing the tests
two by two:

```bash
# Ensures the shared resources are up
docker-compose -p stack1 -f docker-compose.yml up -d &
docker-compose -p stack2 -f docker-compose.yml up -d &
wait

(
  DOCKER_COMPOSE_PROJECT_NAME=stack1 test_1 &&
  DOCKER_COMPOSE_PROJECT_NAME=stack1 test_2
) &
(
  DOCKER_COMPOSE_PROJECT_NAME=stack2 test_3 &&
  DOCKER_COMPOSE_PROJECT_NAME=stack2 test_4
) &
wait
```

`name_manager` enables a generalization of this reasoning.  See the
`./examples` directory for examples.
