// SPDX-License-Identifier: Apache-2.0

// The build branding tool (brief M4-18 Design note 7, brief M5-02 Design note 1, brief M5-03
// Design note 1): the checked-in native files match branding.example.yaml, the defines carry
// every key, the iOS build gets its name, identifier, scheme, team and associated domain, the
// Android build its application ID, name, scheme and App Links host, and a bad branding is
// refused.

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
    expect(env.applicationId, b.applicationId);
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
      'BRANDING_ASSOCIATED_DOMAIN': '',
      'BRANDING_APPLICATION_ID': 'com.example.team',
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

  test('the iOS build gets name, identifier, scheme, team and domain', () async {
    final dir = await Directory.systemTemp.createTemp('branding');
    addTearDown(() => dir.delete(recursive: true));
    File('${dir.path}/pubspec.yaml')
        .writeAsStringSync('name: x\nversion: 1.2.3+4\n');
    const base =
        'app_name: Team Chat\nurl_scheme: team-chat\nbundle_id: com.example.team\n'
        'default_server: https://team.example.com\n';
    File('${dir.path}/b.yaml').writeAsStringSync(
      '${base}associated_domain: team.example.com\ndevelopment_team: ABCDE12345\n',
    );
    File('${dir.path}/plain.yaml').writeAsStringSync(base);
    final out = StringBuffer();
    expect(tool.run(['--input', 'b.yaml'], appDir: dir, out: out), 0);
    final xcconfig = File('${dir.path}/ios/Runner/Configs/Branding.xcconfig')
        .readAsStringSync();
    expect(xcconfig, contains('BRANDING_APP_NAME = Team Chat'));
    expect(xcconfig, contains('BRANDING_BUNDLE_ID = com.example.team'));
    expect(xcconfig, contains('BRANDING_URL_SCHEME = team-chat'));
    expect(xcconfig, contains('BRANDING_DEVELOPMENT_TEAM = ABCDE12345'));
    final entitlements = File('${dir.path}/ios/Runner/Branding.entitlements')
        .readAsStringSync();
    expect(entitlements, contains('com.apple.developer.associated-domains'));
    expect(
      entitlements,
      contains('<string>webcredentials:team.example.com</string>'),
    );
    // M5-04: a signed build shares an App Group and a keychain group with its Notification
    // Service Extension and receives pushes; the extension gets the same groups.
    expect(xcconfig, contains('CLIENTKIT_APP_GROUP = group.com.example.team'));
    expect(
      xcconfig,
      contains('CLIENTKIT_KEYCHAIN_GROUP = ABCDE12345.com.example.team.shared'),
    );
    for (final file in [
      'ios/Runner/Branding.entitlements',
      'ios/NotificationService/Branding.entitlements',
      'ios/ShareExtension/Branding.entitlements',
    ]) {
      final text = File('${dir.path}/$file').readAsStringSync();
      expect(text, contains('<string>group.com.example.team</string>'));
      expect(
        text,
        contains('<string>ABCDE12345.com.example.team.shared</string>'),
      );
    }
    expect(entitlements, contains('<key>aps-environment</key>'));
    // M5-06: a signed build signs every target with its branding entitlements, also on the
    // simulator, so it never needs the simulator groups.
    expect(xcconfig, contains('CLIENTKIT_ENTITLEMENTS = Branding\n'));
    expect(xcconfig, isNot(contains('CLIENTKIT_SIMULATOR')));
    expect(
      entitlements,
      contains('<string>ABCDE12345.com.example.team</string>'),
      reason: 'the app keeps its own default keychain group first',
    );
    final defines = jsonDecode(
      File('${dir.path}/build/branding/dart_defines.json').readAsStringSync(),
    ) as Map<String, Object?>;
    expect(defines['BRANDING_ASSOCIATED_DOMAIN'], 'team.example.com');

    // Without a domain the build has no associated domain entitlement and no passkeys
    // (15 8; D-M5-02-5); without a team the build is unsigned.
    expect(tool.run(['--input', 'plain.yaml'], appDir: dir, out: out), 0);
    expect(
      File('${dir.path}/ios/Runner/Branding.entitlements').readAsStringSync(),
      isNot(contains('associated-domains')),
    );
    expect(
      File('${dir.path}/ios/Runner/Configs/Branding.xcconfig')
          .readAsStringSync(),
      contains('BRANDING_DEVELOPMENT_TEAM =\n'),
    );
    // Without a team there is no App Group, no keychain group and no push.
    expect(
      File('${dir.path}/ios/Runner/Configs/Branding.xcconfig')
          .readAsStringSync(),
      allOf(
        contains('CLIENTKIT_APP_GROUP =\n'),
        contains('CLIENTKIT_KEYCHAIN_GROUP =\n'),
      ),
    );
    for (final file in [
      'ios/NotificationService/Branding.entitlements',
      'ios/ShareExtension/Branding.entitlements',
    ]) {
      expect(
        File('${dir.path}/$file').readAsStringSync(),
        allOf(
          isNot(contains('application-groups')),
          isNot(contains('aps-environment')),
        ),
      );
    }
    // M5-06: an unsigned simulator build gets the groups only when the share device tests ask
    // for them with CLIENTKIT_SIMULATOR_GROUPS=YES in the environment; the simulator checks no
    // signature, a device build stays without groups.
    final unsigned = File('${dir.path}/ios/Runner/Configs/Branding.xcconfig')
        .readAsStringSync();
    for (final line in [
      'CLIENTKIT_ENTITLEMENTS = Branding\n',
      'CLIENTKIT_APP_GROUP[sdk=iphonesimulator*] = '
          r'$(CLIENTKIT_SIMULATOR_APP_GROUP_$(CLIENTKIT_SIMULATOR_GROUPS))',
      'CLIENTKIT_KEYCHAIN_GROUP[sdk=iphonesimulator*] = '
          r'$(CLIENTKIT_SIMULATOR_KEYCHAIN_GROUP_$(CLIENTKIT_SIMULATOR_GROUPS))',
      'CLIENTKIT_ENTITLEMENTS[sdk=iphonesimulator*] = '
          r'$(CLIENTKIT_SIMULATOR_ENTITLEMENTS_$(CLIENTKIT_SIMULATOR_GROUPS))',
      'CLIENTKIT_SIMULATOR_APP_GROUP_YES = group.com.example.team\n',
      'CLIENTKIT_SIMULATOR_KEYCHAIN_GROUP_YES = com.example.team.shared\n',
      'CLIENTKIT_SIMULATOR_ENTITLEMENTS_YES = SimulatorGroups\n',
      'CLIENTKIT_SIMULATOR_ENTITLEMENTS_ = Branding\n',
    ]) {
      expect(unsigned, contains(line));
    }
    expect(
      tool.run(['--input', 'plain.yaml', '--check'], appDir: dir, out: out),
      0,
    );
    expect(
      tool.run(['--input', 'b.yaml', '--check'], appDir: dir, out: out),
      1,
      reason: 'the iOS files of the other branding differ',
    );
  });

  test('the Android build gets application ID, name, scheme and App Links host', () async {
    final dir = await Directory.systemTemp.createTemp('branding');
    addTearDown(() => dir.delete(recursive: true));
    File('${dir.path}/pubspec.yaml')
        .writeAsStringSync('name: x\nversion: 1.2.3+4\n');
    const base =
        'app_name: Team Chat\nurl_scheme: team-chat\nbundle_id: com.example.team-chat\n'
        'default_server: https://team.example.com\napplication_id: com.example.teamchat\n';
    File('${dir.path}/b.yaml')
        .writeAsStringSync('${base}associated_domain: team.example.com\n');
    File('${dir.path}/plain.yaml').writeAsStringSync(base);
    final out = StringBuffer();
    expect(tool.run(['--input', 'b.yaml'], appDir: dir, out: out), 0);
    String properties() =>
        File('${dir.path}/android/branding.properties').readAsStringSync();
    expect(properties(), contains('applicationId=com.example.teamchat\n'));
    expect(properties(), contains('appName=Team Chat\n'));
    expect(properties(), contains('urlScheme=team-chat\n'));
    expect(properties(), contains('appLinksHost=team.example.com\n'));
    final defines = jsonDecode(
      File('${dir.path}/build/branding/dart_defines.json').readAsStringSync(),
    ) as Map<String, Object?>;
    expect(defines['BRANDING_APPLICATION_ID'], 'com.example.teamchat');
    expect(defines['BRANDING_BUNDLE_ID'], 'com.example.team-chat');

    // Without a public domain the build has no App Links (D-M5-03-2).
    expect(tool.run(['--input', 'plain.yaml'], appDir: dir, out: out), 0);
    expect(properties(), contains('appLinksHost=\n'));
    expect(
      tool.run(['--input', 'b.yaml', '--check'], appDir: dir, out: out),
      1,
      reason: 'the Android file of the other branding differs',
    );
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
    expect(
      reason('${ok}associated_domain: https://a.example.com\n'),
      contains('associated_domain'),
    );
    expect(
      reason('${ok}development_team: abc\n'),
      contains('development_team'),
    );
    // The application ID is also the scheme of the OIDC redirect: lower-case letters and
    // digits per segment; a bundle identifier with a hyphen needs its own application_id.
    expect(
      reason(ok.replaceFirst('com.example.a', 'com.example.a-b')),
      contains('application_id'),
    );
    expect(
      reason('${ok}application_id: com.example.A\n'),
      contains('application_id'),
    );
    expect(
      reason('${ok}application_id: com.1example\n'),
      contains('application_id'),
    );
    expect(reason('${ok}application_id: com.example.ab\n'), isNull);
    expect(() => tool.pubspecVersion('name: x\n'), throwsFormatException);
  });
}
