# Own sign-in methods in the apps and the console

Status: design of 2026-11-26 for the plan row V1-14, built by the same ticket. This note explains
the decisions; the normative text is in [01-product.md](../spec/01-product.md) 5.1 (D8), 5.5 and
5.6 (E3), [08-identity.md](../spec/08-identity.md) 2.5, 6.2 and 6.3 and
[24-terminal.md](../spec/24-terminal.md) 12. Rulings: [decisions.d/V1-14.md](../decisions.d/V1-14.md).
Issues #1426 and #677; owner decision of 2026-11-25.

## 1. Goal

A person sets up their own sign-in methods without a third-party tool: a password with a one-time
code (TOTP, with a QR code and a confirmation), a passkey and a link with a further login service.
The server has had the operations since M1 (`getMyCredentials`, `setMyLocalCredential`,
`beginPasskeyRegistration` and `completePasskeyRegistration`, `beginIdentityLink` and
`completeIdentityLink`); no view called them. The Windows app gets a step-up with a passkey
through Windows Hello, so its terminal opens (#677).

Done means:

1. Settings, "Sign-in" (D8 on the desktop, the same section on the phone) and the console page
   "Sign-in" list the own linked identities, passkeys and the state of the password and the
   one-time code, and offer each setup the platform can do.
2. A password with TOTP is set in two steps: password and a new secret (QR code and the secret as
   text), then a code from the authenticator app confirms it.
3. A passkey is registered in the console (browser WebAuthn) and in an app that has a passkey
   port: the Windows app over Windows Hello, a phone build with a public domain over the system.
4. A further OIDC identity is linked from the console (redirect back to the console) and from the
   apps (system browser and the redirect of the login).
5. The Windows app confirms a step-up, and the fresh step-up of the terminal, with a passkey;
   without one the terminal shows a hint that names the way to add one instead of a failed
   opening.
6. The hints `consoleStepUpNoMethod` and `consoleTerminalPasskeySetup` name the real way.

## 2. Decisions

1. **One view for both clients.** The section is a widget of `admin_ui` (`MySignInView` over a
   `SignInRepository`), like the own devices and sessions (`MyDevicesView`), and the app and the
   console only pass what their platform can do: a function that creates a passkey (null hides
   "Add passkey") and a function that links an identity at a provider (null hides linking).
   Reason: the console and the apps show the same states and the same errors, and one widget test
   covers both.
2. **Step-up stays the server's.** The view never asks for a step-up itself. The changing calls
   carry a purpose text (`stepUpPurposeExtra`), and the step-up interceptor of the app or the
   console's action step-up answers `401 step_up_required` (`identity.link` always,
   `local_credential.set` and `passkey.register` unless the session has nothing to confirm with,
   05). A cancelled step-up leaves the view as it was.
3. **Password and one-time code in two steps.** Step one sends the password and `totp_enroll`
   (a person with a confirmed code who only changes the password sends the password alone); the
   answer carries the otpauth URI once. The view shows it as a QR code and the secret as text to
   type, and keeps it only in the state of the open dialog. Step two sends `totp_code`; a wrong
   code keeps the dialog open with the secret. The password is typed twice and must match; its
   length and the list of common passwords are the server's rule (`400 validation_failed`).
   The section shows only for a person with a local identity on an instance with
   `auth.local_password_enabled`.
4. **The QR code comes from the package `qr`** (pure Dart, BSD-3-Clause, no platform code); the
   view paints its modules itself. Reason: a QR encoder of our own (Reed-Solomon, masks, version
   choice) is more code to hold than a small, licensed dependency.
5. **Passkeys in the Windows app over Windows Hello.** The runner serves the method channel
   `bothy/webauthn` over the Win32 WebAuthn API (`webauthn.dll`); Dart builds the client data with
   the origin of the instance and the JSON forms the server reads. A Mac has no passkey port
   (D-M4-18-9 stays): it has the step-up key and needs none for the terminal.
6. **Step-up without a step-up key.** On a device without a step-up key the step-up uses a
   passkey when the app has a passkey port and the challenge carries `passkey_options` (the person
   has a passkey); otherwise, and when the platform gives no assertion (a passkey stored on
   another device), password and TOTP as before (08 6.2). The terminal's fresh step-up
   takes the passkey too, because the server counts a passkey as fresh outside the console (08
   6.3 item 2); without a passkey it ends before any request with `StepUpUnavailable`, and the
   terminal shows the hint to add a passkey under Settings, Sign-in.
7. **Linking in the apps reuses the login's redirect.** `LoginService.linkIdentity` runs
   `beginIdentityLink` with the redirect URI and PKCE of the login ports, waits for the redirect
   and calls `completeIdentityLink`. The console sends the tab to the login service with the
   redirect `/console/sign-in/callback` and completes on its return, the verifier in the tab's
   `sessionStorage` as for the login (M6-01).
8. **The console page is an administration page.** Before its first step-up a console session may
   only read its own profile (08 7.5), so the page "Sign-in" needs the step-up like every other
   page. A person who can log in to the console but has no step-up method sets one up in an app;
   `consoleStepUpNoMethod` says so.
9. **E3 keeps its three ways.** The API has no operation that sets a password while an enrollment
   code is redeemed (D-M4-18-9). E3 stays as built; 01 5.6 says that the password comes after the
   first login under Settings, Sign-in, which needs no step-up on a device with nothing to confirm
   with.

## 3. Not in this ticket

Revoking an own passkey and unlinking an identity from the view (the API has `revokePasskey` and
the owner's person view revokes passkeys); passkeys on macOS; an end-to-end test of the console
that registers a passkey through the page (`make console-e2e` still injects it).
