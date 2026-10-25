// SPDX-License-Identifier: Apache-2.0

// Photos from the camera and the library (brief M5-07 Tests and Design note 4; 15 14; 12 7.1;
// ADR 0047): what the pickers give becomes a JPEG file in the temporary folder, a file the app
// cannot turn into JPEG is refused, the picker's own file never stays, a denied camera or
// library says so, and in the phone composer a photo is attached as `image/jpeg` through the
// outbox while one over `file_max_upload_bytes` is refused with the reason before the outbox
// and deleted.

import 'dart:io';
import 'dart:typed_data';

import 'package:client/files/upload_executor.dart';
import 'package:client/phone/compose/media_picker.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image/image.dart' as img;
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../support/column_host.dart';
import '../../support/file_fakes.dart';
import '../../support/file_widgets.dart';
import '../../support/structure.dart';
import '../../support/thread_fixtures.dart';
import '../../support/thread_host.dart';
import '../../support/widgets.dart';

/// A small photo as JPEG.
Uint8List jpegPhoto() => img.encodeJpg(
  img.Image(width: 8, height: 6)..clear(img.ColorRgb8(40, 120, 200)),
);

/// A small image as PNG, with transparency when [alpha].
Uint8List pngImage({bool alpha = false}) => img.encodePng(
  img.Image(width: 8, height: 6, numChannels: alpha ? 4 : 3)..clear(
    alpha ? img.ColorRgba8(200, 40, 40, 100) : img.ColorRgb8(200, 40, 40),
  ),
);

/// The start of a HEIC file (`ftyp` with brand `heic`), which package `image` cannot decode.
final Uint8List heicStart = Uint8List.fromList([
  0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0x68, 0x65, 0x69, 0x63, //
  0x00, 0x00, 0x00, 0x00, 0x6d, 0x69, 0x66, 0x31, 0x68, 0x65, 0x69, 0x63,
]);

void main() {
  group('MediaPicker', () {
    late Directory root;
    late Directory picked;
    late Directory scratch;
    late FakeMediaPort port;
    late MediaPicker picker;

    setUp(() {
      root = Directory.systemTemp.createTempSync('media');
      picked = Directory('${root.path}/picker')..createSync();
      scratch = Directory('${root.path}/scratch');
      port = FakeMediaPort();
      picker = MediaPicker(port: port, scratch: () async => scratch);
    });
    tearDown(() => root.deleteSync(recursive: true));

    RawMedia raw(String name, List<int> bytes) {
      final f = File('${picked.path}/$name')..writeAsBytesSync(bytes);
      return RawMedia(path: f.path, name: name);
    }

    test('a JPEG of the library keeps its bytes and gets the name .jpg', () async {
      final bytes = jpegPhoto();
      // The platform picker re-encoded a HEIC photo; its name still says HEIC.
      port.next = raw('IMG_0042.HEIC', bytes);
      final outcome = await picker.pick(MediaSource.library);
      expect(port.asked, [MediaSource.library]);
      final file = (outcome as PhotoPicked).file;
      expect(file.name, 'IMG_0042.jpg');
      expect(file.owned, isTrue);
      expect(File(file.path).readAsBytesSync(), bytes);
      expect(file.path, startsWith(scratch.path));
      expect(picked.listSync(), isEmpty);
    });

    test('a PNG from the camera becomes a JPEG', () async {
      port.next = raw('capture.png', pngImage());
      final outcome = await picker.pick(MediaSource.camera);
      final file = (outcome as PhotoPicked).file;
      expect(file.name, 'capture.jpg');
      final bytes = File(file.path).readAsBytesSync();
      expect(isJpeg(bytes), isTrue);
      final decoded = img.decodeJpg(bytes)!;
      expect((decoded.width, decoded.height), (8, 6));
      expect(picked.listSync(), isEmpty);
    });

    test('a transparent image becomes a JPEG on white', () async {
      port.next = raw('sticker.png', pngImage(alpha: true));
      final file =
          ((await picker.pick(MediaSource.library)) as PhotoPicked).file;
      final decoded = img.decodeJpg(File(file.path).readAsBytesSync())!;
      expect(decoded.numChannels, 3);
      // Red at 100 of 255 alpha over white is a light red, not the dark red of black.
      final p = decoded.getPixel(4, 3);
      expect(p.r, greaterThan(200));
      expect(p.g, greaterThan(120));
    });

    test('a file that cannot become a JPEG is refused and deleted', () async {
      port.next = raw('IMG_0043.HEIC', heicStart);
      final outcome = await picker.pick(MediaSource.library);
      expect(outcome, isA<PhotoUnsupported>());
      expect((outcome as PhotoUnsupported).name, 'IMG_0043.HEIC');
      expect(picked.listSync(), isEmpty);
      expect(
        scratch.existsSync() ? scratch.listSync() : const <FileSystemEntity>[],
        isEmpty,
      );
    });

    test('a denied camera, a cancel and a failing picker say so', () async {
      port.error = const MediaAccessDenied(MediaSource.camera);
      final denied = await picker.pick(MediaSource.camera);
      expect((denied as PhotoDenied).source, MediaSource.camera);
      port
        ..error = null
        ..next = null;
      expect(await picker.pick(MediaSource.library), isA<PhotoCancelled>());
      port.error = StateError('no camera');
      expect(await picker.pick(MediaSource.camera), isA<PhotoFailed>());
    });

    test('jpegName replaces the extension', () {
      expect(jpegName('IMG_0042.HEIC'), 'IMG_0042.jpg');
      expect(jpegName('photo'), 'photo.jpg');
      expect(jpegName('.hidden'), '.hidden.jpg');
      expect(jpegName(''), 'photo.jpg');
    });
  });

  group('in the phone composer', () {
    Future<List<OutboxEntry>> uploads(WidgetTester tester, ColumnHost host) =>
        host.run(tester, () async {
          final all = await host.db.outboxDao.allEntries();
          return [
            for (final e in all)
              if (e.kind == UploadOps.upload) e,
          ];
        });

    Future<(ColumnHost, FakeMediaPort, Directory)> open(
      WidgetTester tester, {
      int maxUpload = 1000000,
    }) async {
      final host = await ColumnHost.open(tester);
      final media = FakeMediaPort();
      final temporary = Directory('${host.root.path}/tmp')..createSync();
      final services = FileTestServices(
        host.db,
        host.root,
        fakeFilePorts(media: media, temporary: temporary),
      );
      await host.run(tester, () async {
        await seedThread(host.db, messages: [msg(1)]);
        final base = instanceWith();
        await host.db.instanceDao.put(
          base.copyWith(
            limits: base.limits.copyWith(fileMaxUploadBytes: maxUpload),
          ),
        );
      });
      await host.pump(
        tester,
        HistoryColumn(threadId: threadId(1)),
        location: '/t/${threadId(1).value}',
        platform: 'ios',
        overrides: [
          ...threadOverrides(FakeThreadReads(host.db)),
          ...services.overrides,
        ],
      );
      return (host, media, temporary);
    }

    Future<void> choose(
      WidgetTester tester,
      ColumnHost host,
      String item,
    ) async {
      await tester.tap(
        find.byWidgetPredicate(
          (w) => w is DsTappable && w.label == en.composerAttach,
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('composer.attachMenu')), findsOneWidget);
      await tester.tap(find.byKey(ValueKey('composer.attach.$item')));
      await tester.pumpAndSettle();
    }

    RawMedia write(ColumnHost host, String name, List<int> bytes) {
      final f = File('${host.root.path}/$name')..writeAsBytesSync(bytes);
      return RawMedia(path: f.path, name: name);
    }

    testWidgets('a photo of the library is attached as image/jpeg', (
      tester,
    ) async {
      final (host, media, temporary) = await open(tester);
      media.next = await host.run(
        tester,
        () async => write(host, 'IMG_0042.HEIC', jpegPhoto()),
      );
      await choose(tester, host, 'library');
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('upload.attach-0')),
      );
      expect(media.asked, [MediaSource.library]);
      expect(find.textContaining('IMG_0042.jpg'), findsOneWidget);
      // The preview before sending shows the local file (15 14).
      expect(
        find.byKey(const ValueKey('upload.attach-0.preview')),
        findsOneWidget,
      );
      final entry = (await uploads(tester, host)).single;
      final payload = decodePayload(entry.payload);
      expect(payload['declared_mime_type'], 'image/jpeg');
      expect(payload['name'], 'IMG_0042.jpg');
      // The outbox took a copy; nothing stays in the temporary folder.
      expect(Directory('${temporary.path}/photos').listSync(), isEmpty);
      await host.dispose(tester);
    });

    testWidgets(
      'a photo over file_max_upload_bytes is refused before the outbox',
      (tester) async {
        final (host, media, temporary) = await open(tester, maxUpload: 16);
        media.next = await host.run(
          tester,
          () async => write(host, 'big.jpg', jpegPhoto()),
        );
        await choose(tester, host, 'camera');
        await settleUntil(
          tester,
          host,
          find.byKey(const ValueKey('composer.attachError')),
        );
        expect(
          find.text(en.composerFileTooLarge(dsIsolate('big.jpg'), '16 B')),
          findsOneWidget,
        );
        expect(await uploads(tester, host), isEmpty);
        expect(Directory('${temporary.path}/photos').listSync(), isEmpty);
        await host.dispose(tester);
      },
    );

    testWidgets('a denied camera shows the way to the settings', (
      tester,
    ) async {
      final (host, media, _) = await open(tester);
      media.error = const MediaAccessDenied(MediaSource.camera);
      await choose(tester, host, 'camera');
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('composer.photoNotice')),
      );
      expect(find.text(en.photoCameraDeniedIos), findsOneWidget);
      expect(await uploads(tester, host), isEmpty);
      await host.dispose(tester);
    });
  });
}
