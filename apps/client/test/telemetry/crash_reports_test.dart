// SPDX-License-Identifier: Apache-2.0

import 'dart:async';

import 'package:client/telemetry/crash_reports.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sentry_flutter/sentry_flutter.dart';

/// Records the starts and closes instead of calling Sentry.
final class FakeSentry {
  final List<SentryFlutterOptions> started = [];
  int closed = 0;

  Future<void> start(FlutterOptionsConfiguration configure) async {
    final options = SentryFlutterOptions();
    await configure(options);
    started.add(options);
  }

  Future<void> close() async => closed++;
}

const dsn = 'https://public@crash.example.com/1';

CrashReports reports(FakeSentry fake, {String target = dsn}) => CrashReports(
  dsn: target,
  release: 'client@1.4.0',
  environment: 'production',
  start: fake.start,
  close: fake.close,
);

void main() {
  test('no init without a target', () async {
    final fake = FakeSentry();
    final r = reports(fake, target: '');
    expect(r.hasTarget, isFalse);
    await r.apply(enabled: true);
    expect(fake.started, isEmpty);
    expect(r.running, isFalse);
  });

  test('no init with the switch off', () async {
    final fake = FakeSentry();
    await reports(fake).apply(enabled: false);
    expect(fake.started, isEmpty);
  });

  test('init once with a target and the switch on, scrubbed options', () async {
    final fake = FakeSentry();
    final r = reports(fake);
    await r.apply(enabled: true);
    await r.apply(enabled: true);
    expect(fake.started, hasLength(1));
    final o = fake.started.single;
    expect(o.dsn, dsn);
    expect(o.release, 'client@1.4.0');
    expect(o.environment, 'production');
    expect(o.sendDefaultPii, isFalse);
    expect(o.attachScreenshot, isFalse);
    expect(o.attachViewHierarchy, isFalse);
    expect(o.replay.sessionSampleRate ?? 0, 0);
    expect(o.replay.onErrorSampleRate ?? 0, 0);
    expect(o.enableUserInteractionBreadcrumbs, isFalse);
    expect(o.tracesSampleRate, isNull);
    // The native crash handlers (breakpad minidumps, the native beforeSend) and the session
    // pings bypass the Dart scrubber, so they stay off.
    expect(o.enableNativeCrashHandling, isFalse);
    expect(o.enableAutoSessionTracking, isFalse);
    expect(o.beforeSend, isNotNull);
    expect(o.beforeBreadcrumb, isNotNull);
  });

  test('a switch-off during the start closes Sentry after the start', () async {
    // The start of Sentry takes time; a close that ran before it ended met an empty hub and
    // Sentry kept running until the process ended (AUD-PKG-P02-client-b, #1280).
    final events = <String>[];
    final started = Completer<void>();
    final r = CrashReports(
      dsn: dsn,
      release: 'client@1.4.0',
      environment: 'production',
      start: (_) async {
        events.add('start');
        await started.future;
        events.add('started');
      },
      close: () async => events.add('close'),
    );
    final on = r.apply(enabled: true);
    final off = r.apply(enabled: false);
    await Future<void>.delayed(Duration.zero);
    started.complete();
    await Future.wait([on, off]);
    expect(events, ['start', 'started', 'close']);
    expect(r.running, isFalse);
  });

  test('turning the switch off closes Sentry', () async {
    final fake = FakeSentry();
    final r = reports(fake);
    await r.apply(enabled: true);
    await r.apply(enabled: false);
    expect(fake.closed, 1);
    expect(r.running, isFalse);
  });

  test('the instance tag is a hash, never the ID', () {
    final tag = CrashReports.instanceTag(
      '0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b',
    );
    expect(tag, hasLength(16));
    expect(tag, isNot(contains('0199a3b4')));
    expect(
      CrashReports.instanceTag('0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b'),
      tag,
    );
  });

  test('URLs lose query, fragment, user info, IDs and file names', () {
    expect(
      CrashReports.scrubUrl(
        'https://alex:pw@chat.example.com/api/v1/threads/0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b/messages?limit=5&token=x#top',
      ),
      'https://chat.example.com/api/v1/threads/:id/messages',
    );
    expect(
      CrashReports.scrubUrl('/t/0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b'),
      '/t/:id',
    );
    expect(
      CrashReports.scrubUrl('/files/f1/Quarterly report.pdf'),
      '/files/f1/:id',
    );
    expect(
      CrashReports.scrubUrl('/api/v1/tokens/bt_AbCdEf0123456789xyz'),
      '/api/v1/tokens/:id',
    );
  });
}
