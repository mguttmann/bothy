// SPDX-License-Identifier: Apache-2.0

/// Device keys in the secure storage of each platform (Flutter plugin, Pigeon).
///
/// Two ES256 keys per device and server (docs/spec/08-identity.md 6.1):
/// the `dpop` key signs every DPoP proof without asking the user, the
/// `step_up` key asks for biometrics or the device password on every
/// signature. [DeviceKeyClient] is the entry point; the public JWK and its
/// RFC 7638 thumbprint come from [DeviceKeyInfo], computed in Dart for every
/// platform.
///
/// ```dart
/// final keys = DeviceKeyClient();
/// final info = await keys.ensure('https://chat.example.com', KeyPurpose.dpop);
/// final jwk = info.publicJwk;
/// final signature = await keys.sign(
///   'https://chat.example.com',
///   KeyPurpose.dpop,
///   signingInput,
/// );
/// ```
library;

export 'src/device_key.dart'
    show
        DeviceKeyClient,
        DeviceKeyError,
        DeviceKeyException,
        DeviceKeyInfo,
        KeyPurpose,
        StepUpPrompt;
