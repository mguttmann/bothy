// SPDX-License-Identifier: Apache-2.0

// Photos from the camera and the library (brief M5-07 Tests and Design note 4; 15 14; 12 7.1;
// ADR 0047): what the pickers give becomes a JPEG file in the temporary folder, a file the app
// cannot turn into JPEG is refused, the picker's own file never stays, a denied camera or
// library says so, and in the phone composer a photo is attached as `image/jpeg` through the
// outbox while one over `file_max_upload_bytes` is refused with the reason before the outbox
// and deleted. The library picks several photos at once up to the free attachment slots (M5-16):
// all of them become JPEG attachments of one message in the picker's order, or none of them.

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
      final file = (outcome as PhotoPicked).files.single;
      expect(file.name, 'IMG_0042.jpg');
      expect(file.owned, isTrue);
      expect(File(file.path).readAsBytesSync(), bytes);
      expect(file.path, startsWith(scratch.path));
      expect(picked.listSync(), isEmpty);
    });

    test('a PNG from the camera becomes a JPEG', () async {
      port.next = raw('capture.png', pngImage());
      final outcome = await picker.pick(MediaSource.camera);
      final file = (outcome as PhotoPicked).files.single;
      expect(file.name, 'capture.jpg');
      final bytes = File(file.path).readAsBytesSync();
      expect(isJpeg(bytes), isTrue);
      final decoded = img.decodeJpg(bytes)!;
      expect((decoded.width, decoded.height), (8, 6));
      expect(picked.listSync(), isEmpty);
    });

    test('a transparent image becomes a JPEG on white', () async {
      port.next = raw('sticker.png', pngImage(alpha: true));
      final file = ((await picker.pick(
        MediaSource.library,
      )) as PhotoPicked).files.single;
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

    test('three photos of the library become three JPEGs in order', () async {
      port.many = [
        raw('IMG_0001.HEIC', jpegPhoto()),
        raw('screen.png', pngImage()),
        raw('IMG_0003.jpg', jpegPhoto()),
      ];
      final outcome = await picker.pick(MediaSource.library, limit: 7);
      expect(port.asked, [MediaSource.library]);
      expect(port.limits, [7]);
      final files = (outcome as PhotoPicked).files;
      expect(
        [for (final f in files) f.name],
        ['IMG_0001.jpg', 'screen.jpg', 'IMG_0003.jpg'],
      );
      for (final f in files) {
        expect(f.owned, isTrue);
        expect(f.path, startsWith(scratch.path));
        expect(isJpeg(File(f.path).readAsBytesSync()), isTrue);
      }
      expect({for (final f in files) f.path}, hasLength(3));
      expect(picked.listSync(), isEmpty);
    });

    test(
      'the library limit is passed to the port; null when unknown',
      () async {
        port.many = [raw('a.jpg', jpegPhoto())];
        await picker.pick(MediaSource.library, limit: 1);
        port.many = [raw('b.jpg', jpegPhoto())];
        await picker.pick(MediaSource.library);
        expect(port.limits, [1, null]);
      },
    );

    test('one photo that cannot become a JPEG refuses the whole set', () async {
      port.many = [
        raw('IMG_0001.jpg', jpegPhoto()),
        raw('IMG_0002.HEIC', heicStart),
        raw('IMG_0003.png', pngImage()),
      ];
      final outcome = await picker.pick(MediaSource.library, limit: 10);
      expect((outcome as PhotoUnsupported).name, 'IMG_0002.HEIC');
      // Neither the picker's files nor the JPEG already written stay.
      expect(picked.listSync(), isEmpty);
      expect(
        scratch.existsSync() ? scratch.listSync() : const <FileSystemEntity>[],
        isEmpty,
      );
    });

    test('an unreadable photo of the set fails it and deletes all', () async {
      final ok = raw('IMG_0001.jpg', jpegPhoto());
      port.many = [
        ok,
        RawMedia(path: '${picked.path}/gone.jpg', name: 'gone.jpg'),
        raw('IMG_0003.jpg', jpegPhoto()),
      ];
      final outcome = await picker.pick(MediaSource.library, limit: 10);
      expect(outcome, isA<PhotoFailed>());
      expect(picked.listSync(), isEmpty);
      expect(
        scratch.existsSync() ? scratch.listSync() : const <FileSystemEntity>[],
        isEmpty,
      );
    });

    test('an empty library pick is a cancel', () async {
      port.many = const [];
      expect(
        await picker.pick(MediaSource.library, limit: 4),
        isA<PhotoCancelled>(),
      );
    });

    test('the camera still takes one photo and ignores the limit', () async {
      port.next = raw('capture.jpg', jpegPhoto());
      final outcome = await picker.pick(MediaSource.camera, limit: 5);
      expect(port.asked, [MediaSource.camera]);
      expect(port.limits, isEmpty);
      expect((outcome as PhotoPicked).files, hasLength(1));
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

    Future<List<OutboxEntry>> messages(WidgetTester tester, ColumnHost host) =>
        host.run(tester, () async {
          final all = await host.db.outboxDao.allEntries();
          return [
            for (final e in all)
              if (e.kind == MessageOps.createMessage) e,
          ];
        });

    Future<List<RawMedia>> photos(
      WidgetTester tester,
      ColumnHost host,
      int count, {
      String prefix = 'IMG_',
    }) => host.run(tester, () async {
      return [
        for (var i = 0; i < count; i++)
          write(host, '$prefix${1000 + i}.HEIC', jpegPhoto()),
      ];
    });

    testWidgets('four photos of the library go into one message', (
      tester,
    ) async {
      final (host, media, temporary) = await open(tester);
      media.many = await photos(tester, host, 4);
      await choose(tester, host, 'library');
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('upload.attach-3')),
      );
      // The instance allows 10 attachments, none is attached yet.
      expect(media.limits, [10]);
      for (var i = 0; i < 4; i++) {
        expect(find.byKey(ValueKey('upload.attach-$i')), findsOneWidget);
      }
      final ups = await uploads(tester, host);
      expect(
        [for (final u in ups) decodePayload(u.payload)['name']],
        ['IMG_1000.jpg', 'IMG_1001.jpg', 'IMG_1002.jpg', 'IMG_1003.jpg'],
      );
      expect(Directory('${temporary.path}/photos').listSync(), isEmpty);

      await tester.tap(
        find.byWidgetPredicate(
          (w) => w is DsTappable && w.label == en.composerSend,
        ),
      );
      await host.settle(tester);
      final sent = (await messages(tester, host)).single;
      expect(decodePayload(sent.payload)['attachment_keys'], [
        'attach-0',
        'attach-1',
        'attach-2',
        'attach-3',
      ]);
      await host.dispose(tester);
    });

    testWidgets('the library offers only the free attachment slots', (
      tester,
    ) async {
      final (host, media, _) = await open(tester);
      media.many = await photos(tester, host, 9);
      await choose(tester, host, 'library');
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('upload.attach-8')),
      );
      media.many = await photos(tester, host, 1, prefix: 'LAST_');
      await choose(tester, host, 'library');
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('upload.attach-9')),
      );
      expect(media.limits, [10, 1]);

      // With all 10 slots taken the picker does not open; the refusal says why.
      await choose(tester, host, 'library');
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('composer.attachError')),
      );
      expect(find.text(en.composerTooManyFiles(10)), findsOneWidget);
      expect(media.asked, [MediaSource.library, MediaSource.library]);
      expect(await uploads(tester, host), hasLength(10));
      await host.dispose(tester);
    });

    testWidgets(
      'a set over the limit from a picker that ignores it is refused',
      (tester) async {
        final (host, media, temporary) = await open(tester);
        media.many = await photos(tester, host, 11);
        await choose(tester, host, 'library');
        await settleUntil(
          tester,
          host,
          find.byKey(const ValueKey('composer.attachError')),
        );
        expect(find.text(en.composerTooManyFiles(10)), findsOneWidget);
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
