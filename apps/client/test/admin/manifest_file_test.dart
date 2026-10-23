// SPDX-License-Identifier: Apache-2.0

// The desktop's manifest picker of the apps (brief M6-08 Design note 1; 11 4.1): the system file
// dialog with the types of 11 3.1, its file handed over by name and bytes, a cancel as null.

import 'dart:convert';
import 'dart:typed_data';

import 'package:client/admin/manifest_file.dart';
import 'package:file_selector/file_selector.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'the dialog offers JSON and YAML and hands over name and bytes',
    () async {
      List<XTypeGroup>? offered;
      final pick = manifestPicker(
        label: 'App manifest',
        open: ({List<XTypeGroup> acceptedTypeGroups = const []}) async {
          offered = acceptedTypeGroups;
          return XFile.fromData(
            Uint8List.fromList(utf8.encode('slug: repo-bridge\n')),
            // On the VM an XFile names itself after its path.
            path: '/home/alex/repo-bridge.yaml',
          );
        },
      );
      final file = await pick();
      expect(file!.name, 'repo-bridge.yaml');
      expect(utf8.decode(file.bytes), 'slug: repo-bridge\n');
      expect(offered!.single.label, 'App manifest');
      expect(offered!.single.extensions, ['json', 'yaml', 'yml']);
    },
  );

  test('a cancelled dialog gives null', () async {
    final pick = manifestPicker(
      label: 'App manifest',
      open: ({List<XTypeGroup> acceptedTypeGroups = const []}) async => null,
    );
    expect(await pick(), isNull);
  });
}
