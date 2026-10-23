// SPDX-License-Identifier: Apache-2.0

// Fakes of the desktop ports of M4-30: a window that records its calls and takes events from the
// test, a tray whose menu the test clicks, and a presenter that records its notifications.

import 'dart:async';

import 'package:client/desktop/tray.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:client/notifications/notification_presenter.dart';
import 'package:client/notifications/toast_content.dart';

/// A window that records the calls and lets the test send events.
class FakeWindow implements DesktopWindow {
  final StreamController<DesktopWindowEvent> _events =
      StreamController<DesktopWindowEvent>.broadcast();
  final List<String> calls = [];

  void send(DesktopWindowEvent e) => _events.add(e);

  @override
  Stream<DesktopWindowEvent> get events => _events.stream;

  @override
  Future<void> start({required bool preventClose}) async =>
      calls.add('start preventClose=$preventClose');

  @override
  Future<void> hide() async {
    calls.add('hide');
    send(DesktopWindowEvent.hide);
    await Future<void>.microtask(() {});
  }

  @override
  Future<void> show() async {
    calls.add('show');
    send(DesktopWindowEvent.show);
    send(DesktopWindowEvent.focus);
    await Future<void>.microtask(() {});
  }

  @override
  Future<void> destroy() async => calls.add('destroy');
}

/// A tray that records what it shows and lets the test click its menu.
class FakeTray implements TrayPort {
  final StreamController<TrayAction> _actions =
      StreamController<TrayAction>.broadcast();
  final List<String> shown = [];

  void click(TrayAction a) => _actions.add(a);

  @override
  Future<void> show({
    required String iconAsset,
    required String tooltip,
    required String openLabel,
    required String quitLabel,
  }) async => shown.add('$tooltip|$openLabel|$quitLabel');

  @override
  Stream<TrayAction> get actions => _actions.stream;

  @override
  Future<void> dispose() async {}
}

/// A presenter that records what it was asked to show.
class RecordingPresenter implements NotificationPresenter {
  final List<({ToastContent content, String tag, bool silent, String payload})>
  shown = [];
  final List<String> cancelled = [];
  final StreamController<String> taps = StreamController<String>.broadcast();
  String? launch;

  @override
  Future<void> show(
    ToastContent content, {
    required String tag,
    required bool silent,
    required String payload,
  }) async =>
      shown.add((content: content, tag: tag, silent: silent, payload: payload));

  @override
  Future<void> cancelThread(String tag) async => cancelled.add(tag);

  @override
  Stream<String> get clicks => taps.stream;

  @override
  Future<String?> launchPayload() async => launch;

  @override
  Future<void> dispose() => taps.close();
}
