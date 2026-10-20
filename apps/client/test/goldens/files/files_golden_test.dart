// SPDX-License-Identifier: Apache-2.0

// The goldens of files and voice (brief M4-22 Tests; 12 2; 15 13): the attachment tiles in
// every state of 12 2 with the upload chips, the image preview from the server, and the voice
// player with its transcript; light and dark, densities comfortable and compact, text scaling
// 1 and 2, German and English. One file per scene and theme keeps each file small.

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:alchemist/alchemist.dart';
import 'package:client/files/attachment_mapping.dart';
import 'package:client/files/attachment_tile.dart';
import 'package:client/files/download_service.dart';
import 'package:client/files/image_preview.dart';
import 'package:client/files/providers.dart';
import 'package:client/voice/voice_player.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/file_fakes.dart';
import '../../support/file_widgets.dart';

/// A small image the previews show; written once per run into a temporary folder.
final File _png = () {
  final dir = Directory.systemTemp.createTempSync('files-golden');
  final file = File('${dir.path}/shot.png');
  // A 4 by 4 PNG in one colour, so the preview is a flat square in every renderer.
  file.writeAsBytesSync(
    base64.decode(
      'iVBORw0KGgoAAAANSUhEUgAAAAQAAAAECAIAAAAmkwkpAAAAEUlEQVR4nGPwmvAIjhiI4wAAyFIbwcAdOK4AAAAASUVORK5CYII=',
    ),
  );
  return file;
}();

final FileMeta _image = fileOf(1, name: 'screenshot.png', mime: 'image/png');
final FileMeta _checkingImage = fileOf(
  2,
  name: 'diagram.png',
  mime: 'image/png',
);
final FileMeta _blockedImage = fileOf(3, name: 'photo.png', mime: 'image/png');
final FileMeta _voiceDone = fileOf(
  4,
  name: 'voice.m4a',
  mime: 'audio/mp4',
  durationMs: 12000,
  transcript: TranscriptStatus.done,
);
final FileMeta _voicePending = fileOf(
  5,
  name: 'voice.m4a',
  mime: 'audio/mp4',
  durationMs: 4000,
  transcript: TranscriptStatus.pending,
);

Widget _scope(Widget child) => ProviderScope(
  overrides: [
    filePortsProvider.overrideWithValue(fakeFilePorts()),
    thumbnailProvider(
      _image.id.value,
      'medium',
    ).overrideWithValue(AsyncValue.data(Downloaded(_png))),
    thumbnailProvider(
      _checkingImage.id.value,
      'medium',
    ).overrideWithValue(const AsyncValue.data(DownloadNotReady())),
    thumbnailProvider(
      _blockedImage.id.value,
      'medium',
    ).overrideWithValue(const AsyncValue.data(DownloadBlocked())),
    cachedBytesProvider(_png.path)
        .overrideWithValue(AsyncValue.data(_png.readAsBytesSync())),
    transcriptTextProvider(_voiceDone.id.value).overrideWithValue(
      const AsyncValue.data('Please call Sam back tomorrow.'),
    ),
  ],
  child: child,
);

Widget _cell(Size size, Widget child) => Builder(
  builder: (context) => ColoredBox(
    color: context.dsColors.surfaceCanvas,
    child: SizedBox.fromSize(
      size: size,
      child: Padding(
        padding: const EdgeInsets.all(DsSpace.s12),
        child: SingleChildScrollView(child: child),
      ),
    ),
  ),
);

/// Every view of 12 2: stored files and upload chips.
Widget tiles() => _scope(
  _cell(
    const Size(560, 900),
    Wrap(
      spacing: DsSpace.s6,
      runSpacing: DsSpace.s6,
      children: [
        AttachmentTile(file: fileOf(10, name: 'report.pdf')),
        AttachmentTile(
          file: fileOf(
            11,
            name: 'export.csv',
            upload: UploadState.pending,
            scan: ScanStatus.pending,
          ),
        ),
        AttachmentTile(
          file: fileOf(12, name: 'build.log', upload: UploadState.failed),
        ),
        AttachmentTile(
          file: fileOf(13, name: 'setup.zip', scan: ScanStatus.infected),
        ),
        AttachmentTile(file: deletedFile(14)),
        const UploadChip(
          attachmentKey: 'a',
          name: 'notes.txt',
          sizeBytes: 2048,
          state: AttachmentState.waiting,
        ),
        const UploadChip(
          attachmentKey: 'b',
          name: 'video.mp4',
          sizeBytes: 5 * 1024 * 1024,
          state: AttachmentState.uploading,
          percent: 40,
        ),
        UploadChip(
          attachmentKey: 'c',
          name: 'draft.docx',
          sizeBytes: 81920,
          state: AttachmentState.failed,
          reason: 'A part was sent twice with different content',
          onRetry: () {},
          onRemove: () {},
        ),
        const UploadChip(
          attachmentKey: 'd',
          name: 'old.tar',
          sizeBytes: 900,
          state: AttachmentState.canceled,
        ),
      ],
    ),
  ),
);

/// The image preview: a thumbnail from the server, "being checked" and "blocked".
Widget images() => _scope(
  _cell(
    const Size(560, 700),
    Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AttachmentTile(file: _image),
        const SizedBox(height: DsSpace.s8),
        ImagePreview(file: _checkingImage),
        const SizedBox(height: DsSpace.s8),
        ImagePreview(file: _blockedImage),
      ],
    ),
  ),
);

/// The voice player with a transcript, and one whose transcript is pending.
Widget voice() => _scope(
  _cell(
    const Size(560, 420),
    Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        VoicePlayer(file: _voiceDone),
        const SizedBox(height: DsSpace.s12),
        VoicePlayer(file: _voicePending),
      ],
    ),
  ),
);

/// Largest share of differing pixels a golden of this file may show (one pixel in 100,000).
///
/// The rasterizer anti-aliases the close icon of a tile one colour step apart on macOS and on
/// the Linux runner, so an exact CI golden cannot match both hosts (docs/DECISIONS.md D-M4-22-12).
const double _tolerance = 0.00001;

/// A [LocalFileComparator] that passes a comparison whose difference stays within [_tolerance].
class _TolerantComparator extends LocalFileComparator {
  _TolerantComparator(super.testFile);

  @override
  Future<bool> compare(Uint8List imageBytes, Uri golden) async {
    final result = await GoldenFileComparator.compareLists(
      imageBytes,
      await getGoldenBytes(golden),
    );
    if (result.passed || result.diffPercent <= _tolerance) {
      result.dispose();
      return true;
    }
    final error = await generateFailureOutput(result, golden, basedir);
    result.dispose();
    throw FlutterError(error);
  }
}

void main() {
  setUpAll(() {
    final previous = goldenFileComparator;
    if (previous is LocalFileComparator) {
      goldenFileComparator = _TolerantComparator(
        previous.basedir.resolve('files_golden_test.dart'),
      );
    }
  });
  final scenes = {
    'attachment_tiles': ('attachment tiles in every state of 12 2', tiles),
    'image_preview': ('image preview from the server', images),
    'voice_player': ('voice player with transcript', voice),
  };
  for (final theme in DsThemeMode.values) {
    for (final MapEntry(:key, value: (title, builder)) in scenes.entries) {
      goldenTest(
        'the $title in ${theme.name}',
        fileName: '${key}_${theme.name}',
        pumpBeforeTest: precacheImages,
        builder: () => dsGoldenMatrix(
          name: title,
          themes: [theme],
          densities: const [DsDensityMode.comfortable, DsDensityMode.compact],
          textScales: const [1.0, 2.0],
          locales: const [Locale('de'), Locale('en')],
          builder: builder,
        ),
      );
    }
  }
}
