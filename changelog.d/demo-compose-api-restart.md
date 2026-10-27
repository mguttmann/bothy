### Fixed

- Demo instance: a restart of `api` (or of Docker) no longer leaves the demo unreachable. The TLS
  front, the demo agent and the demo host shared the network namespace of `api` and stayed in the
  old one after a restart; the namespace is now held by the container `demo-net` of
  `infra/compose/demo.yaml`, and every long-running service of the demo restarts unless stopped.
  `infra/compose/demo.sh` takes a fixed port (`DEMO_PORT`) and checks a restart of `api` on
  request (`DEMO_CHECK_RESTART=1`).
