// SPDX-License-Identifier: Apache-2.0

// The phone app as one device of Alex against the fake server (docs/decisions.d/
// AUD-client-app-01.md): the app widget of a phone build over the real services of
// `InstanceServices.open` (auth session, encrypted local database, sync engine), signed in over
// loopback HTTP and a real gateway socket. Widget tests use it where the path from a frame of
// the gateway to the screen matters.

import 'package:client/app/app.dart';
import 'package:client/app/providers/session.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../../support/fake_instance.dart';
import '../../support/until.dart';
import 'phone_app.dart' show phoneWindow;

/// The running phone app of Alex over [fake].
class PhoneDevice {
  PhoneDevice._(this.tester, this.fake, this.container);

  /// Signs Alex in on [fake] (a phone build, `platform: 'ios'` or `'android'`), shows the app
  /// in a phone window and waits for the phone shell with a live connection.
  static Future<PhoneDevice> start(
    WidgetTester tester,
    FakeInstance fake,
  ) async {
    phoneWindow(tester);
    final container = appContainer(fake);
    await tester.runAsync(() async {
      final control = await connected(container, fake);
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      await eventually(
        'live',
        () =>
            container.read(syncEngineProvider)?.state.phase ==
            ConnectionPhase.live,
      );
    });
    await tester.pumpWidget(
      UncontrolledProviderScope(container: container, child: const ClientApp()),
    );
    final device = PhoneDevice._(tester, fake, container);
    await device.until('the phone shell', () => shell.evaluate().isNotEmpty);
    return device;
  }

  /// The phone shell.
  static final Finder shell = find.byKey(const ValueKey('phone.shell'));

  final WidgetTester tester;

  /// The fake server and the ports of the app.
  final FakeInstance fake;

  /// The providers of the app.
  final ProviderContainer container;

  bool _ended = false;

  /// The router of the app.
  GoRouter get router => GoRouter.of(tester.element(shell));

  /// The path the router shows.
  String get location =>
      router.routerDelegate.currentConfiguration.uri.toString();

  /// Lets real IO run and pumps frames, without moving the fake clock, until [condition]
  /// holds; fails naming [what] after the patience of the tests.
  Future<void> until(String what, bool Function() condition) =>
      pumpUntil(tester, condition, what: what);

  /// Lets real IO and the frames of about [rounds] rounds pass without a condition, for what
  /// must not happen.
  Future<void> settle({int rounds = 60}) async {
    for (var i = 0; i < rounds; i++) {
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 5)),
      );
      await tester.pump();
    }
  }

  /// Ends the app inside the test zone, where its timers live: the last step of a test body.
  Future<void> end() async {
    if (_ended) return;
    _ended = true;
    await tester.pumpWidget(const SizedBox());
    container.dispose();
    await tester.pump(const Duration(minutes: 5));
  }

  /// Releases the providers of a test that failed before [end]; for `tearDown`, which runs
  /// outside the test zone and cannot pump.
  void dispose() {
    if (_ended) return;
    _ended = true;
    container.dispose();
  }
}
