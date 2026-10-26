### Fixed

- The platform suite opens sessions only after the report that ends every entry scan of the
  start (`waitReady`), so an opening no longer races a running scan and ends with
  `shared_location`; new `TestPlatform_OpenAfterTheEntryScan`.
