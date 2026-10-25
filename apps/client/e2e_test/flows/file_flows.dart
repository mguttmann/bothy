// SPDX-License-Identifier: Apache-2.0

// Upload and download of a file against the real server (brief M4-34 product requirement 13;
// 12 3, 15 7.5): Alex attaches a file and sends it, Sam sees the attachment through the API, and
// "Save as" writes the downloaded bytes, which equal the original. The overlay turns the virus
// scan off (`file.scan.mode = off`), so the file is ready at once; the scan has its own tests.

import 'dart:io';

import 'package:client/files/file_ports.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../../test/support/file_fakes.dart';
import '../support/e2e_app.dart';
import '../support/e2e_context.dart';
import '../support/e2e_keys.dart';
import 'messaging_flows.dart' show composerText;

final AppLocalizations _en = lookupAppLocalizations(const Locale('en'));

/// Registers the file flow.
void fileFlows(E2eContext c) {
  final world = c.world;

  testWidgets('upload a file and download it again', (tester) async {
    final nonce = randomNonce().substring(0, 8);
    final picker = FakePicker();
    final dir = await Directory.systemTemp.createTemp('e2e-files');
    final original = File('${dir.path}/report-$nonce.txt')
      ..writeAsStringSync('The quarterly report $nonce\n' * 64);
    picker
      ..next = [PickedFile(path: original.path, name: 'report-$nonce.txt')]
      ..saveTo = '${dir.path}/saved.txt';
    final app = await E2eApp.start(
      tester,
      world,
      files: fakeFilePorts(picker: picker),
    );
    try {
      await app.signIn(await c.alexCode());
      await app.openChannel(world.generalId);
      await app.waitFor(find.byKey(const ValueKey('composer')));
      await app.tapFinder(
        find.byWidgetPredicate(
          (w) => w is DsTappable && w.label == _en.composerAttach,
        ),
      );
      await app.waitFor(
        find.descendant(
          of: find.byKey(const ValueKey('composer.attachments')),
          matching: find.textContaining('report-$nonce.txt'),
        ),
      );
      await tester.enterText(composerText, 'The report $nonce');
      await tester.pump();
      await tester.sendKeyEvent(
        LogicalKeyboardKey.enter,
        // A profile build has no debug names to look the physical key up (the performance level).
        physicalKey: PhysicalKeyboardKey.enter,
      );
      await tester.pump();

      String? fileId;
      await app.untilAsync('Sam sees the attachment', () async {
        final messages = await c.sam.messages(world.generalThreadId);
        final m = messages
            .where((m) => m.contentMd == 'The report $nonce')
            .firstOrNull;
        if (m == null || m.attachments.isEmpty) return false;
        fileId = m.attachments.first.file.id;
        return true;
      });

      await app.tap('file.$fileId.save');
      final saved = File(picker.saveTo!);
      await app.until(
        'the saved file',
        () => saved.existsSync() && saved.lengthSync() == original.lengthSync(),
      );
      expect(saved.readAsBytesSync(), original.readAsBytesSync());
      expect(picker.asked, contains('report-$nonce.txt'));
    } finally {
      await app.stop();
      await dir.delete(recursive: true);
    }
  });
}
