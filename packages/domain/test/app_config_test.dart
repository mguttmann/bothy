// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

/// Rows of the generated block of section 4: key to (place, default).
Map<String, (String, String)> _reference() {
  final text = readRepoFile('docs/operations/configuration.md');
  final block = text
      .split('<!-- config-reference:start -->')[1]
      .split('<!-- config-reference:end -->')[0];
  final rows = <String, (String, String)>{};
  for (final line in block.split('\n')) {
    if (!line.startsWith('| `')) continue;
    final cells = line.split('|').map((c) => c.trim()).toList();
    String unquote(String c) =>
        c.length >= 2 && c.startsWith('`') && c.endsWith('`')
        ? c.substring(1, c.length - 1)
        : c;
    rows[unquote(cells[1])] = (cells[3], unquote(cells[5]));
  }
  return rows;
}

void main() {
  final reference = _reference();
  final config = AppConfig.defaults();

  test(
    'defaults() holds every key of place App with the reference default',
    () {
      final appKeys = reference.entries
          .where((e) => e.value.$1 == 'App')
          .map((e) => e.key);
      expect(appKeys, isNotEmpty);
      for (final key in appKeys) {
        final entry = appConfigKeys.firstWhere(
          (k) => k.key == key,
          orElse: () => fail('$key is missing'),
        );
        expect(entry.defaultText, reference[key]!.$2, reason: key);
        expect(entry.place, 'App');
        expect(
          config.value<Object>(key),
          parseAppConfigValue(key, entry.kind, reference[key]!.$2),
        );
      }
    },
  );

  test('the keys of D-W-115 and D-W-116 are there with place Server', () {
    const listed = [
      'sync.reconnect_backoff_base',
      'sync.reconnect_backoff_cap',
      'sync.typing_min_interval',
      'sync.typing_ttl',
      'sync.max_focused_threads',
      'client.cache_max_age',
      'client.outbox_retry_base',
      'client.outbox_retry_cap',
      'auth.device_pending_ttl',
    ];
    expect(serverKeysReadByApp, unorderedEquals(listed));
    for (final key in listed) {
      expect(
        reference[key]?.$1,
        'Server',
        reason: '$key must stay in the reference with place Server',
      );
      final entry = appConfigKeys.firstWhere((k) => k.key == key);
      expect(entry.defaultText, reference[key]!.$2, reason: key);
    }
    expect(
      appConfigKeys.length,
      reference.values.where((r) => r.$1 == 'App').length + listed.length,
    );
    expect(AppConfig.keys, hasLength(appConfigKeys.length));
  });

  test('limits of InstanceLimits are never in AppConfig', () {
    for (final key in AppConfig.keys) {
      expect(
        key,
        isNot(
          anyOf(
            'file.max_upload_bytes',
            'voice.max_duration',
            'message.max_content_bytes',
          ),
        ),
      );
    }
  });

  test('outboxStuckAfter parses 2m as two minutes', () {
    expect(config.outboxStuckAfter, const Duration(minutes: 2));
    expect(config.quickSwitchMaxResults, 20);
    expect(config.cacheMaxBytes, 1024 * 1024 * 1024);
    expect(config.voiceBitrate, 64000);
    expect(config.perfTerminalThroughputMin, 1024 * 1024);
    expect(config.desktopMinWidthFourColumns, 1280);
    expect(config.perfJankRatioMax, 0.01);
    expect(config.timelineSpeeds, ['60', '300', '900']);
    expect(config.tvShowCosts, isFalse);
    expect(config.cacheMaxAge, const Duration(days: 30));
    expect(config.devicePendingTtl, const Duration(hours: 24));
  });

  test('withOverrides replaces values and rejects unknown keys and malformed values', () {
    final tuned = config.withOverrides({
      'ui.quick_switch_max_results': '30',
      'client.outbox_stuck_after': '90s',
    });
    expect(tuned.quickSwitchMaxResults, 30);
    expect(tuned.outboxStuckAfter, const Duration(seconds: 90));
    expect(
      config.quickSwitchMaxResults,
      20,
      reason: 'the original stays unchanged',
    );
    expect(
      () => config.withOverrides({'ui.rocket': '1'}),
      throwsA(
        isA<FormatException>().having(
          (e) => e.message,
          'message',
          contains('ui.rocket'),
        ),
      ),
    );
    expect(
      () => config.withOverrides({'client.outbox_stuck_after': '1h30m'}),
      throwsA(
        isA<FormatException>()
            .having(
              (e) => e.message,
              'message',
              contains('client.outbox_stuck_after'),
            )
            .having((e) => e.message, 'message', isNot(contains('1h30m'))),
      ),
    );
    expect(() => config.value<Object>('ui.rocket'), throwsArgumentError);
  });

  group('parsers', () {
    Object parse(AppConfigKind kind, String text) =>
        parseAppConfigValue('a.b', kind, text);

    test('accept the grammar of the reference', () {
      expect(parse(AppConfigKind.boolean, 'true'), isTrue);
      expect(
        parse(AppConfigKind.duration, '250ms'),
        const Duration(milliseconds: 250),
      );
      expect(parse(AppConfigKind.duration, '30s'), const Duration(seconds: 30));
      expect(parse(AppConfigKind.duration, '2h'), const Duration(hours: 2));
      expect(parse(AppConfigKind.duration, '0s'), Duration.zero);
      expect(parse(AppConfigKind.bytes, '300MiB'), 300 * 1024 * 1024);
      expect(parse(AppConfigKind.bytes, '4KiB'), 4096);
      expect(parse(AppConfigKind.byteRate, '2KiB/s'), 2048);
      expect(parse(AppConfigKind.bitRate, '128kbps'), 128000);
      expect(parse(AppConfigKind.pixels, '960px'), 960);
      expect(parse(AppConfigKind.integer, '-3'), -3);
      expect(parse(AppConfigKind.decimal, '.5'), 0.5);
      expect(parse(AppConfigKind.decimal, '1e3'), 1000.0);
      expect(parse(AppConfigKind.list, 'a, b'), ['a', 'b']);
      expect(parse(AppConfigKind.list, '["x,y"]'), ['x,y']);
      expect(parse(AppConfigKind.list, ''), isEmpty);
    });

    test('reject everything else', () {
      for (final (kind, text) in [
        (AppConfigKind.boolean, 'yes'),
        (AppConfigKind.duration, '01s'),
        (AppConfigKind.duration, '1.5s'),
        (AppConfigKind.duration, '-1s'),
        (AppConfigKind.bytes, '1TiB'),
        (AppConfigKind.byteRate, '1MiB'),
        (AppConfigKind.bitRate, '1mbps'),
        (AppConfigKind.pixels, '10'),
        (AppConfigKind.integer, '+1'),
        (AppConfigKind.integer, '0x10'),
        (AppConfigKind.integer, '9' * 25),
        (AppConfigKind.decimal, 'NaN'),
        (AppConfigKind.decimal, '1e999'),
        (AppConfigKind.list, 'a,,b'),
        (AppConfigKind.list, '[1]'),
        (AppConfigKind.list, '[""]'),
        (AppConfigKind.list, '[oops'),
      ]) {
        expect(
          () => parse(kind, text),
          throwsFormatException,
          reason: '$kind $text',
        );
      }
    });
  });
}
