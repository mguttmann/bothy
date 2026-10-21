// SPDX-License-Identifier: Apache-2.0

// The window in the notification area (brief M4-30 Tests; 15 10.3): closing hides the window on
// Windows and keeps the engine connected; Quit disposes the engine and then ends the window;
// on macOS a close is left to the system; the tray menu opens and quits.

import 'package:client/desktop/tray.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/desktop_fakes.dart';

/// The engine as far as these tests need it: connected until disposed.
class FakeEngine {
  bool connected = true;
  int disposals = 0;

  Future<void> dispose() async {
    disposals++;
    connected = false;
  }
}

void main() {
  late FakeWindow window;
  late FakeEngine engine;

  setUp(() {
    window = FakeWindow();
    engine = FakeEngine();
  });

  WindowBehavior behavior({required bool hideOnClose}) => WindowBehavior(
    window: window,
    hideOnClose: hideOnClose,
    shutdown: engine.dispose,
  );

  test('closing hides on Windows and keeps the engine connected', () async {
    final b = behavior(hideOnClose: true);
    await b.start();
    expect(window.calls, ['start preventClose=true']);
    window.send(DesktopWindowEvent.close);
    await pumpEventQueue();
    expect(window.calls, contains('hide'));
    expect(window.calls, isNot(contains('destroy')));
    expect(engine.connected, isTrue);
    expect(engine.disposals, 0);
    expect(b.active, isFalse, reason: 'hidden counts as idle');
    await b.dispose();
  });

  test('Quit disposes the engine first, then ends the window', () async {
    final b = behavior(hideOnClose: true);
    await b.start();
    window.send(DesktopWindowEvent.close);
    await b.quit();
    expect(engine.connected, isFalse);
    expect(engine.disposals, 1);
    expect(window.calls.last, 'destroy');
    await b.quit();
    expect(engine.disposals, 1, reason: 'a second Quit does nothing');
    await b.dispose();
  });

  test('on macOS a close is left to the system', () async {
    final b = behavior(hideOnClose: false);
    await b.start();
    expect(window.calls, ['start preventClose=false']);
    window.send(DesktopWindowEvent.close);
    await pumpEventQueue();
    expect(window.calls, ['start preventClose=false']);
    expect(engine.connected, isTrue);
    await b.dispose();
  });

  test('Open shows the hidden window and makes it active again', () async {
    final b = behavior(hideOnClose: true);
    await b.start();
    window.send(DesktopWindowEvent.close);
    await pumpEventQueue();
    expect(b.active, isFalse);
    await b.open();
    expect(window.calls.last, 'show');
    expect(b.active, isTrue);
    await b.dispose();
  });

  test('the tray menu has Open and Quit and drives the window', () async {
    final b = behavior(hideOnClose: true);
    await b.start();
    final port = FakeTray();
    final tray = DesktopTray(port: port, behavior: b);
    await tray.start(
      tooltip: 'ExampleChat',
      openLabel: 'Open',
      quitLabel: 'Quit',
    );
    expect(port.shown, ['ExampleChat|Open|Quit']);
    window.send(DesktopWindowEvent.close);
    await pumpEventQueue();
    port.click(TrayAction.open);
    await pumpEventQueue();
    expect(window.calls.last, 'show');
    port.click(TrayAction.quit);
    await pumpEventQueue();
    expect(engine.disposals, 1);
    expect(window.calls.last, 'destroy');
    await tray.dispose();
    await b.dispose();
  });
}
