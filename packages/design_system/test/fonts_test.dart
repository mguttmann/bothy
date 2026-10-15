// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

// flutter test runs with the package folder as the working directory.
final _fonts = Directory('fonts');
const _manifestName = 'MANIFEST.json';
const _license = 'OFL-1.1';

Map<String, dynamic> _manifest() =>
    jsonDecode(File('${_fonts.path}/$_manifestName').readAsStringSync())
        as Map<String, dynamic>;

Map<String, Map<String, dynamic>> _files() =>
    (_manifest()['files'] as Map<String, dynamic>).map(
      (k, v) => MapEntry(k, v as Map<String, dynamic>),
    );

/// The problems of the bundled font folder, as `fonts_test` reports them.
List<String> fontProblems(
  Directory dir,
  Map<String, Map<String, dynamic>> files,
) {
  final present = {
    for (final f in dir.listSync().whereType<File>())
      if (!f.path.endsWith(_manifestName)) f.uri.pathSegments.last,
  };
  final problems = <String>[
    for (final name in present.difference(files.keys.toSet()))
      '$name: not listed in $_manifestName',
    for (final name in files.keys.toSet().difference(present))
      '$name: listed in $_manifestName but missing',
  ];
  for (final MapEntry(key: name, value: entry) in files.entries) {
    if (!present.contains(name)) continue;
    final digest = sha256
        .convert(File('${dir.path}/$name').readAsBytesSync())
        .toString();
    if (digest != entry['sha256']) {
      problems.add('$name: sha256 differs from $_manifestName');
    }
    if (entry['license'] != _license) {
      problems.add('$name: license is not $_license');
    }
    final licenseFile = entry['license_file'];
    if (name.endsWith('.ttf') &&
        (licenseFile is! String || !present.contains(licenseFile))) {
      problems.add('$name: no license text next to it');
    }
    for (final key in ['source', 'commit']) {
      if (entry[key] is! String || (entry[key] as String).isEmpty) {
        problems.add('$name: no $key');
      }
    }
  }
  return problems..sort();
}

void main() {
  test('every bundled font file matches the manifest', () {
    expect(fontProblems(_fonts, _files()), isEmpty);
  });

  test('the four variable files and two license texts are bundled', () {
    expect(_files().keys.toSet(), {
      'AtkinsonHyperlegibleNext-VariableFont_wght.ttf',
      'AtkinsonHyperlegibleNext-Italic-VariableFont_wght.ttf',
      'AtkinsonHyperlegibleMono-VariableFont_wght.ttf',
      'AtkinsonHyperlegibleMono-Italic-VariableFont_wght.ttf',
      'AtkinsonHyperlegibleNext.OFL.txt',
      'AtkinsonHyperlegibleMono.OFL.txt',
    });
  });

  group('the check fails', () {
    late Directory tmp;
    setUp(() {
      tmp = Directory.systemTemp.createTempSync('ds-fonts-');
      for (final f in _fonts.listSync().whereType<File>()) {
        f.copySync('${tmp.path}/${f.uri.pathSegments.last}');
      }
    });
    tearDown(() => tmp.deleteSync(recursive: true));

    test('on a changed file', () {
      File('${tmp.path}/AtkinsonHyperlegibleMono.OFL.txt')
          .writeAsStringSync('changed\n', mode: FileMode.append);
      expect(fontProblems(tmp, _files()), [
        'AtkinsonHyperlegibleMono.OFL.txt: sha256 differs from MANIFEST.json',
      ]);
    });

    test('on a missing and an extra file', () {
      File('${tmp.path}/AtkinsonHyperlegibleMono-Italic-VariableFont_wght.ttf')
          .deleteSync();
      File('${tmp.path}/Extra.ttf').writeAsBytesSync([0]);
      expect(fontProblems(tmp, _files()), [
        'AtkinsonHyperlegibleMono-Italic-VariableFont_wght.ttf: listed in MANIFEST.json but missing',
        'Extra.ttf: not listed in MANIFEST.json',
      ]);
    });

    test('on another license or a font without license text', () {
      final files = _files();
      files['AtkinsonHyperlegibleNext-VariableFont_wght.ttf'] = {
        ...files['AtkinsonHyperlegibleNext-VariableFont_wght.ttf']!,
        'license': 'MIT',
      };
      File('${tmp.path}/AtkinsonHyperlegibleMono.OFL.txt').deleteSync();
      files.remove('AtkinsonHyperlegibleMono.OFL.txt');
      expect(fontProblems(tmp, files), [
        'AtkinsonHyperlegibleMono-Italic-VariableFont_wght.ttf: no license text next to it',
        'AtkinsonHyperlegibleMono-VariableFont_wght.ttf: no license text next to it',
        'AtkinsonHyperlegibleNext-VariableFont_wght.ttf: license is not OFL-1.1',
      ]);
    });
  });

  test('pubspec.yaml declares both families with every font file', () {
    final pubspec = File('pubspec.yaml').readAsStringSync();
    expect(pubspec, contains('- family: ${DsFonts.familySans.first}'));
    expect(pubspec, contains('- family: ${DsFonts.familyMono.first}'));
    for (final name in _files().keys.where((n) => n.endsWith('.ttf'))) {
      expect(pubspec, contains('asset: fonts/$name'));
    }
  });

  testWidgets(
    'font family resolves: DsType.body uses Atkinson Hyperlegible Next',
    (tester) async {
      await tester.pumpWidget(
        const Directionality(
          textDirection: TextDirection.ltr,
          child: Text('Alex', style: DsType.body),
        ),
      );
      final paragraph = tester.renderObject<RenderParagraph>(find.text('Alex'));
      expect(paragraph.text.style!.fontFamily, 'Atkinson Hyperlegible Next');
      expect(DsType.code.fontFamily, 'Atkinson Hyperlegible Mono');
    },
  );

  test('dsTextStyle keeps the family and adds the package name and the weight axis', () {
    final style = dsTextStyle(DsType.title);
    expect(style.fontFamily, DsFonts.familySans.first);
    expect(
      style.fontFamilyFallback!.first,
      'packages/design_system/${DsFonts.familySans.first}',
    );
    expect(style.fontFamilyFallback!.skip(1), DsType.title.fontFamilyFallback);
    expect(style.fontVariations, [
      FontVariation.weight(DsType.title.fontWeight!.value.toDouble()),
    ]);
    expect(dsTextStyle(const TextStyle()).fontFamilyFallback, isEmpty);
  });
}
