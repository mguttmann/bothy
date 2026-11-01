### Changed

- The flake list issue ages: a flaky test not seen for 7 days moves to a collapsed section
  "No longer seen" with its counts, and when no test is left in the table the hourly `flakes`
  workflow closes the issue with a comment; the next flake opens a new issue.
