// SPDX-License-Identifier: Apache-2.0

/// Attaching files in the composer (brief M4-22 Scope and Design note 2; 15 14, 04 13.2): the
/// attachment button, drag and drop and paste hand files to [AttachController], which checks
/// the size against `file_max_upload_bytes` and the count against `message_max_attachments`
/// before anything is written, and puts one upload entry per file into the outbox, before the
/// message that will attach it. [AttachDropTarget] and [AttachPasteShortcut] are the drop
/// area and the paste key of the composer.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:desktop_drop/desktop_drop.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:sync_engine/sync_engine.dart';

import '../voice/recorder.dart';
import 'file_cache.dart';
import 'file_ports.dart';
import 'upload_executor.dart';
import 'upload_planner.dart';

/// Why files were refused before any entry was written.
sealed class AttachRefusal {
  const AttachRefusal();
}

/// [name] is larger than `file_max_upload_bytes` ([limitBytes]).
final class FileTooLarge extends AttachRefusal {
  /// Creates the refusal.
  const FileTooLarge(this.name, this.limitBytes);

  /// The file name (a foreign name).
  final String name;

  /// The limit in bytes.
  final int limitBytes;
}

/// More than `message_max_attachments` ([limit]) files for one message.
final class TooManyFiles extends AttachRefusal {
  /// Creates the refusal.
  const TooManyFiles(this.limit);

  /// The limit.
  final int limit;
}

/// [name] cannot be read, or is empty (12 3.1: `size_bytes` from 1).
final class FileUnreadable extends AttachRefusal {
  /// Creates the refusal.
  const FileUnreadable(this.name);

  /// The file name.
  final String name;
}

/// A file attached in the composer whose upload entry is in the outbox.
final class PendingAttachment {
  /// Creates the attachment.
  const PendingAttachment({
    required this.key,
    required this.entryId,
    required this.name,
    required this.sizeBytes,
    required this.path,
    required this.mimeType,
    this.pastedText,
  });

  /// The attachment key: the message's slot `file:<key>` and the upload's `attachment_key`.
  final String key;

  /// The upload entry (a new one after "Upload again").
  final String entryId;

  /// The file name.
  final String name;

  /// The size.
  final int sizeBytes;

  /// The local file; the preview before sending shows it (15 14).
  final String path;

  /// The declared type.
  final String mimeType;

  /// The text the person pasted when a long paste became this file (M4-47), kept so "Insert
  /// as text" can put it back after the upload deleted the file; null for any other file.
  final String? pastedText;

  /// A copy with [entryId].
  PendingAttachment withEntry(String entryId) => PendingAttachment(
    key: key,
    entryId: entryId,
    name: name,
    sizeBytes: sizeBytes,
    path: path,
    mimeType: mimeType,
    pastedText: pastedText,
  );
}

/// The attachments of the composer of [thread] in [channel]; without a thread, of the first
/// message of a new post in the forum [channel] (M4-47).
///
/// Example:
/// ```dart
/// final attach = AttachController(thread: t, channel: c, outbox: engine.outbox,
///     progress: uploads, newKey: () => ids.next());
/// if (await attach.add(picked)) composer.focus();
/// await actions.send(t, text, attachments: attach.take());
/// ```
final class AttachController extends ChangeNotifier {
  /// Creates the controller; [newKey] makes the attachment keys, [limits] are the instance's
  /// (null while unknown: the server checks then).
  AttachController({
    required this.thread,
    required this.channel,
    required this._outbox,
    required this._progress,
    required this._newKey,
    required this._outgoing,
    this.limits,
  });

  /// The thread, or null for the first message of a new post.
  final ThreadId? thread;

  /// Its channel.
  final ChannelId channel;

  final Outbox _outbox;
  final UploadProgress _progress;
  final String Function() _newKey;

  /// The folder the attached files are copied into: the upload reads the copy, so it can
  /// resume after a restart without the original (a sandboxed macOS app loses access to a
  /// picked file when it quits) and a later change of the original never mixes two versions.
  final Future<Directory> Function() _outgoing;

  /// The limits of the instance (D-W-115), or null.
  InstanceLimits? limits;

  final List<PendingAttachment> _items = [];

  /// The refusal of the last [add], or null.
  AttachRefusal? refusal;

  /// Whether files are dragged over the composer.
  bool get dragging => _dragging;
  bool _dragging = false;

  /// Shows or hides the drop hint while files are dragged over the composer.
  void setDragging(bool value) {
    if (value == _dragging) return;
    _dragging = value;
    notifyListeners();
  }

  /// The attachments in their order.
  List<PendingAttachment> get attachments => List.unmodifiable(_items);

  /// How many more files this message takes under `message_max_attachments`, or null while the
  /// limits are unknown (the server checks then).
  int? get freeSlots {
    final maxCount = limits?.messageMaxAttachments;
    if (maxCount == null) return null;
    final free = maxCount - _items.length;
    return free < 0 ? 0 : free;
  }

  /// Whether one more file fits; when none does, [refusal] becomes [TooManyFiles] as an [add]
  /// over the limit would set it, so a picker need not open for files that would be refused.
  bool hasRoom() {
    final maxCount = limits?.messageMaxAttachments;
    if (maxCount == null || _items.length < maxCount) return true;
    return _refuse(TooManyFiles(maxCount));
  }

  /// Attaches [files]: refuses all of them with [refusal] when one is over a limit or cannot
  /// be read, before any entry is written (15 14, 04 13.2); otherwise writes one upload entry
  /// per file and returns true.
  Future<bool> add(List<PickedFile> files) async {
    refusal = null;
    if (files.isEmpty) return false;
    final maxCount = limits?.messageMaxAttachments;
    if (maxCount != null && _items.length + files.length > maxCount) {
      return _refuse(TooManyFiles(maxCount), files);
    }
    final maxBytes = limits?.fileMaxUploadBytes;
    for (final f in files) {
      int size;
      try {
        size = await File(f.path).length();
      } on FileSystemException {
        return _refuse(FileUnreadable(f.name), files);
      }
      if (size < 1) return _refuse(FileUnreadable(f.name), files);
      if (maxBytes != null && size > maxBytes) {
        return _refuse(FileTooLarge(f.name, maxBytes), files);
      }
    }
    for (final f in files) {
      final key = _newKey();
      final UploadSource source;
      try {
        final dir = Directory(
          '${(await _outgoing()).path}${Platform.pathSeparator}$key',
        );
        await dir.create(recursive: true);
        final copy = await File(f.path).copy(
          '${dir.path}${Platform.pathSeparator}${FileCache.safeFileName(f.name)}',
        );
        if (f.owned) await File(f.path).delete();
        source = await prepareUpload(copy.path, name: f.name);
      } on FileSystemException {
        return _refuse(FileUnreadable(f.name));
      }
      _items.add(await _enqueue(source, key: key, owned: true));
      notifyListeners();
    }
    return true;
  }

  /// Attaches [content] as a text file [name] the app writes itself (M4-47: a long paste, or
  /// the rest of a draft over `message_max_content_bytes`), declared as [mimeType];
  /// [pastedText] is kept on the attachment for "Insert as text". Refuses like [add] (count,
  /// size) before anything is written and returns false then.
  Future<bool> addText(
    String name,
    String content, {
    required String mimeType,
    String? pastedText,
  }) async {
    refusal = null;
    final maxCount = limits?.messageMaxAttachments;
    if (maxCount != null && _items.length + 1 > maxCount) {
      return _refuse(TooManyFiles(maxCount));
    }
    final bytes = utf8.encode(content);
    if (bytes.isEmpty) return _refuse(FileUnreadable(name));
    final maxBytes = limits?.fileMaxUploadBytes;
    if (maxBytes != null && bytes.length > maxBytes) {
      return _refuse(FileTooLarge(name, maxBytes));
    }
    final key = _newKey();
    final UploadSource source;
    try {
      final dir = Directory(
        '${(await _outgoing()).path}${Platform.pathSeparator}$key',
      );
      await dir.create(recursive: true);
      final file = File(
        '${dir.path}${Platform.pathSeparator}${FileCache.safeFileName(name)}',
      );
      await file.writeAsBytes(bytes, flush: true);
      source = await prepareUpload(file.path, name: name, mimeType: mimeType);
    } on FileSystemException {
      return _refuse(FileUnreadable(name));
    }
    final item = await _enqueue(source, key: key, owned: true);
    _items.add(
      pastedText == null
          ? item
          : PendingAttachment(
              key: item.key,
              entryId: item.entryId,
              name: item.name,
              sizeBytes: item.sizeBytes,
              path: item.path,
              mimeType: item.mimeType,
              pastedText: pastedText,
            ),
    );
    notifyListeners();
    return true;
  }

  Future<PendingAttachment> _enqueue(
    UploadSource source, {
    String? key,
    bool owned = false,
  }) async {
    final attachmentKey = key ?? _newKey();
    final entry = await _outbox.enqueue(
      UploadOps.request(
        threadId: thread,
        channelId: channel,
        source: source,
        key: attachmentKey,
        owned: owned,
      ),
    );
    return PendingAttachment(
      key: attachmentKey,
      entryId: entry.id,
      name: source.name,
      sizeBytes: source.sizeBytes,
      path: source.path,
      mimeType: source.mimeType,
    );
  }

  bool _refuse(AttachRefusal why, [List<PickedFile> files = const []]) {
    refusal = why;
    notifyListeners();
    // A refused file the app wrote itself (a pasted image, a photo) is not kept anywhere
    // (D-M5-07-5).
    for (final f in files) {
      if (!f.owned) continue;
      try {
        final file = File(f.path);
        if (file.existsSync()) file.deleteSync();
      } on FileSystemException {
        // Lies in a temporary folder, which the system empties.
      }
    }
    return false;
  }

  /// Removes the attachment [key], discards its upload entry and deletes its copy in the
  /// outgoing folder.
  Future<void> remove(String key) async {
    final i = _items.indexWhere((a) => a.key == key);
    if (i < 0) return;
    final item = _items.removeAt(i);
    refusal = null;
    notifyListeners();
    await _outbox.discard(item.entryId);
    final copy = Directory(
      '${(await _outgoing()).path}${Platform.pathSeparator}$key',
    );
    try {
      if (copy.existsSync()) await copy.delete(recursive: true);
    } on FileSystemException {
      // A copy that cannot be deleted now goes with the instance folder (D-M4-22-5).
    }
  }

  /// "Upload again": a failed upload of [key] becomes a new entry with a new session
  /// (brief M4-22 Design note 1).
  Future<void> retry(String key) async {
    final i = _items.indexWhere((a) => a.key == key);
    if (i < 0) return;
    final fresh = await _outbox.retry(_items[i].entryId);
    _items[i] = _items[i].withEntry(fresh);
    notifyListeners();
  }

  /// Hands the attachments to a message and clears the composer; null without attachments.
  MessageAttachments? take() {
    if (_items.isEmpty) return null;
    final taken = List.of(_items);
    _items.clear();
    refusal = null;
    notifyListeners();
    return _attachmentsOf(taken);
  }

  MessageAttachments _attachmentsOf(List<PendingAttachment> items) =>
      MessageAttachments(
        keys: [for (final a in items) a.key],
        uploads: [for (final a in items) a.entryId],
        fileIds: {
          for (final a in items) a.key: ?_progress.completedFileOf(a.key),
        },
        preview: [
          for (final a in items)
            {'key': a.key, 'name': a.name, 'size_bytes': a.sizeBytes},
        ],
      );

  /// The upload entry of the recording [file] (`audio/mp4`, 12 8.1) and the attachments of
  /// its voice message, without touching the composer's own attachments.
  ///
  /// The recording moves into the outgoing folder first, like an attached file: a recording
  /// lies in a temporary folder the system may empty, while the upload may wait for the
  /// network for a long time, and the outbox deletes the moved file after its upload
  /// (D-M5-07-5).
  Future<MessageAttachments> voice(RecordedFile file) async {
    final key = _newKey();
    final name = file.path.split(Platform.pathSeparator).last;
    final dir = Directory(
      '${(await _outgoing()).path}${Platform.pathSeparator}$key',
    );
    await dir.create(recursive: true);
    final target = '${dir.path}${Platform.pathSeparator}$name';
    try {
      return await withMovedFile(file.path, target, () async {
        final source = await prepareUpload(
          target,
          name: name,
          mimeType: 'audio/mp4',
        );
        return _attachmentsOf([await _enqueue(source, key: key, owned: true)]);
      });
    } on Object {
      // The recording went back for another try; the empty folder of the key goes.
      try {
        if (dir.listSync().isEmpty) await dir.delete();
      } on FileSystemException {
        // Nothing to clean up.
      }
      rethrow;
    }
  }
}

/// Moves the file [from] to [to] and runs [use]; when [use] throws, the file moves back to
/// [from] before the error goes on, so the caller keeps a valid path for another try (a
/// failed voice send keeps its recording, D-M5-07-5).
@visibleForTesting
Future<T> withMovedFile<T>(
  String from,
  String to,
  Future<T> Function() use,
) async {
  await _move(from, to);
  try {
    return await use();
  } on Object {
    await _move(to, from);
    rethrow;
  }
}

Future<void> _move(String from, String to) async {
  try {
    await File(from).rename(to);
  } on FileSystemException {
    // Another volume: copy, then delete the source.
    await File(from).copy(to);
    await File(from).delete();
  }
}

/// The drop area of the composer: files dropped on [child] go to [controller] (desktop_drop,
/// D-M4-22-4), which shows a refusal itself.
class AttachDropTarget extends StatelessWidget {
  /// Creates the drop area.
  const AttachDropTarget({
    super.key,
    required this.controller,
    required this.child,
  });

  /// The attachments.
  final AttachController controller;

  /// The composer.
  final Widget child;

  @override
  Widget build(BuildContext context) => DropTarget(
    onDragEntered: (_) => controller.setDragging(true),
    onDragExited: (_) => controller.setDragging(false),
    onDragDone: (details) {
      controller.setDragging(false);
      unawaited(
        controller.add([
          for (final f in details.files) PickedFile(path: f.path, name: f.name),
        ]),
      );
    },
    child: child,
  );
}

/// Paste (Cmd+V on macOS, Ctrl+V on Windows) of files or an image from the clipboard into
/// [controller] through [paste]; a paste without files leaves the text paste of the field.
class AttachPasteShortcut extends StatelessWidget {
  /// Creates the shortcut around [child].
  const AttachPasteShortcut({
    super.key,
    required this.controller,
    required this.paste,
    required this.scratch,
    required this.child,
  });

  /// The attachments.
  final AttachController controller;

  /// The clipboard.
  final PasteSource paste;

  /// Where a pasted image is written.
  final Future<Directory> Function() scratch;

  /// The composer.
  final Widget child;

  Future<void> _paste() async {
    final files = await paste.read(await scratch());
    if (files.isNotEmpty) await controller.add(files);
  }

  @override
  Widget build(BuildContext context) => Focus(
    canRequestFocus: false,
    skipTraversal: true,
    onKeyEvent: (node, event) {
      if (event is! KeyDownEvent ||
          event.logicalKey != LogicalKeyboardKey.keyV) {
        return KeyEventResult.ignored;
      }
      final keys = HardwareKeyboard.instance;
      final command = Platform.isMacOS
          ? keys.isMetaPressed
          : keys.isControlPressed;
      // The text of the clipboard still pastes into the field; files are added beside it.
      if (command) unawaited(_paste());
      return KeyEventResult.ignored;
    },
    child: child,
  );
}
