# tools/observability

Generators of the observability sources and the alert path test: `sli_bounds.py`, the SLI bounds
from the Sloth files (docs/spec/25-visualization.md 3.3, R-VZ-03, VZ-T03), and `alert_e2e.py`,
the runner of `make alert-e2e` (M7-20).

## `sli_bounds.py`

Targets and thresholds of the SLOs live only in `infra/observability/slo/*.yaml`. The generator
reads per SLO the `service/name`, the label `bothy_sli_metric`, the `objective`, the threshold
(the value of `le` in `error_query`, else in `total_query`, in seconds; zero without one) and the
period (`slo_period` of the file, else the Sloth default of 30 days) and writes:

| Output | Content |
|---|---|
| `services/server/internal/stats/domain/slibounds.gen.go` | sorted map from the `sli.*` metric to SLO, objective, threshold and period; the own measurement counts `good` with the threshold, the traffic light uses objective and period |
| `packages/bothy_viz/lib/src/slo/slo_catalog.g.dart` | sorted list of SLO, metric and l10n key; no thresholds, because the server delivers the values in `getStatsSlo` |

Both files carry a `DO NOT EDIT` header naming the generator.

Label rules, checked by the generator and by the check `sli-labels` of `make ops-check`: every
SLO has exactly one label `bothy_sli_metric`; its value is one of the `sli.*` values of the
`CHECK` of `stats_minutes.metric` in docs/spec/03-data-model.md (read with a narrow parser);
every such value belongs to exactly one SLO. A missing label, an unknown metric, two SLOs with the
same metric and an `sli.*` metric without an SLO each fail with a message.

```sh
uv run --project tools --locked --quiet python tools/observability/sli_bounds.py --write
uv run --project tools --locked --quiet python tools/observability/sli_bounds.py --check
```

`--check` generates into a temporary folder and prints a unified diff on drift (exit 1); the
check `sli-bounds` of `make ops-check` runs it. After a change to a Sloth file, regenerate the
Prometheus rules with the pinned `sloth` (the command in the file header) and run `--write`.

Tests: `uv run --project tools --locked pytest tools/observability/tests`.

## `alert_e2e.py`

The end-to-end test of the alert path (brief M7-20 Design note 2,
[alerting.md](../../docs/observability/alerting.md) section 5). It starts the Compose smoke stack
from the server image of `make image` (or `E2E_SERVER_IMAGE`) with the profile `observability`
and the overlay `infra/compose/test/compose.alert-e2e.yaml`, checks that Alertmanager refuses to
start without its URL files, bootstraps the owner, lets `bothy-smoke alerts` set up the channel
"Operations" with step-up, writes the URLs with `infra/compose/alert-setup.sh`, starts
Alertmanager and Prometheus with the test rules and waits for the firing and the resolved
messages; then it checks the records of the stub. The stack, the probe image and the temp
directory are always removed (`E2E_KEEP=1` keeps them); `E2E_ARTIFACTS` names a directory for the
recorded messages and stub records. No webhook URL is printed. Exit codes: 0 pass, 1 a check
failed, 2 a precondition is missing.

```sh
make alert-e2e
```

Tests without Docker: `tools/observability/tests/test_alert_e2e.py`; the setup script and the
start guard of Alertmanager: `tools/observability/tests/test_alert_setup.py`.
