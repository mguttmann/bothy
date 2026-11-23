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
- the certificate of the demo CA of this run (valid for two days, a new one each run);
- the enrollment code of Alex, the owner (valid once).

To look at it:

1. Trust the demo CA in the trust store of the system that runs the app or the browser (on
   macOS the keychain, on Windows the certificate store, on Android the system store of the
   emulator). The app has no setting to trust a CA of its own; without the system trust it shows
   "Certificate invalid". Remove the CA again when the demo ends.
2. Build and start the app ([apps/client](../../apps/client/README.md)) for macOS, Windows, iOS or
   Android, enter the address and redeem the enrollment code of Alex. There is no Linux desktop
   app: on Linux the app runs in the Android emulator. The certificate names only `localhost`
   and `127.0.0.1`, so map the port into the emulator with `adb reverse tcp:<port> tcp:<port>`
   and use the printed address there.
3. The web console at the same address does not take an enrollment code: it signs in with a
   password and TOTP ([08-identity.md](../spec/08-identity.md) 7.5), which `make demo` does not
   set. The showcase creates them with `bothy-smoke seed`
   ([tools/showcase/README.md](../../tools/showcase/README.md), platform `console`).

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
and no host container (in the profile `tailscale` the service is `api-tailscale`):

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
