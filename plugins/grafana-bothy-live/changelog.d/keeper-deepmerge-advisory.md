### Security

- The advisory gate ignores GHSA-gcqx-w48m-w2h4 (deepmerge 4.3.1, no fixed release) in the
  Grafana panel plugin until 2026-11-18: deepmerge is a dev dependency of jest and the type
  check and never ships. The ignore of the withdrawn GHSA-hp3w-g68c-fv3c (sprintf-js) is gone.
