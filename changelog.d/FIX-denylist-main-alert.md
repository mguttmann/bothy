### Fixed

- A red deny-list scan of `main` no longer goes unnoticed: the failed run opens an issue with the
  label `denylist-main`, or comments on the open one, naming the run, the commit, finding counts
  per kind and masked locations, never a matched term or rule. A manual run scans the tree of
  `main` plus only the commits of an optional input range, not the whole history.
