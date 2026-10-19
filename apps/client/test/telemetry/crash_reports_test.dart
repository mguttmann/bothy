// SPDX-License-Identifier: Apache-2.0

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
    expect(o.beforeSend, isNotNull);
    expect(o.beforeBreadcrumb, isNotNull);
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
