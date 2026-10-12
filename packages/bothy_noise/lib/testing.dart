// SPDX-License-Identifier: Apache-2.0

/// The responder of the handshake, only for tests and fake hosts
/// (docs/spec/15-client-architecture.md 23.1, docs/DECISIONS.md D-M2-38-14). Apps import
/// `package:bothy_noise/bothy_noise.dart`; M4-35 uses this library as a fake host.
library;

export 'src/handshake.dart' show NkResponder;
