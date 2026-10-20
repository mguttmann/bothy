// SPDX-License-Identifier: Apache-2.0

// Fakes of the file and voice ports (brief M4-22 Tests: fake recorder, fake players, no
// dialogs) and a harness of the upload path: the fake server of M4-17 on 127.0.0.1, Alex's
// REST client, an in-memory database and an outbox with the app's executors.

import 'dart:async';
import 'dart:io';

import 'package:api_client/api_client.dart' hide File;
import 'package:client/files/file_ports.dart';
import 'package:client/files/ticket_player.dart';
import 'package:client/files/upload_executor.dart';
import 'package:client/voice/recorder.dart';
import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import 'fakes.dart';
import 'other_device.dart';

/// A picker that returns what the test sets.
class FakePicker implements FilePicker {
  /// The files the next [pickFiles] returns.
  List<PickedFile> next = const [];

  /// The path the next [saveLocation] returns, or null for "cancelled".
  String? saveTo;

  /// The names asked for in [saveLocation].
  final List<String> asked = [];

  @override
  Future<List<PickedFile>> pickFiles() async => next;

  @override
  Future<String?> saveLocation(String suggestedName) async {
    asked.add(suggestedName);
    return saveTo;
  }
}

/// A clipboard with the files the test sets.
class FakePaste implements PasteSource {
  /// The files on the clipboard.
  List<PickedFile> files = const [];

  @override
  Future<List<PickedFile>> read(Directory scratch) async => files;
}

/// An opener that records the paths.
class FakeOpener implements FileOpener {
  /// The opened paths.
  final List<String> opened = [];

  @override
  Future<bool> open(String path) async {
    opened.add(path);
    return true;
  }
}

/// A recorder that writes [bytes] as the recording.
class FakeRecorder implements VoiceRecorder {
  /// Creates the recorder; [permitted] answers [hasPermission].
  FakeRecorder({this.permitted = true, List<int>? bytes})
    : bytes = bytes ?? _m4a;

  /// Whether the microphone may be used.
  bool permitted;

  /// The bytes of the recording.
  final List<int> bytes;

  /// The calls, for assertions.
  final List<String> calls = [];

  /// The bit rate of the last start.
  int? bitRate;

  String? _path;
  DateTime? _started;
  final StreamController<double> _levels = StreamController.broadcast();

  @override
  Future<bool> hasPermission() async {
    calls.add('permission');
    return permitted;
  }

  @override
  Future<void> start({required String path, required int bitRate}) async {
    calls.add('start');
    this.bitRate = bitRate;
    _path = path;
    _started = DateTime.now();
  }

  @override
  Future<RecordedFile?> stop() async {
    calls.add('stop');
    final path = _path;
    if (path == null) return null;
    await File(path).writeAsBytes(bytes, flush: true);
    return RecordedFile(
      path: path,
      duration: DateTime.now().difference(_started!),
      sizeBytes: bytes.length,
    );
  }

  @override
  Future<void> cancel() async {
    calls.add('cancel');
    _path = null;
  }

  @override
  Stream<double> get amplitude => _levels.stream;

  @override
  Future<void> dispose() async {
    calls.add('dispose');
    await _levels.close();
  }

  /// The start of an MP4 file (`ftyp` box of an `.m4a`), so the fake server detects
  /// `audio/mp4`.
  static const List<int> _m4a = [
    0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0x4d, 0x34, 0x41, 0x20, //
    0x00, 0x00, 0x00, 0x00, 0x4d, 0x34, 0x41, 0x20, 0x69, 0x73, 0x6f, 0x6d,
  ];
}

/// A player that records what it was asked to do.
class FakePlayer implements TicketPlayer {
  /// Every player created, for assertions.
  static final List<FakePlayer> created = [];

  /// Creates the player and records it in [created].
  FakePlayer() {
    created.add(this);
  }

  /// The URLs played.
  final List<Uri> played = [];

  /// The speeds set.
  final List<double> speeds = [];

  /// Whether [dispose] ran.
  bool disposed = false;

  final StreamController<PlayerPhase> _phases = StreamController.broadcast();
  final StreamController<Duration> _position = StreamController.broadcast();

  @override
  Future<void> play(Uri url) async {
    played.add(url);
    _phases.add(PlayerPhase.playing);
  }

  @override
  Future<void> pause() async => _phases.add(PlayerPhase.paused);

  @override
  Future<void> resume() async => _phases.add(PlayerPhase.playing);

  @override
  Future<void> setSpeed(double speed) async => speeds.add(speed);

  @override
  Stream<Duration> get position => _position.stream;

  @override
  Stream<PlayerPhase> get phases => _phases.stream;

  /// Ends the playback.
  void complete() => _phases.add(PlayerPhase.completed);

  @override
  Future<void> dispose() async {
    disposed = true;
    await _phases.close();
    await _position.close();
  }
}

/// The ports of files and voice with the fakes above.
FilePorts fakeFilePorts({
  FakePicker? picker,
  FakePaste? paste,
  FakeOpener? opener,
  FakeRecorder? recorder,
}) => FilePorts(
  picker: picker ?? FakePicker(),
  paste: paste ?? FakePaste(),
  opener: opener ?? FakeOpener(),
  recorder: () => recorder ?? FakeRecorder(),
  player: FakePlayer.new,
  video: (url) => const SizedBox(key: ValueKey('file.video.surface')),
);

/// The fake server with Alex's REST client, an in-memory database and an outbox with the
/// message, upload and attaching executors of the app (brief M4-22 Design notes 1 and 2).
class UploadHarness {
  UploadHarness._(
    this.server,
    this.seed,
    this.alex,
    this.db,
    this.outbox,
    this.progress,
    this.folder,
  );

  /// Starts the fake (parts of [partBytes]) and the outbox (started).
  static Future<UploadHarness> start({int partBytes = 4}) async {
    final server = await FakeServer.start();
    final seed = WorldSeed.standard(server.world);
    server.world.fileSettings.partBytes = partBytes;
    final alex = OtherDevice(server);
    await alex.signIn();
    final db = openInMemoryForTests();
    final progress = UploadProgress();
    final executors = OutboxExecutors();
    MessageOps.register(executors, alex.auth.api);
    UploadOps.register(
      executors,
      alex.auth.api,
      progress,
      baseUrl: server.baseUri,
    );
    AttachmentMessageOps.wrap(executors, progress);
    final folder = await Directory.systemTemp.createTemp('uploads');
    return UploadHarness._(
      server,
      seed,
      alex,
      db,
      _outboxOn(db, executors),
      progress,
      folder,
    ).._executors = executors;
  }

  static int _n = 0;

  static Outbox _outboxOn(LocalDatabase db, OutboxExecutors executors) =>
      Outbox(
        db: db,
        executors: executors,
        config: AppConfig.defaults().withOverrides(fastConfig),
        ids: Uuid7Generator(
          nowMillis: () => DateTime.now().millisecondsSinceEpoch,
          random: (bits) => (_n++ * 7919) % (1 << (bits > 30 ? 30 : bits)),
        ),
      );

  late OutboxExecutors _executors;

  /// A new outbox on the same database and executors, as the next start of the app builds
  /// it; the caller closes it.
  Outbox reopen() => _outboxOn(db, _executors);

  /// The fake.
  final FakeServer server;

  /// Alex, Sam and their channels.
  final WorldSeed seed;

  /// Alex's device.
  final OtherDevice alex;

  /// The local database.
  final LocalDatabase db;

  /// The outbox.
  final Outbox outbox;

  /// The upload progress.
  final UploadProgress progress;

  /// A temporary folder for the files to send.
  final Directory folder;

  /// Alex's REST client.
  ApiClient get api => alex.auth.api;

  /// Writes [bytes] as [name] into [folder].
  Future<File> file(String name, List<int> bytes) async {
    final f = File('${folder.path}/$name');
    await f.writeAsBytes(bytes, flush: true);
    return f;
  }

  /// Stops the outbox, the database and the fake and deletes [folder].
  Future<void> stop() async {
    await outbox.close();
    await db.close();
    await alex.dispose();
    await server.stop();
    if (folder.existsSync()) await folder.delete(recursive: true);
  }
}
