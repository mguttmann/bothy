// SPDX-License-Identifier: Apache-2.0

// Copying single blocks on the real system clipboard of macOS and Windows (01 6.12, R-PR-25;
// brief M4-48): the copy button of a code block puts its raw content byte for byte on the
// clipboard (tabs, unicode, a long line, no fence, no language tag); the button of a quote puts
// its text without markers there and its HTML form in the clipboard's HTML flavor (the
// pasteboard type public.html on macOS, the format "HTML Format" on Windows).

import 'package:client/app/rich_clipboard.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';

final String _longLine = List.filled(400, 'wide\tline').join(' ');
final String _code =
    '\tif (x) {\n'
    '\t\treturn "ñ \u{1F600} אב é";  \n'
    '\t}\n'
    '$_longLine\n';

String get _source =>
    '```dart\n$_code\n```\n\n'
    '> Quoted **bold** line\n'
    '> second ñ line\n';

Finder _button(Type block) => find
    .descendant(of: find.byType(block), matching: find.byType(DsTappable))
    .last;

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('code and quote copies reach the system clipboard', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: DsThemeScope(
          dsContext: const DsContext(
            theme: DsThemeMode.light,
            density: DsDensityMode.comfortable,
            motion: DsMotionMode.reduced,
            accent: dsDefaultAccent,
          ),
          child: Scaffold(
            body: SingleChildScrollView(
              child: DsClipboardScope(
                clipboard: const PlatformRichClipboard(),
                child: DsMarkdown(_source, onOpenLink: (_) {}),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(_button(DsMarkdownCodeBlock));
    await tester.pumpAndSettle();
    final code = await Clipboard.getData(Clipboard.kTextPlain);
    // The fence's last line break is not part of the copy; the blank line before it is.
    expect(code?.text, _code);

    await tester.tap(_button(DsMarkdownQuoteBlock));
    await tester.pumpAndSettle();
    final quote = await Clipboard.getData(Clipboard.kTextPlain);
    expect(quote?.text, 'Quoted bold line\nsecond ñ line');
    final html = await PlatformRichClipboard.channel.invokeMethod<String>(
      'readHtml',
    );
    expect(html, contains('<strong>bold</strong>'));
    expect(html, contains('second ñ line'));
  });
}
