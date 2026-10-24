### Fixed

- The server and postgres image builds retry a download that fails on a transient error of the
  Go module proxy or the restic release host, instead of failing the image job.
