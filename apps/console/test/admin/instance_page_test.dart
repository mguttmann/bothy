// SPDX-License-Identifier: Apache-2.0

// The instance page of the console (brief M6-09 Scope, Design note 2): it hosts the instance
// administration of admin_ui, offers the browser's file input for the logo, and reads discovery
// again after a branding change, so the console's theme follows the new accent at once.

import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:console/console.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../login_test.dart' show pumpConsole;
import '../support/fakes.dart';
import 'admin_pages_test.dart' show browserWindow, signedIn;

Map<String, Object?> _branding(String accent, int version) => {
  'product_name': 'Team Chat',
  'accent_color': accent,
  'version': version,
};

void main() {
  testWidgets('a new accent reaches the theme of the console', (tester) async {
    browserWindow(tester);
    final b = signedIn();
    var accent = 'loch';
    b.server.routes['GET /api/v1/instance/settings'] = (_) =>
        const Answer(200, {'items': <Object>[]});
    b.server.routes['GET /api/v1/instance'] = (_) => Answer(200, {
      'instance_id': '0192f4a0-7b1c-7db7-89b9-2fe4c48dcb3e',
      'branding': _branding(accent, 1),
      'locale_default': 'de',
      'spec_version': '0.2.0',
      'features': <Object>[],
      'limits': {
        'message_max_content_bytes': 40000,
        'message_max_attachments': 10,
        'file_max_upload_bytes': 104857600,
        'thread_title_max_chars': 120,
        'voice_max_duration_ms': 300000,
      },
    });
    b.server.routes['PUT /api/v1/instance/branding'] = (r) {
      accent = (r.body! as Map)['accent_color']! as String;
      return Answer(200, _branding(accent, 2));
    };
    b.server.routes['GET /api/v1/instance/discovery'] = (_) {
      final d = discovery();
      return Answer(200, {
        ...d,
        'branding': {'product_name': 'Team Chat', 'accent_color': accent},
      });
    };

    final c = await pumpConsole(tester, b, location: '/instance');
    expect(c.page, ConsolePage.instance);
    expect(find.byType(InstanceView), findsOneWidget);
    DsAccent themeAccent() =>
        Theme.of(tester.element(find.byType(InstanceView)))
            .extension<DsAccent>()!;
    expect(themeAccent(), DsAccent.of('loch', Brightness.light));

    await tester.tap(find.byKey(const ValueKey('instance.tab.branding')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('accent.moss')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('branding.save')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('confirm.submit')));
    await tester.pumpAndSettle();

    final put = b.server.seen.lastWhere(
      (s) => s.path == '/api/v1/instance/branding',
    );
    expect(put.headers['if-match'], '"1"');
    expect(c.discovery!.branding.accentColor, 'moss');
    expect(themeAccent(), DsAccent.of('moss', Brightness.light));
  });

  test('the logo comes from the browser file input with image types', () async {
    final files = FakeFileChooser()
      ..next = ChosenFile('logo.png', Uint8List.fromList([1, 2]));
    final picked = await logoPickerOf(files)();
    expect(picked!.name, 'logo.png');
    expect(files.choices.single, (logoAccept, 'logo'));
    expect(logoAccept, contains('image/webp'));
  });
}
