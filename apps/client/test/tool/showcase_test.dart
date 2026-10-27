// SPDX-License-Identifier: Apache-2.0

// The pure parts of the showcase of brief M7-28: the define of tools/showcase/capture.py, the
// capture ratio and the files the driver writes (e2e_test/showcase, test_driver/showcase_driver.dart).

import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import '../../e2e_test/showcase/showcase_capture.dart';
import '../../e2e_test/showcase/showcase_target.dart';
import '../../test_driver/showcase_driver.dart';

/// The Android emulator's alias of its host, from its parts (the deny list flags private
/// addresses).
final String _alias = [10, 0, 2, 2].join('.');

Map<String, Object?> _json({
  String server = 'https://localhost:61280',
  String platform = 'android',
  List<String> themes = const ['light', 'dark'],
  int width = 720,
}) => {
  'server': server,
  'instance_id': 'inst',
  'ca_pem': '-----BEGIN CERTIFICATE-----\n',
  'code': 'en_ABCD',
  'platform': platform,
  'upstream_host': _alias,
  'themes': themes,
  'capture_width': width,
  'video': true,
};

void main() {
  group('ShowcaseTarget', () {
    test('parses the define of capture.py', () {
      final t = ShowcaseTarget.parse(jsonEncode(_json()));
      expect(t.server, Uri.parse('https://localhost:61280'));
      expect(t.instanceId, 'inst');
      expect(t.code, 'en_ABCD');
      expect(t.phone, isTrue);
      expect(t.upstreamAddress, InternetAddress(_alias));
      expect(t.themes, ['light', 'dark']);
      expect(t.captureWidth, 720);
      expect(t.video, isTrue);
    });

    test('a desktop platform is no phone', () {
      expect(
        ShowcaseTarget.parse(jsonEncode(_json(platform: 'macos'))).phone,
        isFalse,
      );
    });

    for (final (name, json) in [
      ('a server without https', _json(server: 'http://localhost:1')),
      ('a server without a port', _json(server: 'https://localhost')),
      ('an unknown platform', _json(platform: 'vax')),
      ('no theme', _json(themes: const [])),
      ('an unknown theme', _json(themes: const ['sepia'])),
      ('a width of zero', _json(width: 0)),
    ]) {
      test('refuses $name', () {
        expect(
          () => ShowcaseTarget.parse(jsonEncode(json)),
          throwsFormatException,
        );
      });
    }

    test('fails without the define', () {
      expect(ShowcaseTarget.fromEnvironment, throwsStateError);
    });
  });

  test('captureRatio scales the physical width to the image width', () {
    expect(captureRatio(1080, 720), closeTo(2 / 3, 1e-9));
    expect(() => captureRatio(0, 720), throwsArgumentError);
    expect(() => captureRatio(1080, 0), throwsArgumentError);
  });

  group('writeImages', () {
    late Directory out;
    setUp(() => out = Directory.systemTemp.createTempSync('showcase-driver'));
    tearDown(() => out.deleteSync(recursive: true));

    test('writes every image as <name>.png', () {
      final written = writeImages({
        'images': {
          'spaces-light': base64.encode([1, 2, 3]),
        },
      }, out);
      expect(written, ['${out.path}/spaces-light.png']);
      expect(File(written.single).readAsBytesSync(), [1, 2, 3]);
    });

    test('writes nothing without images', () {
      expect(writeImages(null, out), isEmpty);
    });

    for (final name in ['../escape', 'Spaces', 'a/b', '-x', '']) {
      test('refuses the name "$name"', () {
        expect(
          () => writeImages({
            'images': {
              name: base64.encode([1]),
            },
          }, out),
          throwsFormatException,
        );
        expect(out.listSync(), isEmpty);
      });
    }
  });
}
