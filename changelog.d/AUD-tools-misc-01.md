### Security

- The deny-list scan (`make denylist`, the pre-push hook, the pull request jobs and
  `denylist-main`) reads the files of every commit it checks and not only the final state: a
  term that one commit adds and a later commit of the same push removes, deletes or renames
  away now fails the scan, with the commit named. A later commit does not take an earlier one
  back from a public repository.

### Fixed

- A push of the kind `wake_ended` (a wake-up that ended without firing, or a run that ended at
  the usage limit of the account) has its text in the apps. iOS and macOS showed the key
  `PUSH_BODY_WAKE_ENDED` instead of a text, Android the text of a new message. A test now holds
  the push texts and the lists of kinds in the native code to the kinds of the API.
