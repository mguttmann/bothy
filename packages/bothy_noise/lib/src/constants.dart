// SPDX-License-Identifier: Apache-2.0

// Protocol constants of docs/spec/24-terminal.md 5.2 and 5.4. They change only with a new
// protocol version, never through configuration (docs/spec/02-architektur.md "Code-Layout",
// D-N10).

/// The Noise protocol of the terminal stream (docs/spec/24-terminal.md 5.2).
const String noiseProtocolName = 'Noise_NK_25519_ChaChaPoly_SHA256';

/// The label that opens the prologue (docs/spec/24-terminal.md 5.2); 16 ASCII bytes.
const String prologueLabel = 'bothy-terminal/1';

/// The largest Noise message in bytes (Noise revision 34, section 3).
const int maxNoiseMessageLength = 65535;

/// The length of the ChaChaPoly authentication tag (Noise revision 34, section 12.3).
const int noiseTagLength = 16;

/// The largest plaintext of one transport message.
const int maxTransportPlaintextLength = maxNoiseMessageLength - noiseTagLength;

/// The largest encoded frame: the largest transport plaintext (docs/spec/24-terminal.md 5.4).
const int maxFrameLength = maxTransportPlaintextLength;

/// The length of an X25519 key and of the SHA-256 handshake hash.
const int keyLength = 32;

/// The length of a session or host ID in the prologue.
const int idLength = 16;
