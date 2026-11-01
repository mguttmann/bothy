### Security

- `POST /api/v1/auth/step-up/password` no longer counts a refusal of the login rules as a failed
  step-up: while the password path of the account is locked it answers `401 step_up_required`
  with reason `password_locked`, for a password changed during the attempt with reason
  `password_changed`. Neither fails the challenge or counts toward `auth.step_up_fail_limit`,
  so a lockout caused by wrong logins can no longer revoke the session. The refusal is audited
  as `auth.step_up` deny with cause `locked` or `credential_changed`.
