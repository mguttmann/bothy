// SPDX-License-Identifier: Apache-2.0

/// The end-to-end layer of the terminal in pure Dart: the handshake
/// `Noise_NK_25519_ChaChaPoly_SHA256` (initiator), the transport, the prologue, the frames and
/// the host key fingerprint (docs/spec/24-terminal.md 4.3, 5.2 to 5.4).
///
/// The same code runs in the desktop app, the mobile app and the web console: no Flutter, no
/// networking, no storage, and no dependency with crypto code except `cryptography`
/// (docs/spec/15-client-architektur.md 3.1 rule 7). The responder for tests and fake hosts is
/// in `package:bothy_noise/testing.dart`.
library;

export 'src/constants.dart'
    show
        maxFrameLength,
        maxNoiseMessageLength,
        maxTransportPlaintextLength,
        noiseProtocolName,
        noiseTagLength,
        prologueLabel;
export 'src/errors.dart'
    show FrameError, FrameException, NoiseError, NoiseException;
export 'src/fingerprint.dart' show crockfordBase32, hostKeyFingerprint;
export 'src/frame.dart'
    show
        CloseFrame,
        DataFrame,
        ExitFrame,
        FrameSender,
        ReplayBeginFrame,
        ReplayEndFrame,
        ResizeFrame,
        StartFrame,
        TerminalFrame,
        decodeFrame,
        encodeFrame;
export 'src/handshake.dart' show NkInitiator, NkResult;
export 'src/ids.dart'
    show buildPrologue, ephemeralKeyHash, generateEphemeralKeyPair, uuidToBytes;
export 'src/transport.dart' show NoiseTransport;
