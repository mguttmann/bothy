// SPDX-License-Identifier: Apache-2.0

// The build branding tool (brief M4-18 Design note 7): the checked-in native files match
// branding.example.yaml, the defines carry every key, and a bad branding is refused.

import 'dart:convert';
import 'dart:io';

import 'package:client/app/branding.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../tool/branding_defines.dart' as tool;

void main() {
  final app = Directory.current;

  test('the checked-in native files equal the example branding', () {
    final out = StringBuffer();
    expect(
      tool.run(['--check'], appDir: app, out: out),
      0,
      reason: '$out',
    );
  });

  test('the defaults of the build equal the example branding', () {
    final b = tool.Branding.parse(
      File('${app.path}/branding.example.yaml').readAsStringSync(),
      appVersion: '0.0.0',
    );
    const env = BuildBranding.fromEnvironment;
    expect(env.appName, b.appName);
    expect(env.urlScheme, b.urlScheme);
    expect(env.bundleId, b.bundleId);
    expect(env.defaultServer, b.defaultServer);
    expect(Uri.parse(b.defaultServer).host, endsWith('example.com'));
  });

  test('a run writes the defines and the native files', () async {
    final dir = await Directory.systemTemp.createTemp('branding');
    addTearDown(() => dir.delete(recursive: true));
    File('${dir.path}/pubspec.yaml')
        .writeAsStringSync('name: x\nversion: 1.2.3+4\n');
    File('${dir.path}/b.yaml').writeAsStringSync(
      'app_name: Team Chat\nurl_scheme: team-chat\nbundle_id: com.example.team\n'
      'default_server: https://team.example.com\n',
    );
    final out = StringBuffer();
    expect(tool.run(['--input', 'b.yaml'], appDir: dir, out: out), 0);
    final defines = jsonDecode(
      File('${dir.path}/build/branding/dart_defines.json').readAsStringSync(),
    ) as Map<String, Object?>;
    expect(defines, {
      'BRANDING_APP_NAME': 'Team Chat',
      'BRANDING_URL_SCHEME': 'team-chat',
      'BRANDING_BUNDLE_ID': 'com.example.team',
      'BRANDING_DEFAULT_SERVER': 'https://team.example.com',
      'BRANDING_APP_VERSION': '1.2.3',
    });
    expect(
      File('${dir.path}/macos/Runner/Configs/Branding.xcconfig')
          .readAsStringSync(),
      contains('BRANDING_URL_SCHEME = team-chat'),
    );
    expect(
      File('${dir.path}/windows/runner/branding.h').readAsStringSync(),
      contains('#define BRANDING_APP_NAME_W L"Team Chat"'),
    );
    expect(
      tool.run(['--input', 'b.yaml', '--check'], appDir: dir, out: out),
      0,
    );
    File('${dir.path}/windows/runner/branding.h').writeAsStringSync('x');
    expect(
      tool.run(['--input', 'b.yaml', '--check'], appDir: dir, out: out),
      1,
    );
    expect(tool.run(['--bogus'], appDir: dir, out: out), 2);
  });

  test('a bad branding is refused with its reason', () {
    String? reason(String yaml) {
      try {
        tool.Branding.parse(yaml, appVersion: '1.0.0');
        return null;
      } on FormatException catch (e) {
        return e.message;
      }
    }

    const ok =
        'app_name: A\nurl_scheme: a-b\nbundle_id: com.example.a\n'
        'default_server: https://a.example.com\n';
    expect(reason(ok), isNull);
    expect(reason('x'), contains('no map'));
    expect(reason('app_name: A\n'), contains('url_scheme'));
    expect(reason(ok.replaceFirst('A\n', 'A"; rm\n')), contains('app_name'));
    expect(reason(ok.replaceFirst('a-b', 'A_B')), contains('url_scheme'));
    expect(
      reason(ok.replaceFirst('com.example.a', 'nodots')),
      contains('bundle_id'),
    );
    expect(
      reason(ok.replaceFirst('https://a', 'http://a')),
      contains('default_server'),
    );
    expect(() => tool.pubspecVersion('name: x\n'), throwsFormatException);
  });
}
