// SPDX-License-Identifier: Apache-2.0

// A SelfRepository that behaves like the server behind the step-up interceptor: changing calls
// of `device.approve` and `device.revoke` ask the step-up prompt while no confirmation is
// fresh, `session.revoke` never asks, a pairing code without a pairing answers 404 and ends
// the confirmation (08 5.2, 5.3, 6.2, 10). Plus the app frame the views run in.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

/// The fixed time of the tests: Saturday, 17 October 2026, 09:00 UTC.
final DateTime testNow = DateTime.utc(2026, 10, 17, 9);

/// The clock of the views.
DateTime testClock() => testNow;

/// Alex's devices: this laptop, an active phone, a waiting tablet and a revoked old phone.
List<SelfDevice> alexDevices() => [
  SelfDevice(
    id: 'laptop',
    name: 'Alex laptop',
    platform: 'macos',
    state: SelfDeviceState.active,
    createdAt: DateTime.utc(2026, 10, 1),
    appVersion: '1.4.0',
    lastSeenAt: testNow,
    isThisDevice: true,
  ),
  SelfDevice(
    id: 'phone',
    name: 'Alex phone',
    platform: 'ios',
    state: SelfDeviceState.active,
    createdAt: DateTime.utc(2026, 10, 2),
    appVersion: '1.4.0',
    lastSeenAt: DateTime.utc(2026, 10, 16, 18, 30),
  ),
  SelfDevice(
    id: 'tablet',
    name: 'Alex tablet',
    platform: 'android',
    state: SelfDeviceState.pending,
    createdAt: DateTime.utc(2026, 10, 17, 8, 55),
  ),
  SelfDevice(
    id: 'old',
    name: 'Alex old phone',
    platform: 'windows',
    state: SelfDeviceState.revoked,
    createdAt: DateTime.utc(2026, 9, 1),
    lastSeenAt: DateTime.utc(2026, 9, 20, 7),
  ),
];

/// Alex's sessions: the current one on the laptop and one on the phone.
List<SelfSession> alexSessions() => [
  SelfSession(
    id: 's-laptop',
    deviceId: 'laptop',
    deviceName: 'Alex laptop',
    authMethod: SelfAuthMethod.passwordTotp,
    lastUsedAt: testNow,
    current: true,
  ),
  SelfSession(
    id: 's-phone',
    deviceId: 'phone',
    deviceName: 'Alex phone',
    authMethod: SelfAuthMethod.devicePairing,
    lastUsedAt: DateTime.utc(2026, 10, 16, 18, 30),
  ),
];

/// The fake repository.
class FakeSelfRepository implements SelfRepository {
  /// Creates the fake; [prompt] is asked as the step-up interceptor would.
  FakeSelfRepository({
    required this.prompt,
    List<SelfDevice>? devices,
    List<SelfSession>? sessions,
  }) : deviceList = devices ?? alexDevices(),
       sessionList = sessions ?? alexSessions();

  /// The step-up prompt.
  final StepUpPrompt prompt;

  /// The devices.
  List<SelfDevice> deviceList;

  /// The sessions.
  List<SelfSession> sessionList;

  /// The verification code per waiting device.
  Map<String, String> codes = {'tablet': '481207'};

  /// The open pairing codes and the device each one adds.
  Map<String, String> pairings = {'HQ7P3MXR': 'Alex desktop'};

  /// Operations the server refuses, by name, with their problem.
  Map<String, ApiProblem> refuse = {};

  /// Whether a step-up of the session is fresh.
  bool stepUpFresh = false;

  /// The purpose texts of the step-ups asked so far.
  final List<String> stepUps = [];

  /// The calls in order.
  final List<String> calls = [];

  Future<void> _stepUp(String purpose) async {
    if (stepUpFresh) return;
    stepUps.add(purpose);
    if (!await prompt.ask(purpose)) throw const StepUpCancelled();
    stepUpFresh = true;
  }

  void _refused(String op) {
    final problem = refuse[op];
    if (problem != null) throw SelfActionFailed(problem);
  }

  SelfDevice _with(SelfDevice d, SelfDeviceState state) => SelfDevice(
    id: d.id,
    name: d.name,
    platform: d.platform,
    state: state,
    createdAt: d.createdAt,
    appVersion: d.appVersion,
    lastSeenAt: d.lastSeenAt,
    isThisDevice: d.isThisDevice,
  );

  void _replace(SelfDevice d) =>
      deviceList = [for (final x in deviceList) x.id == d.id ? d : x];

  @override
  Future<List<SelfDevice>> devices() async => List.of(deviceList);

  @override
  Future<List<SelfSession>> sessions() async => List.of(sessionList);

  @override
  Future<SelfDevice> approveDevice(
    String deviceId,
    String code, {
    required String purpose,
  }) async {
    calls.add('approveDevice $deviceId $code');
    _refused('approveDevice');
    await _stepUp(purpose);
    final device = deviceList.firstWhere((d) => d.id == deviceId);
    final next = _with(
      device,
      codes[deviceId] == code
          ? SelfDeviceState.active
          : SelfDeviceState.revoked,
    );
    _replace(next);
    return next;
  }

  @override
  Future<SelfDevice> approvePairing(
    String code, {
    required String purpose,
  }) async {
    calls.add('approvePairing $code');
    _refused('approvePairing');
    await _stepUp(purpose);
    final name = pairings.remove(code);
    if (name == null) {
      stepUpFresh = false;
      throw const SelfActionFailed(ApiProblem(status: 404, code: 'not_found'));
    }
    final device = SelfDevice(
      id: 'new',
      name: name,
      platform: 'windows',
      state: SelfDeviceState.active,
      createdAt: testNow,
    );
    deviceList = [...deviceList, device];
    return device;
  }

  @override
  Future<SelfDevice> revokeDevice(
    String deviceId, {
    required String reason,
    required String purpose,
  }) async {
    calls.add('revokeDevice $deviceId $reason');
    _refused('revokeDevice');
    await _stepUp(purpose);
    final next = _with(
      deviceList.firstWhere((d) => d.id == deviceId),
      SelfDeviceState.revoked,
    );
    _replace(next);
    return next;
  }

  @override
  Future<void> revokeSession(String sessionId) async {
    calls.add('revokeSession $sessionId');
    _refused('revokeSession');
    sessionList = [
      for (final s in sessionList)
        if (s.id != sessionId) s,
    ];
  }
}

/// Pumps [child] in an app with the localizations of [locale], the design theme and a
/// navigator under [navigatorKey]; returns after the first frames settled.
Future<void> pumpInApp(
  WidgetTester tester,
  Widget child, {
  GlobalKey<NavigatorState>? navigatorKey,
  Locale locale = const Locale('en'),
}) async {
  tester.view
    ..physicalSize = const Size(1200, 1000)
    ..devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  const ds = DsContext(
    theme: DsThemeMode.light,
    density: DsDensityMode.comfortable,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  await tester.pumpWidget(
    MaterialApp(
      navigatorKey: navigatorKey,
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      theme: buildDsTheme(ds),
      builder: (context, app) => DsThemeScope(dsContext: ds, child: app!),
      home: Scaffold(
        body: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: child,
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}
