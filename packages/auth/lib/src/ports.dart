// SPDX-License-Identifier: Apache-2.0

/// The ports through which `auth` reaches the outside world: device keys, the secret store,
/// the system browser, the app redirect, passkeys and time (brief M4-09 Design note 1).
///
/// The platform packages implement them (M4-10, M4-11, M5-01 for the keys; M4-18 for the
/// store, browser and redirect); tests use fakes, so every flow runs without I/O
/// (15 1 principle 5).
library;

import 'dart:typed_data';

/// A non-exportable device key that signs JOSE input (08 6.1, 15 9).
///
/// All clients use ES256 on P-256 (R-CL-04), because the Secure Enclave and StrongBox know no
/// other curve.
abstract interface class DeviceKeySigner {
  /// The JOSE algorithm of the key; `ES256` on every client.
  String get alg;

  /// The public JWK (RFC 7517) with `kty`, `crv`, `x`, `y`; never a private member.
  Map<String, String> get publicJwk;

  /// The RFC 7638 thumbprint of [publicJwk], the server's `jkt`.
  String get thumbprint;

  /// Signs [signingInput] (the ASCII `header.payload` of a compact JWS) and returns the JOSE
  /// form of the signature: `R || S` with 32 bytes each for ES256 (RFC 7518 3.4), never DER.
  ///
  /// A step-up key asks the operating system for biometrics or the device PIN here; a refused
  /// or cancelled confirmation throws (the step-up flow maps it to `StepUpCancelled`).
  Future<Uint8List> sign(Uint8List signingInput);
}

/// The keys of a native device (R-CL-05): [dpop] signs every proof without asking the
/// person; [stepUp] signs step-up confirmations only after user verification.
final class DeviceKeys {
  /// Creates the keys; [stepUp] is `null` on a device without a step-up key.
  const DeviceKeys({required this.dpop, this.stepUp});

  /// The DPoP key; also the key of the refresh family (08 7.4).
  final DeviceKeySigner dpop;

  /// The step-up key with user presence (08 6.1, 6.2), or `null` where the platform has none:
  /// Windows by design (08 6.1, D-M4-11-2), or a device whose key store cannot hold a key with
  /// user presence. Such a device registers without `step_up_key`, and its step-up with the
  /// device key fails with `StepUpUnavailable`.
  final DeviceKeySigner? stepUp;
}

/// Persistent secret storage of the platform (keychain, Keystore, Credential Manager), by key.
///
/// The refresh token and the device ID live here; the access token stays in memory
/// (15 8). Writes must be durable when the returned future completes, because the session
/// publishes a rotated token only after its write (docs/DECISIONS.md D-M4-09-3).
abstract interface class SecretStore {
  /// The value stored under [key], or `null`.
  Future<String?> read(String key);

  /// Stores [value] under [key], replacing an older value.
  Future<void> write(String key, String value);

  /// Removes [key]; removing a missing key is no error.
  Future<void> delete(String key);
}

/// Opens a URL in the system browser (ASWebAuthenticationSession, Custom Tabs, default
/// browser), never in a web view (08 2.2 item 3, RFC 8252 8.12).
abstract interface class BrowserLauncher {
  /// Opens [url]; completes when the browser was handed the URL, not when the login ends.
  Future<void> open(Uri url);
}

/// Receives the redirect of the server back to the app (15 8): a loopback listener on
/// `127.0.0.1` with a free port on desktop, a private scheme or claimed `https` link on mobile.
abstract interface class RedirectReceiver {
  /// The `app_redirect_uri` to register for the next login, for example
  /// `http://127.0.0.1:49152/callback`; a loopback receiver starts listening here.
  Future<Uri> redirectUri();

  /// Waits for the first redirect that starts with [expectedPrefix] and returns it with its
  /// query (`login_code`, `login_attempt_id` or `error`).
  ///
  /// Throws a `TimeoutException` when none arrives within [timeout].
  Future<Uri> waitForRedirect(Uri expectedPrefix, Duration timeout);
}

/// WebAuthn on the platform, for instances with a public domain (08 2.5, 15 8).
///
/// Options and results are the JSON forms of WebAuthn Level 3 as the server sends and
/// expects them; this package does not look inside.
abstract interface class PasskeyAuthenticator {
  /// Runs `navigator.credentials.get` with [requestOptions]
  /// (`PublicKeyCredentialRequestOptions`) and returns the `PublicKeyCredential` JSON.
  Future<Map<String, Object?>> getAssertion(
    Map<String, Object?> requestOptions,
  );

  /// Runs `navigator.credentials.create` with [creationOptions]
  /// (`PublicKeyCredentialCreationOptions`) and returns the `PublicKeyCredential` JSON.
  Future<Map<String, Object?>> createCredential(
    Map<String, Object?> creationOptions,
  );
}

/// Time for proofs, expiry checks and polling waits; injected so tests control it.
abstract interface class Clock {
  /// The current time.
  DateTime now();

  /// Waits [duration] (a backoff between polls).
  Future<void> delay(Duration duration);
}

/// The [Clock] of the running app: the system time and real timers.
final class SystemClock implements Clock {
  /// Creates the clock.
  const SystemClock();

  @override
  DateTime now() => DateTime.now().toUtc();

  @override
  Future<void> delay(Duration duration) => Future<void>.delayed(duration);
}
