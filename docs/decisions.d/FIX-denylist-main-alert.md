# FIX-denylist-main-alert Alert on a red deny-list run of main

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-denylist-main-alert-1 | 2026-11-07 | A manual run without input scans the tree of `main` and no commits. An input range must be two existing commit ids (`BASE..TIP`); anything else makes the run red. | A manual run over the whole history would be red on old public commits that cannot be changed; a strict input form keeps options and revision expressions away from git. |
| D-FIX-denylist-main-alert-2 | 2026-11-07 | A failed run on `main` comments on the open issue with the label `denylist-main` instead of opening a second one; a rejected input still opens or comments an issue, without counts. | One open issue per problem keeps the alert readable; a rejected input is a red run of `main` and must not stay silent. |
