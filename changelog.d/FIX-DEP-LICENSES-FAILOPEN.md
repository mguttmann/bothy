### Security

- The dependency license step of `make licenses` no longer excuses a failing scan because a
  Gradle lock file lists packages that do not ship: such a package explains the scanner's exit
  code 1 only by a license violation or a vulnerability of its own, and a failing scan that
  nothing in its report explains fails the step. The pinned scanner names only packages with a
  finding, so no run is known to have passed wrongly.
