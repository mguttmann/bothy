# CI-SPLIT-MERGE-QUEUE Fast pull request checks and the full suite in the merge queue

Owner request of 2026-10-31: pull requests wait for hours because each runs the full suite of
every platform on the few macOS runners; split the CI and run the full suite once per batch in
GitHub's merge queue.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-CSMQ-1 | 2026-10-31 | `ci.yml` has a fast stage for pull requests (`ci-plan --stage pr`) and a full stage for `merge_group` (`--stage queue`: every job of a push to main plus the trials of the diff); a push to `main` keeps the full run. | The full suite runs once per batch before the merge instead of once per pull request; the watcher keeps its run on `main`. |
| D-CSMQ-2 | 2026-10-31 | The fast stage narrows only the platform jobs: a native folder runs its own platform (`ios/` also macOS, which runs ClientKit; `android/` also `test (apps)` for the JVM tests); other Dart changes and shared inputs run `apps (Windows)` and `apps (Android API 36)`; dependency manifests, the lock file and `platform-tests.json` run all five. | The platform jobs are the slow and scarce ones; dependencies reach every native build, so failing those early is cheaper than a split batch. |
| D-CSMQ-3 | 2026-10-31 | The stages are switched by the repository variable `BOTHY_MERGE_QUEUE`, which `tools/ci/merge_queue.py apply` sets only after the ruleset exists and `disable` resets first. | Without a queue behind it the fast stage would let pull requests land without the full suite. |
| D-CSMQ-4 | 2026-10-31 | Ruleset: squash, groups of 1 to 5, two groups built at a time, only the head of a group must pass (`HEADGREEN`), timeout 360 minutes; the required check stays `ci-ok` of the branch protection. | Two builds hold four of the five macOS jobs of a free account, the push to `main` needs the rest; the contract test may take 240 minutes. |
| D-CSMQ-5 | 2026-10-31 | `sonar` does not run in the merge queue; the queue compares flakes and advisories with the base of the group. | The pull request's scan is the gate of its new code; a new advisory that `main` has too must not stop unrelated pull requests (the daily scan and the push to `main` report it). |
| D-CSMQ-6 | 2026-10-31 | The repository belongs to a personal account, for which GitHub offers no merge queue (D-W-34; the ruleset API answers 422 "Invalid rule 'merge_queue'"). The workflow and the tool are ready; the queue starts when the repository moves to an organization and `merge_queue.py apply` runs. Until then pull requests keep the full plan. | Owner decision: moving the repository changes its owner and URL. |
