### Security

- The advisory gate `make osv` passes again after four advisories published on 2026-11-10 for
  node-forge before 1.4.3 (GHSA-6889-vf6x-f3g5, GHSA-6cv4-p8m7-4r25, GHSA-j8rw-jp99-435v and
  GHSA-w4xr-hqv8-ch44): the Node development tools override node-forge to 1.4.3. No finding is
  accepted, and a test holds every override of these tools to the version in their lock file.
