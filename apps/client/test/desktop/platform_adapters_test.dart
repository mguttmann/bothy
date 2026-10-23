// SPDX-License-Identifier: Apache-2.0

// The platform adapters of M4-30 over their method channels (brief M4-30 Scope, 15 10.3): the
// window of `window_manager`, the tray of `tray_manager`, the macOS login item over the runner
// channel and the fallbacks without a platform part.

import 'package:client/desktop/start_at_login.dart';
import 'package:client/desktop/tray.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:tray_manager/tray_manager.dart';
import 'package:window_manager/window_manager.dart';

/// Records the calls on [channel] and answers with [answers] by method name.
List<MethodCall> record(
  String channel, [
  Map<String, Object?> answers = const {},
]) {
  final calls = <MethodCall>[];
  TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
      .setMockMethodCallHandler(MethodChannel(channel), (call) async {
        calls.add(call);
        final answer = answers[call.method];
        if (answer is Exception) throw answer;
        return answer;
      });
  addTearDown(
    () => TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(MethodChannel(channel), null),
  );
  return calls;
}

List<String> methods(List<MethodCall> calls) => [
  for (final c in calls) c.method,
];

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('WindowManagerWindow', () {
    test('start prevents the close and forwards the window events', () async {
      final calls = record('window_manager', {'isMinimized': true});
      final window = WindowManagerWindow(manager: windowManager);
      final events = <DesktopWindowEvent>[];
      final sub = window.events.listen(events.add);
      await window.start(preventClose: true);
      expect(methods(calls), ['ensureInitialized', 'setPreventClose']);
      expect(calls.last.arguments, {'isPreventClose': true});

      window
        ..onWindowClose()
        ..onWindowFocus()
        ..onWindowBlur()
        ..onWindowMinimize()
        ..onWindowRestore()
        ..onWindowEvent('hide')
        ..onWindowEvent('show')
        ..onWindowEvent('resize');
      await pumpEventQueue();
      expect(events, [
        DesktopWindowEvent.close,
        DesktopWindowEvent.focus,
        DesktopWindowEvent.blur,
        DesktopWindowEvent.minimize,
        DesktopWindowEvent.restore,
        DesktopWindowEvent.hide,
        DesktopWindowEvent.show,
      ]);

      calls.clear();
      await window.show();
      expect(
        methods(calls),
        containsAllInOrder(['isMinimized', 'restore', 'show', 'focus']),
      );
      calls.clear();
      await window.hide();
      await window.destroy();
      expect(methods(calls), ['hide', 'destroy']);
      await sub.cancel();
    });

    test('the no-op window does nothing', () async {
      const w = NoDesktopWindow();
      await w.start(preventClose: true);
      await w.hide();
      await w.show();
      await w.destroy();
      expect(await w.events.isEmpty, isTrue);
    });
  });

  group('TrayManagerTray', () {
    test('shows icon, tooltip and menu; clicks become actions', () async {
      final calls = record('tray_manager');
      final tray = TrayManagerTray(manager: trayManager);
      final actions = <TrayAction>[];
      final sub = tray.actions.listen(actions.add);
      await tray.show(
        iconAsset: trayIconAsset,
        tooltip: 'ExampleChat',
        openLabel: 'Open',
        quitLabel: 'Quit',
      );
      expect(methods(calls), ['setIcon', 'setToolTip', 'setContextMenu']);
      expect(
        (calls.first.arguments as Map)['iconPath'] as String,
        endsWith(trayIconAsset),
      );

      tray
        ..onTrayIconMouseDown()
        ..onTrayMenuItemClick(MenuItem(key: 'open', label: 'Open'))
        ..onTrayMenuItemClick(MenuItem(key: 'quit', label: 'Quit'))
        ..onTrayMenuItemClick(MenuItem(key: 'other', label: 'Other'))
        ..onTrayIconRightMouseDown();
      await pumpEventQueue();
      expect(actions, [TrayAction.open, TrayAction.open, TrayAction.quit]);
      expect(methods(calls), contains('popUpContextMenu'));

      await sub.cancel();
      await tray.dispose();
      expect(methods(calls).last, 'destroy');
    });

    test('the no-op tray does nothing', () async {
      const t = NoTray();
      await t.show(
        iconAsset: trayIconAsset,
        tooltip: 'ExampleChat',
        openLabel: 'Open',
        quitLabel: 'Quit',
      );
      expect(await t.actions.isEmpty, isTrue);
      await t.dispose();
    });
  });

  group('MacLoginItem', () {
    const channel = 'bothy/start_at_login';

    test('reads and writes the login item over the runner channel', () async {
      final calls = record(channel, {'isSupported': true, 'isEnabled': true});
      final item = await MacLoginItem.open();
      expect(item.supported, isTrue);
      expect(await item.isEnabled(), isTrue);
      await item.setEnabled(false);
      expect(methods(calls), ['isSupported', 'isEnabled', 'setEnabled']);
      expect(calls.last.arguments, false);
    });

    test('a runner without the channel or an error offers none', () async {
      final missing = await MacLoginItem.open(
        channel: const MethodChannel('bothy/start_at_login_missing'),
      );
      expect(missing.supported, isFalse);
      expect(await missing.isEnabled(), isFalse);
      await expectLater(missing.setEnabled(true), throwsStateError);

      record(channel, {'isSupported': PlatformException(code: 'old_macos')});
      final failing = await MacLoginItem.open();
      expect(failing.supported, isFalse);
    });

    test('without a login start on the system nothing is written', () async {
      const none = NoStartAtLogin();
      expect(none.supported, isFalse);
      expect(await none.isEnabled(), isFalse);
      await expectLater(none.setEnabled(true), throwsStateError);
    });
  });
}
