// SPDX-License-Identifier: Apache-2.0

// The suite script of the mobile E2E run (brief M5-11 Design note 2; tool/mobile_e2e_suite.sh)
// with stand-ins for adb and patrol on PATH: every named suite runs as its own patrol run with
// the world as defines and the Gradle properties of the E2E debug build, the exit code is that of
// the first suite that failed, the emulator is cleaned up even when a clean-up step fails (the
// app is gone already), and a missing input ends it with 2 before anything runs.
@TestOn('mac-os || linux')
library;

import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  final script = '${Directory.current.path}/tool/mobile_e2e_suite.sh';
  late Directory tmp;
  late File calls;
  late Map<String, String> env;

  setUp(() {
    tmp = Directory.systemTemp.createTempSync('mobile-e2e-suite');
    calls = File('${tmp.path}/calls.log')..createSync();
    final bin = Directory('${tmp.path}/bin')..createSync();
    // adb answers like an emulator whose app the Gradle run removed already: `uninstall` fails.
    _tool(bin, 'adb', '''
echo "adb \$*" >>"${calls.path}"
case "\$*" in
*get-state*) echo device ;;
*uninstall*) exit 1 ;;
esac
exit 0
''');
    // patrol records its target, defines and Gradle properties and fails for FAKE_PATROL_FAIL.
    _tool(bin, 'patrol', '''
target=""
defines=""
while [ "\$#" -gt 0 ]; do
  case "\$1" in
  --target) target=\$2; shift ;;
  --dart-define) defines="\$defines \${2%%=*}=\${#2}"; shift ;;
  esac
  shift
done
echo "patrol \$target\$defines push=\${ORG_GRADLE_PROJECT_e2ePush:-} ca=\${ORG_GRADLE_PROJECT_e2eCa:-}" >>"${calls.path}"
case "\$target" in
*"/\${FAKE_PATROL_FAIL:-none}_e2e_test.dart") exit 7 ;;
esac
exit 0
''');
    File('${tmp.path}/e2e.json').writeAsStringSync('{"server_url":"x"}');
    File('${tmp.path}/ca.pem').writeAsStringSync('EXAMPLE CA');
    env = {
      'PATH': '${bin.path}:/usr/bin:/bin',
      'HOME': tmp.path,
      'E2E_CONFIG': '${tmp.path}/e2e.json',
      'E2E_CA': '${tmp.path}/ca.pem',
      'E2E_SERVER_URL': 'https://localhost:4321',
      'MOBILE_E2E_SERIAL': 'emulator-5580',
    };
  });

  tearDown(() => tmp.deleteSync(recursive: true));

  Future<ProcessResult> run(
    List<String> args, {
    Map<String, String> extra = const {},
  }) => Process.run(
    'sh',
    [script, ...args],
    environment: {...env, ...extra},
    includeParentEnvironment: false,
  );

  List<String> logged() => calls.readAsLinesSync();

  test(
    'android: every suite runs once, then the emulator is cleaned up',
    () async {
      final r = await run(['android']);
      expect(r.exitCode, 0, reason: '${r.stdout}${r.stderr}');
      final lines = logged();
      final patrol = [
        for (final l in lines)
          if (l.startsWith('patrol ')) l,
      ];
      final config = base64.encode(utf8.encode('{"server_url":"x"}')).length;
      final ca = base64.encode(utf8.encode('EXAMPLE CA')).length;
      expect(patrol, [
        for (final s in ['login', 'push', 'share', 'offline'])
          'patrol e2e_test/phone/${s}_e2e_test.dart '
              'E2E_CONFIG=${'E2E_CONFIG='.length + config} '
              'E2E_CA=${'E2E_CA='.length + ca} '
              'push=true ca=${tmp.path}/ca.pem',
      ]);
      expect(lines.first, 'adb -s emulator-5580 get-state');
      expect(lines, contains('adb -s emulator-5580 reverse tcp:4321 tcp:4321'));
      // The clean-up runs every step although the removal of the app fails.
      final last = lines.lastIndexWhere((l) => l.startsWith('patrol '));
      expect(lines.sublist(last + 1), [
        'adb -s emulator-5580 reverse --remove tcp:4321',
        'adb -s emulator-5580 shell rm -f /sdcard/Download/share-sample.jpg',
        'adb -s emulator-5580 uninstall com.example.chat',
        'adb -s emulator-5580 uninstall com.example.chat.test',
      ]);
      expect(
        '${r.stdout}',
        contains(
          'mobile-e2e android login: PASS\n'
          'mobile-e2e android push: PASS\n'
          'mobile-e2e android share: PASS\n'
          'mobile-e2e android offline: PASS',
        ),
      );
    },
  );

  test(
    'the exit code is that of the first failing suite; later suites still run',
    () async {
      final r = await run(
        ['android', 'push', 'offline'],
        extra: {'FAKE_PATROL_FAIL': 'push'},
      );
      expect(r.exitCode, 7, reason: '${r.stdout}${r.stderr}');
      expect(logged().where((l) => l.startsWith('patrol ')), hasLength(2));
      expect(
        '${r.stdout}',
        contains(
          'mobile-e2e android push: FAIL (exit 7)\n'
          'mobile-e2e android offline: PASS',
        ),
      );
      expect(
        logged().last,
        'adb -s emulator-5580 uninstall com.example.chat.test',
      );
    },
  );

  test('a missing input ends it with 2 before anything runs', () async {
    for (final (args, extra) in [
      (<String>[], <String, String>{}),
      (['android', 'logout'], <String, String>{}),
      (['android'], {'E2E_CONFIG': '${tmp.path}/missing.json'}),
      (['android'], {'E2E_SERVER_URL': 'https://chat.example.com'}),
      (['android'], {'MOBILE_E2E_SERIAL': ''}),
    ]) {
      final r = await run(args, extra: extra);
      expect(r.exitCode, 2, reason: '$args $extra: ${r.stderr}');
    }
    expect(logged(), isEmpty);
  });
}

/// Writes the stand-in [name] into [bin] with the shell [body].
void _tool(Directory bin, String name, String body) {
  final file = File('${bin.path}/$name')..writeAsStringSync('#!/bin/sh\n$body');
  Process.runSync('chmod', ['+x', file.path]);
}
