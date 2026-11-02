### Fixed

- The operator of a channel may upload files without a grant (a fixed thread operator that is not
  the channel's operator still needs a channel grant, because uploads are decided on the channel):
  the operator rule gives `upload` together with `write` and takes both away when the operator
  changes. Before, an agent that sends files (answer attachments, media) had its files refused
  with `403 forbidden` and `capability_missing` until the owner added a grant with `upload` by
  hand. Existing operators get it with the update; grants added by hand stay (07 2.9, R-AZ-13).
