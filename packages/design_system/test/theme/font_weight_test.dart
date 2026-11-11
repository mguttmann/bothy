// SPDX-License-Identifier: Apache-2.0

// A weight set on a style of `dsTextStyle` reaches the glyphs (AUD-pkg-design-system-01,
// finding design-system-1). The engine drives the `wght` axis of the variable font files from
// `FontWeight` unless a style pins the axis itself; a pinned axis kept unread titles and
// Markdown bold at the weight of their role, although `TextStyle.fontWeight` said bold.
//
// So these tests do not read `fontWeight`: they measure the glyphs the engine lays out with the
// bundled fonts (test/flutter_test_config.dart loads them) and compare them with the same text
// at the weight with the axis set by hand, and with the regular weight.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

import '../components/support.dart';

/// [style] at [weight] with the `wght` axis set by hand: the reference of a true weight, on
/// every host and whatever `dsTextStyle` does.
TextStyle atWeight(TextStyle style, FontWeight weight) => style.copyWith(
  fontWeight: weight,
  fontVariations: [FontVariation.weight(weight.value.toDouble())],
);

/// The width the engine gives [text] in [style].
double widthOf(String text, TextStyle style) {
  final painter = TextPainter(
    text: TextSpan(text: text, style: style),
    textDirection: TextDirection.ltr,
  )..layout();
  final width = painter.width;
  painter.dispose();
  return width;
}

/// The style in effect on the span below [root] whose own text holds [needle] (a row wraps a
/// foreign name in the isolates U+2068 and U+2069, so the text is not always equal).
TextStyle styleOf(InlineSpan root, String needle) {
  TextStyle? found;
  void walk(InlineSpan span, TextStyle? inherited) {
    if (span is! TextSpan) return;
    final style = inherited?.merge(span.style) ?? span.style;
    if (span.text?.contains(needle) ?? false) found = style;
    for (final child in span.children ?? const <InlineSpan>[]) {
      walk(child, style);
    }
  }

  walk(root, null);
  if (found == null) throw StateError('no span with the text "$needle"');
  return found!;
}

/// Expects the glyphs of [needle] in the paragraph of [paragraph] to be drawn at [weight]: as
/// wide as with the axis set by hand, and wider than at the regular weight.
void expectDrawnAt(
  WidgetTester tester,
  Finder paragraph,
  String needle,
  FontWeight weight,
) {
  final render = tester.renderObject<RenderParagraph>(paragraph);
  final start = render.text.toPlainText().indexOf(needle);
  expect(start, isNonNegative, reason: 'the paragraph shows "$needle"');
  final boxes = render.getBoxesForSelection(
    TextSelection(baseOffset: start, extentOffset: start + needle.length),
  );
  final drawn = boxes.fold<double>(0, (sum, box) => sum + box.toRect().width);
  final style = styleOf(render.text, needle);
  final regular = widthOf(needle, atWeight(style, FontWeight.w400));
  final wanted = widthOf(needle, atWeight(style, weight));
  // The font must tell the two weights apart, or the test could not fail.
  expect(wanted - regular, greaterThan(1), reason: 'the font has both weights');
  expect(
    drawn,
    closeTo(wanted, 0.5),
    reason:
        '"$needle" is drawn ${drawn.toStringAsFixed(2)} wide; '
        'w${weight.value} is ${wanted.toStringAsFixed(2)}, '
        'regular is ${regular.toStringAsFixed(2)}',
  );
}

/// The paragraph that shows [needle].
Finder paragraphWith(String needle) => find.byWidgetPredicate(
  (w) => w is RichText && w.text.toPlainText().contains(needle),
);

void main() {
  test('dsTextStyle pins no axis of the font', () {
    for (final role in [DsType.body, DsType.title, DsType.code]) {
      expect(dsTextStyle(role).fontVariations, isNull);
      expect(dsTextStyle(role).fontWeight, role.fontWeight);
    }
  });

  test('a role keeps its own weight on the glyphs', () {
    // Unchanged by the fix: the engine sets the axis from the weight of the role.
    const text = 'Release notes';
    for (final role in [DsType.bodyStrong, DsType.label, DsType.title]) {
      expect(
        widthOf(text, dsTextStyle(role)),
        closeTo(widthOf(text, atWeight(role, role.fontWeight!)), 0.01),
      );
      expect(
        widthOf(text, dsTextStyle(role)),
        greaterThan(widthOf(text, atWeight(role, FontWeight.w400)) + 1),
      );
    }
  });

  test('a weight set after dsTextStyle changes the glyphs', () {
    const text = 'Release notes';
    for (final role in [DsType.bodySm, DsType.body, DsType.caption]) {
      for (final weight in [FontWeight.w600, FontWeight.w700]) {
        final bold = dsTextStyle(role).copyWith(fontWeight: weight);
        expect(
          widthOf(text, bold),
          closeTo(widthOf(text, atWeight(role, weight)), 0.01),
          reason: 'copyWith(fontWeight: w${weight.value}) on a role',
        );
        expect(
          widthOf(text, bold),
          greaterThan(widthOf(text, dsTextStyle(role)) + 1),
        );
      }
    }
  });

  testWidgets('a bold Text below the theme is drawn bold', (tester) async {
    // The path of a view that asks for bold on the ambient style of `buildDsTheme`.
    await pumpDs(
      tester,
      Builder(
        builder: (context) => Column(
          children: [
            const Text(
              'Not ready',
              style: TextStyle(fontWeight: FontWeight.w700),
            ),
            Text(
              'Rejected',
              style: Theme.of(context).textTheme.bodySmall!
                  .copyWith(fontWeight: FontWeight.w600),
            ),
          ],
        ),
      ),
    );
    expectDrawnAt(
      tester,
      paragraphWith('Not ready'),
      'Not ready',
      FontWeight.w700,
    );
    expectDrawnAt(
      tester,
      paragraphWith('Rejected'),
      'Rejected',
      FontWeight.w600,
    );
  });

  testWidgets('an unread thread row draws its title bold, a read one regular', (
    tester,
  ) async {
    Future<void> show(int unread) => pumpDs(
      tester,
      DsThreadRow(
        DsThreadRowData(
          id: 't1',
          title: 'Release notes',
          timeLabel: '10:42',
          unreadCount: unread,
        ),
        onOpen: () {},
      ),
    );

    await show(2);
    expectDrawnAt(
      tester,
      paragraphWith('Release notes'),
      'Release notes',
      FontWeight.w700,
    );

    await show(0);
    final read = tester.renderObject<RenderParagraph>(
      paragraphWith('Release notes'),
    );
    expect(
      read.getMaxIntrinsicWidth(double.infinity),
      closeTo(
        widthOf(
          'Release notes',
          atWeight(styleOf(read.text, 'Release notes'), FontWeight.w400),
        ),
        0.5,
      ),
    );
  });

  testWidgets('unread sidebar rows draw their name bold', (tester) async {
    Widget onSidebar(Widget child) => Builder(
      builder: (context) =>
          ColoredBox(color: context.dsColors.surfaceSidebar, child: child),
    );

    await pumpDs(
      tester,
      onSidebar(
        Column(
          children: [
            DsChannelRow(
              const DsChannelRowData(id: 'c', name: 'deploys', unreadCount: 3),
              onOpen: () {},
            ),
            DsSidebarThreadRow(
              const DsSidebarThreadRowData(
                id: 't',
                title: 'Flaky test',
                unreadCount: 1,
              ),
              onOpen: () {},
            ),
            DsSidebarAllPostsRow(unreadCount: 4, onOpen: () {}),
          ],
        ),
      ),
    );
    final l = await en();
    for (final name in ['deploys', 'Flaky test', l.sidebarAllPosts]) {
      expectDrawnAt(tester, paragraphWith(name), name, FontWeight.w700);
    }
  });

  testWidgets('Markdown bold and a mention are drawn semi-bold', (
    tester,
  ) async {
    await pumpDs(
      tester,
      DsMarkdown(
        'Plain and **strong** words for @sam here',
        onOpenLink: (_) {},
        mentionHandles: const {'sam'},
      ),
    );
    expectDrawnAt(tester, paragraphWith('strong'), 'strong', FontWeight.w600);
    expectDrawnAt(tester, paragraphWith('@sam'), '@sam', FontWeight.w600);
  });
}
