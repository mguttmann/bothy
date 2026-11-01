### Fixed

- Server tests no longer fail at random with "JSON null in ..." when a random token in a
  response or event payload holds the letters `null` (issue #238, `TestIdentityLink`): they find
  a JSON null by decoding the JSON (`testkit.HasJSONNull`, `testkit.NoJSONNull`) instead of
  searching the text, and the Semgrep rule `bothy-go-test-null-substring` refuses the text
  check. Test only, no product change.
