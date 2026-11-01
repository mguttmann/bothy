### Fixed

- Step-up actions work again in the Windows app, which has no step-up key by design: the app
  now asks for the password and the 6-digit code of the authenticator app in the confirmation
  dialog and sends them to the server, instead of failing with "The confirmation was
  cancelled" before any request. A wrong password or code shows the dialog again with an error
  line. iOS, Android and macOS keep confirming with the device key; the terminal still needs a
  device key or a passkey and stays unavailable on Windows. The password step-up needs a local
  account with TOTP and local passwords switched on (`auth.local_password_enabled`); without
  them the action stays unavailable on Windows.
