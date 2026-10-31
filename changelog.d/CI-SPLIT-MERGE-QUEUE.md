### Added

- CI: a fast stage for pull requests and the full suite in GitHub's merge queue. With the merge
  queue of `main` (`tools/ci/merge_queue.py apply`) a pull request runs the gates, the security
  checks and the tests of its changed components only, with the platform jobs narrowed to the
  platform folders it touches (or one desktop and one mobile job for a Dart change); the queue
  runs every job once per batch of up to five pull requests before they land.
