# Run the demo

Status: built M7-27. Rules: [16-operations.md](../spec/16-operations.md) section 16. The data set:
[infra/demo/README.md](../../infra/demo/README.md).

The demo instance shows every view of the product with made-up, white-label data: the people
Alex, Sam and Robin, the spaces Support, Marketing, Engineering and Research, agents with runs, an
approval card that waits, costs, the statistics of [25-visualization.md](../spec/25-visualization.md)
and a terminal on a demo host. No model is called, no external service is used and nothing is
published. The showcase (M7-28) is produced from it: `make showcase PLATFORM=<platform>`
([tools/showcase/README.md](../../tools/showcase/README.md)).

## 1. On a developer machine: `make demo`

Needs Docker Engine 26 or newer with Compose 2.26 and buildx, `uv` (`make bootstrap`), `openssl`,
`curl` and the server image of `make image`.

```sh
make image
make demo
```

`infra/compose/demo.sh` starts the stack (the installation file `compose.yaml` with the smoke and
the demo overlay), runs `bothy setup demo` in the `api` container, pairs the host service container
when the command asks for it and prints:

- the address `https://localhost:<port>`, published on `127.0.0.1` only;
- the certificate of the demo CA of this run: trust it in the app or the browser for this run;
- the enrollment code of Alex, the owner: sign in with it in the app (it is valid once).

The warm-up posts prompts at the pace of the data set, so the command takes a few minutes; the
views fill while it runs. Ctrl-C stops the demo and removes the stack, its volumes and the
temporary folder.

Inputs: `DEMO_SEED` (default `1`; the same seed gives the same content), `DEMO_PACE` (a Go
duration, default the value of the data set), `DEMO_SERVER_IMAGE` (default the tag of
`make image`), `DEMO_PORT` (the port of `https://localhost:<port>`, default a free port; a port
in use on `127.0.0.1` is a missing precondition), `DEMO_KEEP=1` (leave the stack running and print
how to remove it), `DEMO_CHECK_RESTART=1` (see below), `DEMO_WAIT_TIMEOUT_S`, `DEMO_LOG_TAIL`.
Exit code `2` names a missing precondition.

Restarts: `api`, the TLS front, the demo agent and the demo host share one network namespace,
which the container `demo-net` holds; it runs nothing that can end. A restart of `api` (by hand or
by its restart policy) leaves the others where they are, and every service of the demo restarts
unless stopped, so a restart of Docker or of its VM brings the demo back without a manual step.
`DEMO_CHECK_RESTART=1 make demo` checks this: once the demo is ready it restarts `api` with
`docker restart`, waits until discovery answers again at the address and in the demo agent and
until the demo host reaches the server on its loopback, then removes the demo (exit `0`, or `1` when the demo does not come back).

## 2. On an installation

`bothy setup demo` runs in every environment, but only on an empty instance: an instance whose
setup is complete gives exit `3` and nothing is written. Without the overlay there is no demo agent
and no host container:

```sh
docker compose exec api bothy setup demo --data /path/in/the/container
```

Without `--exchange` the command creates no agent token and leaves the host in `pairing`; the
people, spaces, threads, agents, prices and the warm-up prompts are there, the runs stay queued.
Mount `infra/demo` into the container (`/usr/share/bothy/demo` is the default of `--data`).

## 3. Checks

- `make demo-e2e`: the integration test `TestDemoViews` (needs Go, Docker and `uv`): the command on
  an empty instance of the test harness with the demo agent and the Fake-Host of the terminal
  tests; every view of 25 has data through its query, a watcher run is running with a live box,
  an approval card waits, the warm-up booked its costs and a terminal session opens on the host.
- `TestDemoDeterministic`, `TestDemoRefusesNonEmpty` and `TestDemoOnlyUseCases` run with the Go
  tests of the server ([../quality/gates.md](../quality/gates.md)).
- `make denylist` and `make gitleaks` cover `infra/demo/`.
