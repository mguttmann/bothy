// SPDX-License-Identifier: Apache-2.0

// Local work inside the stream loop is no tampering: a failed `identify` (the access token or
// the DPoP proof could not be made) detaches the tab with "Reattach", and a pin that could not
// be stored leaves the connected session alone. Only Noise and frame errors give the hard
// warning (docs/spec/24-terminal.md 5.3 and 5.4; docs/decisions.d/M4-35.md D-M4-35-21).

import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';

void main() {
  test('a failed identify detaches the tab instead of blocking it', () async {
    final h = await Harness.create();
    h.transport.identifyError = StateError('token refresh failed');
    await h.open();
    await h.until((s) => s.phase != TerminalPhase.connecting);
    final state = h.controller.state;
    expect(state.phase, TerminalPhase.detached);
    expect(state.detachCause, DetachCause.authenticationFailed);
    expect(state.blockCause, isNull);
    await h.settle();
    expect(h.host.openStreams, 0);

    // Reattach works once the login works again.
    h.transport.identifyError = null;
    await h.controller.attach();
    await h.connected();
  });

  test('a pin that cannot be stored keeps the session connected', () async {
    final h = await Harness.create();
    h.pins.writeError = StateError('quota exceeded');
    await h.open();
    await h.connected();
    await h.settle();
    expect(h.controller.state.phase, TerminalPhase.connected);
    expect(h.controller.state.blockCause, isNull);
    expect(h.pins.pins, isEmpty);
  });
}
