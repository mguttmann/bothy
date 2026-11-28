# Open minors of FIX-adapter-install-keeps-disabled

Collected minors without a fix in FIX-adapter-install-keeps-disabled; the next run in the Hermes
adapter's installer reads and takes them along.

- The summary line `platform enabled:` of `installer._summary` reads `enabled` with `is True`,
  while Hermes reads it with `_coerce_bool` (`gateway/config.py`): a hand-written quoted
  `enabled: "true"` shows `no (kept off ...)` although the gateway connects. Only the display is
  off; the install rule itself is right, and unquoted YAML `true` or `yes` is a bool already.
