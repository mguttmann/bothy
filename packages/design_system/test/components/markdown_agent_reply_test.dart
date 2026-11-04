// SPDX-License-Identifier: Apache-2.0

// An agent reply in the format rules of docs/spec/09-agents.md 4.11 (F-AGENT-FORMAT), rendered
// as an agent message on every client platform: the desktop (macOS, Windows, density
// comfortable) and the phone (iOS, Android, density touch); the web console uses the same
// component. Running text wraps to the view, a narrow GFM table fits, a wide one keeps its
// columns on one line and scrolls sideways (with a visible scrollbar on the desktop, 01 6.14
// item 1 applied to tables), a code block shows its language, a `text` block keeps one line per
// paragraph for the copy, and a list shows its bullets. The golden of the same reply is
// test/goldens/components/agent_reply_golden_test.dart.

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../goldens/components/agent_reply.dart';
import 'support.dart';

const _desktop = TargetPlatformVariant({
  TargetPlatform.macOS,
  TargetPlatform.windows,
});

const _phone = TargetPlatformVariant({
  TargetPlatform.iOS,
  TargetPlatform.android,
});

const _all = TargetPlatformVariant({
  TargetPlatform.macOS,
  TargetPlatform.windows,
  TargetPlatform.iOS,
  TargetPlatform.android,
});

bool get _isPhone =>
    debugDefaultTargetPlatformOverride == TargetPlatform.iOS ||
    debugDefaultTargetPlatformOverride == TargetPlatform.android;

Future<void> _pump(
  WidgetTester tester, {
  bool? touch,
  double? width,
  bool selectable = false,
}) async {
  final phone = touch ?? _isPhone;
  final message = DsMessage(
    agentReplyMessage(),
    collapseLines: 400,
    onOpenLink: (_) {},
    selectable: selectable,
  );
  await tester.pumpWidget(
    MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: DsThemeScope(
        dsContext: testContext(
          density: phone ? DsDensityMode.touch : DsDensityMode.comfortable,
        ),
        child: Scaffold(
          body: Align(
            alignment: Alignment.topLeft,
            child: SizedBox(
              width: width ?? (phone ? 360 : 640),
              child: SingleChildScrollView(
                child: selectable ? SelectionArea(child: message) : message,
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump();
}

/// The table whose cells hold [text].
Finder _table(String text) => find.ancestor(
  of: find.textContaining(text, findRichText: true),
  matching: find.byType(Table),
);

/// The horizontal scroll around the table that holds [text].
Finder _tableScroll(String text) => find.ancestor(
  of: _table(text),
  matching: find.byWidgetPredicate(
    (w) => w is Scrollable && w.axisDirection == AxisDirection.right,
  ),
);

ScrollPosition _position(WidgetTester tester, String text) =>
    tester.state<ScrollableState>(_tableScroll(text).first).position;

/// The rendered paragraph whose text contains [text].
RenderParagraph _paragraph(WidgetTester tester, String text) =>
    tester.renderObject<RenderParagraph>(
      find.textContaining(text, findRichText: true).first,
    );

/// A point on the first cell of the wide table, inside the view.
Offset _onWideTable(WidgetTester tester) =>
    tester.getTopLeft(_table(agentReplyWideCell).first) + const Offset(12, 8);

double _lineHeight(RenderParagraph p) => p
    .getBoxesForSelection(const TextSelection(baseOffset: 0, extentOffset: 1))
    .first
    .toRect()
    .height;

void main() {
  testWidgets('every part renders without an overflow or a hard line break', (
    tester,
  ) async {
    await _pump(tester);
    expect(tester.takeException(), isNull);
    // The paragraph is one source line; the view wraps it to several lines.
    final text = _paragraph(tester, agentReplyParagraphStart);
    expect(text.text.toPlainText(), isNot(contains('\n')));
    expect(text.size.height, greaterThan(_lineHeight(text) * 1.5));
    // Code with its language label, the text block with `text`.
    expect(find.text('python'), findsOneWidget);
    expect(find.text('text'), findsOneWidget);
    // The list shows its bullets.
    expect(find.text('•'), findsNWidgets(2));
  }, variant: _all);

  testWidgets('a narrow table fits the view and does not scroll', (
    tester,
  ) async {
    await _pump(tester);
    expect(_position(tester, agentReplyNarrowCell).maxScrollExtent, 0);
  }, variant: _all);

  testWidgets('a wide table keeps each cell on one line and scrolls sideways', (
    tester,
  ) async {
    await _pump(tester);
    expect(tester.takeException(), isNull);
    final position = _position(tester, agentReplyWideCell);
    expect(position.maxScrollExtent, greaterThan(0));
    // No column collapses to a word per line: the longest cell stays one line.
    final cell = _paragraph(tester, agentReplyWideCell);
    expect(cell.size.height, lessThan(_lineHeight(cell) * 1.5));
  }, variant: _all);

  testWidgets('desktop: the wide table shows a visible scrollbar', (
    tester,
  ) async {
    await _pump(tester);
    // The scrollbar of the table, not the one of the page around the message.
    final bars = find.ancestor(
      of: _table(agentReplyWideCell),
      matching: find.descendant(
        of: find.byType(DsMessage),
        matching: find.byWidgetPredicate((w) => w is RawScrollbar),
      ),
    );
    expect(bars, findsOneWidget);
    expect(tester.widget<RawScrollbar>(bars).thumbVisibility, isTrue);
  }, variant: _desktop);

  testWidgets('desktop: shift and the mouse wheel scroll the wide table', (
    tester,
  ) async {
    await _pump(tester);
    final pointer = TestPointer(1, PointerDeviceKind.mouse);
    final at = _onWideTable(tester);
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
    await tester.sendEventToBinding(pointer.hover(at));
    await tester.sendEventToBinding(pointer.scroll(const Offset(0, 120)));
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
    await tester.pumpAndSettle();
    expect(_position(tester, agentReplyWideCell).pixels, greaterThan(0));
  }, variant: _desktop);

  testWidgets('desktop: a sideways trackpad pan scrolls the wide table', (
    tester,
  ) async {
    await _pump(tester);
    final at = _onWideTable(tester);
    final gesture = await tester.createGesture(
      kind: PointerDeviceKind.trackpad,
    );
    await gesture.panZoomStart(at);
    await gesture.panZoomUpdate(at, pan: const Offset(-150, 0));
    await gesture.panZoomEnd();
    await tester.pumpAndSettle();
    expect(_position(tester, agentReplyWideCell).pixels, greaterThan(0));
  }, variant: _desktop);

  testWidgets('desktop: dragging the scrollbar thumb scrolls the wide table', (
    tester,
  ) async {
    await _pump(tester);
    // The always visible thumb fades in once; until then it takes no press.
    await tester.pumpAndSettle();
    final area = tester.getRect(_tableScroll(agentReplyWideCell).first);
    final thumb = Offset(area.left + 20, area.bottom - 3);
    final gesture = await tester.createGesture(kind: PointerDeviceKind.mouse);
    await gesture.addPointer(location: thumb - const Offset(0, 20));
    await gesture.moveTo(thumb);
    await tester.pump();
    await gesture.down(thumb);
    await tester.pump();
    for (var i = 0; i < 4; i++) {
      await gesture.moveBy(const Offset(20, 0));
      await tester.pump();
    }
    await gesture.up();
    await tester.pumpAndSettle();
    expect(_position(tester, agentReplyWideCell).pixels, greaterThan(0));
  }, variant: _desktop);

  testWidgets('phone: a drag scrolls the wide table sideways', (tester) async {
    await _pump(tester);
    final at = _onWideTable(tester);
    await tester.dragFrom(at, const Offset(-200, 0));
    await tester.pumpAndSettle();
    expect(_position(tester, agentReplyWideCell).pixels, greaterThan(0));
  }, variant: _phone);

  testWidgets('with text selection the wide table still scrolls sideways', (
    tester,
  ) async {
    await _pump(tester, selectable: true);
    expect(tester.takeException(), isNull);
    expect(
      _position(tester, agentReplyWideCell).maxScrollExtent,
      greaterThan(0),
    );
  }, variant: _all);
}
