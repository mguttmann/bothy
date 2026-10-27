// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

void main() {
  group('SemVer', () {
    SemVer v(String text) => SemVer.parse(text);

    test('parses core, pre-release and build metadata', () {
      final s = v('1.20.3-beta.2+exp.sha.5114f85');
      expect((s.major, s.minor, s.patch), (1, 20, 3));
      expect(s.preRelease, ['beta', '2']);
      expect(s.toString(), '1.20.3-beta.2+exp.sha.5114f85');
    });

    test('compares numerically, not as text', () {
      expect(v('1.2.0').compareTo(v('1.10.0')), lessThan(0));
      expect(v('2.0.0').compareTo(v('1.99.99')), greaterThan(0));
      expect(v('1.0.0').compareTo(v('1.0.0')), 0);
    });

    test('follows the precedence of SemVer 2.0.0 item 11 for pre-releases', () {
      // The example chain of the specification, each smaller than the next.
      const chain = [
        '1.0.0-alpha',
        '1.0.0-alpha.1',
        '1.0.0-alpha.beta',
        '1.0.0-beta',
        '1.0.0-beta.2',
        '1.0.0-beta.11',
        '1.0.0-rc.1',
        '1.0.0',
      ];
      for (var i = 0; i < chain.length - 1; i++) {
        expect(
          v(chain[i]).compareTo(v(chain[i + 1])),
          lessThan(0),
          reason: '${chain[i]} < ${chain[i + 1]}',
        );
        expect(v(chain[i + 1]).compareTo(v(chain[i])), greaterThan(0));
      }
    });

    test('ignores build metadata in the precedence', () {
      expect(v('1.0.0+1').compareTo(v('1.0.0+2')), 0);
      expect(v('1.0.0-rc.1+a') < v('1.0.0'), isTrue);
    });

    test('refuses what is no SemVer', () {
      for (final text in [
        '',
        '1',
        '1.2',
        '1.2.3.4',
        'v1.2.3',
        '01.2.3',
        '1.02.3',
        '1.2.3-',
        '1.2.3-01',
        '1.2.3-beta..1',
        '1.2.3+',
        ' 1.2.3',
        '1.2.x',
      ]) {
        expect(SemVer.tryParse(text), isNull, reason: text);
        expect(() => SemVer.parse(text), throwsFormatException, reason: text);
      }
    });
  });

  group('decideClientVersion', () {
    const rules = [
      ClientVersionRule(
        platform: 'ios',
        minVersion: '1.2.0',
        recommendedVersion: '1.4.0',
      ),
      ClientVersionRule(platform: 'android', minVersion: '2.0.0'),
      ClientVersionRule(
        platform: 'macos',
        minVersion: '0.0.0',
        recommendedVersion: '3.1.0',
      ),
    ];

    ClientVersionDecision decide(
      String platform,
      String appVersion, {
      List<ClientVersionRule> with_ = rules,
      void Function(String key, String value)? onUnparsable,
    }) => decideClientVersion(
      platform: platform,
      appVersion: appVersion,
      rules: with_,
      onUnparsable: onUnparsable,
    );

    test('below the minimum of its own platform the app is blocked', () {
      expect(decide('ios', '1.1.9'), const ClientVersionBelowMinimum('1.2.0'));
      expect(
        decide('android', '1.99.0'),
        const ClientVersionBelowMinimum('2.0.0'),
      );
    });

    test('a pre-release of the minimum is below it', () {
      expect(
        decide('ios', '1.2.0-beta.1'),
        const ClientVersionBelowMinimum('1.2.0'),
      );
    });

    test('at or above the minimum and below the recommendation: notice', () {
      expect(
        decide('ios', '1.2.0'),
        const ClientVersionUpdateRecommended('1.4.0'),
      );
      expect(
        decide('macos', '3.0.9'),
        const ClientVersionUpdateRecommended('3.1.0'),
      );
    });

    test('at or above every rule: allowed', () {
      expect(decide('ios', '1.4.0'), const ClientVersionAllowed());
      expect(decide('android', '2.0.0'), const ClientVersionAllowed());
    });

    test('the rules of other platforms never apply', () {
      expect(decide('windows', '0.0.1'), const ClientVersionAllowed());
      expect(decide('android', '1.3.0'), isA<ClientVersionBelowMinimum>());
    });

    test('an unparsable rule counts as absent and is reported', () {
      final seen = <(String, String)>[];
      final d = decide(
        'ios',
        '0.1.0',
        with_: const [
          ClientVersionRule(
            platform: 'ios',
            minVersion: 'one',
            recommendedVersion: '2.0',
          ),
        ],
        onUnparsable: (key, value) => seen.add((key, value)),
      );
      expect(d, const ClientVersionAllowed());
      expect(seen, [
        ('client.min_version.ios', 'one'),
        ('client.recommended_version.ios', '2.0'),
      ]);
    });

    test('an unparsable minimum leaves a valid recommendation in force', () {
      expect(
        decide(
          'ios',
          '1.0.0',
          with_: const [
            ClientVersionRule(
              platform: 'ios',
              minVersion: '',
              recommendedVersion: '1.1.0',
            ),
          ],
        ),
        const ClientVersionUpdateRecommended('1.1.0'),
      );
    });

    test('an app version that is no SemVer is never blocked', () {
      final seen = <String>[];
      expect(
        decide('ios', 'dev', onUnparsable: (key, _) => seen.add(key)),
        const ClientVersionAllowed(),
      );
      expect(seen, ['app version']);
    });

    test('a second rule for the same platform: the first one counts', () {
      expect(
        decide(
          'ios',
          '1.5.0',
          with_: const [
            ClientVersionRule(platform: 'ios', minVersion: '1.0.0'),
            ClientVersionRule(platform: 'ios', minVersion: '9.0.0'),
          ],
        ),
        const ClientVersionAllowed(),
      );
    });
  });
}
