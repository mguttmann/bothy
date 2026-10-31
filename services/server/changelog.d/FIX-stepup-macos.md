### Changed

- `POST /api/v1/auth/step-up/password` also confirms the step-up of an app session whose
  device has no active `step_up` key (the Windows app), with password and TOTP and
  `step_up_method = password_totp`; such a session gets no cookie. A device with a `step_up`
  key, a share extension session and a `cli` device still get `403 forbidden` with reason
  `step_up_not_possible`. The OIDC re-login stays console-only.
