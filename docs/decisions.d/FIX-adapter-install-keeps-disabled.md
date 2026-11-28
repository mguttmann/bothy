# FIX-adapter-install-keeps-disabled: `install` keeps a parked Hermes profile off

Reported from operation on 2026-11-28: a Hermes profile of an operator, parked on 2026-11-27 with
`gateway.platforms.bothy.enabled: false`, had `enabled: true` again after the adapter's `install`
of the update to 2257e3466.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-adapter-install-keeps-disabled-1 | 2026-11-28 | Cause: `plan_install` set `section["enabled"] = True` on every run, whatever the section held. Reproduced with the profile's backup of `config.yaml` taken right before that install (`enabled: false`): `plan_install` of main returned `enabled: True`. | Read from the code and run against the real backup, in memory only; the profile was not changed. |
| D-FIX-adapter-install-keeps-disabled-2 | 2026-11-28 | `install` writes `enabled: true` only when the section has no `enabled` key (a first install) or `--enable` is given; any existing value stays. The summary prints `platform enabled: yes` or `platform enabled: no (kept off; install --enable switches it on)`. `plugins.enabled` still lists the platform key. | An operator's `enabled: false` is a decision, and an adapter update must not undo it. The gateway connects only platforms whose `enabled` is true (`gateway/config.py` of the operator's installed Hermes Agent, read on 2026-11-28), so a loadable but disabled plugin stays quiet; `check` keeps reporting the plugin as loadable. Hermes' own `persist_home_channel` follows the same rule (`setdefault("enabled", True)`). An explicit switch keeps switching a parked profile on a deliberate act. |
