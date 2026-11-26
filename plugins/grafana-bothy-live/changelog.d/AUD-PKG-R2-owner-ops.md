### Security

- The nested copy of i18next 19.9.2 that `i18next-pseudo` pulled in is replaced by the i18next
  26.5.2 the plugin already uses (an npm override for versions below 26.3.4), which fixes
  GHSA-6jcc-5g8w-32mx.
