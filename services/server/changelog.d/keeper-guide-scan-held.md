### Fixed

- The guide "Connect your own agent" no longer shows `mime_type` in the answer of `getMessage`
  in step 9: the server sets the detected type only after its scan, and the scan may still run
  when the reader (or `TestGuide`) asks, so the answer held `application/octet-stream` (flaky
  `TestGuide`). The text explains the two values. `TestGuide` now holds the scan of that file for
  the whole run (`clamdtest.Script.StallScans`, with `file.scan.timeout` above the run), so no
  block can depend on when the scan ends.
