// SPDX-License-Identifier: Apache-2.0

// The goldens of the document viewer (ticket FEAT-markdown-viewer): the sample Markdown with
// every element the viewer renders, on the desktop (with the table of contents as a side panel)
// and on the phone, in light and dark.

import 'package:alchemist/alchemist.dart';
import 'package:client/files/viewer/document_content.dart';
import 'package:client/files/viewer/document_view.dart';
import 'package:client/files/viewer/document_viewer.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../../support/document_samples.dart';

/// The viewer of the sample in a window of [size].
Widget _viewer(Size size, {required bool desktop}) => Builder(
  builder: (context) => MediaQuery(
    data: MediaQuery.of(context).copyWith(size: size),
    child: SizedBox.fromSize(
      size: size,
      child: DocumentView(
        name: 'release-notes.md',
        content: DocumentContent.prepare(
          sampleMarkdown,
          DocumentKind.markdown,
          totalBytes: sampleMarkdown.length,
        ),
        desktop: desktop,
        onClose: () {},
        onOpenLink: (_) {},
        // The relative link and image name other attachments, which a golden has none of.
        images: (uri) =>
            uri.scheme == 'https' ? RemoteImageOnClick(uri: uri) : null,
        onSave: () async => null,
        onOpenExternally: () async => null,
      ),
    ),
  ),
);

void main() {
  final scenes = {
    'document_viewer_desktop': (
      'document viewer on the desktop',
      () => _viewer(const Size(1100, 1300), desktop: true),
    ),
    'document_viewer_phone': (
      'document viewer on the phone',
      () => _viewer(const Size(390, 2000), desktop: false),
    ),
  };
  for (final theme in DsThemeMode.values) {
    for (final MapEntry(:key, value: (title, builder)) in scenes.entries) {
      goldenTest(
        'the $title in ${theme.name}',
        fileName: '${key}_${theme.name}',
        pumpBeforeTest: dsPumpIcons,
        builder: () =>
            dsGoldenMatrix(name: title, themes: [theme], builder: builder),
      );
    }
  }
}
