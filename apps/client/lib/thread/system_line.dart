// SPDX-License-Identifier: Apache-2.0

/// System lines of the history (SEC-OUT-3; 01 6.1: "Reopened by message from Sam"): only a
/// message of kind `system` that the server wrote (`origin = system`) is drawn as a system
/// line; the component draws it from its code, never from content.
library;

import 'package:domain/domain.dart';

/// Whether [message] is a system line of the server.
bool isSystemLine(Message message) =>
    message.kind == MessageKind.system &&
    message.origin == MessageOrigin.system;
