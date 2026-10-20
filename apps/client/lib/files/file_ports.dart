// SPDX-License-Identifier: Apache-2.0

/// The outside world of files and voice (brief M4-22 Scope and Design note 4): the file
/// picker and save dialog, the clipboard, opening a file with the system, the microphone and
/// the media players. The app takes them from `AppPorts.files`; tests replace them.
library;

import 'dart:io';

import 'package:file_selector/file_selector.dart' as selector;
import 'package:flutter/widgets.dart';
import 'package:pasteboard/pasteboard.dart';
import 'package:url_launcher/url_launcher.dart';

import '../voice/recorder.dart';
import 'ticket_player.dart';

/// A file the person picked, dropped or pasted: where it lies and its name.
final class PickedFile {
  /// Creates the file at [path] named [name].
  const PickedFile({required this.path, required this.name});

  /// The file on disk.
  final String path;

  /// The name to send (the last path segment; the server sanitizes it again, 12 6.2).
  final String name;
}

/// The open and save dialogs of the platform.
abstract interface class FilePicker {
  /// Lets the person pick files; empty when cancelled.
  Future<List<PickedFile>> pickFiles();

  /// Lets the person choose where to save a file named [suggestedName]; null when cancelled.
  Future<String?> saveLocation(String suggestedName);
}

/// The files of the clipboard (paste, brief M4-22 Scope).
abstract interface class PasteSource {
  /// The files on the clipboard, or an image written into [scratch] as a PNG; empty when the
  /// clipboard holds neither.
  Future<List<PickedFile>> read(Directory scratch);
}

/// Opens a local file with the system's default app.
abstract interface class FileOpener {
  /// Opens [path]; false when the system refused.
  Future<bool> open(String path);
}

/// Builds the video surface of a ticket URL (12 9.2); the URL is a bearer secret for a short
/// time, so the surface keeps it in memory only.
typedef VideoSurfaceBuilder = Widget Function(Uri ticketUrl);

/// The ports of files and voice.
///
/// Example: `FilePorts(picker: fakePicker, recorder: FakeRecorder.new)` in a test.
final class FilePorts {
  /// Creates the ports; every one has the platform implementation as its default.
  FilePorts({
    FilePicker? picker,
    PasteSource? paste,
    FileOpener? opener,
    VoiceRecorder Function()? recorder,
    TicketPlayer Function()? player,
    VideoSurfaceBuilder? video,
  }) : picker = picker ?? const SelectorFilePicker(),
       paste = paste ?? const PasteboardSource(),
       opener = opener ?? const LauncherFileOpener(),
       recorder = recorder ?? RecordVoiceRecorder.new,
       player = player ?? AudioTicketPlayer.new,
       video = video ?? ((url) => TicketVideoView(url: url));

  /// The open and save dialogs.
  final FilePicker picker;

  /// The clipboard.
  final PasteSource paste;

  /// Opens files with the system.
  final FileOpener opener;

  /// Creates a recorder for one recording (package `record`, 15 13).
  final VoiceRecorder Function() recorder;

  /// Creates an audio player for one playback.
  final TicketPlayer Function() player;

  /// Builds the video surface of a ticket URL.
  final VideoSurfaceBuilder video;
}

/// [FilePicker] over package `file_selector`.
final class SelectorFilePicker implements FilePicker {
  /// Creates the picker.
  const SelectorFilePicker();

  @override
  Future<List<PickedFile>> pickFiles() async => [
    for (final f in await selector.openFiles())
      PickedFile(path: f.path, name: f.name),
  ];

  @override
  Future<String?> saveLocation(String suggestedName) async =>
      (await selector.getSaveLocation(suggestedName: suggestedName))?.path;
}

/// [PasteSource] over package `pasteboard`: copied files, else a copied image.
final class PasteboardSource implements PasteSource {
  /// Creates the source.
  const PasteboardSource();

  @override
  Future<List<PickedFile>> read(Directory scratch) async {
    final paths = await Pasteboard.files();
    if (paths.isNotEmpty) {
      return [
        for (final p in paths)
          PickedFile(path: p, name: p.split(Platform.pathSeparator).last),
      ];
    }
    final image = await Pasteboard.image;
    if (image == null || image.isEmpty) return const [];
    await scratch.create(recursive: true);
    final name = 'pasted-${DateTime.now().toUtc().millisecondsSinceEpoch}.png';
    final file = File('${scratch.path}${Platform.pathSeparator}$name');
    await file.writeAsBytes(image, flush: true);
    return [PickedFile(path: file.path, name: name)];
  }
}

/// [FileOpener] over package `url_launcher` with a `file:` URI.
final class LauncherFileOpener implements FileOpener {
  /// Creates the opener.
  const LauncherFileOpener();

  @override
  Future<bool> open(String path) => launchUrl(Uri.file(path));
}
