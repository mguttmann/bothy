// SPDX-License-Identifier: Apache-2.0

// Readable code blocks (01 6.14, R-PR-28; brief M4-49): a fenced block wraps long lines by
// default so its whole content is visible, the toggle "Wrap lines" in its header switches to one
// line per line with a working horizontal scroll (a visible scrollbar, shift and the wheel, the
// trackpad and a drag), the choice goes to the device through [DsCodeBlockScope], a long block
// shows collapsed with a fade and "Show all" / "Show less", wrapped lines keep their indentation
// (hanging indent), and the copy stays the exact text of the block (01 6.12 item 3).

import 'dart:ui' show Tristate;

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support.dart';

/// Records what the renderer writes.
final class _Recorder implements DsClipboard {
  final List<DsCopyData> writes = [];

  @override
  Future<void> write(DsCopyData data) async => writes.add(data);
}

const _desktop = TargetPlatformVariant({
  TargetPlatform.macOS,
  TargetPlatform.windows,
});

const _phone = TargetPlatformVariant({
  TargetPlatform.iOS,
  TargetPlatform.android,
});

/// The live case of the owner's report: one long sentence as a fenced block "to copy".
const _sentence =
    'Please copy this long sentence into the ticket, so that everyone involved '
    'knows exactly which steps are still open tonight and who takes them over '
    'before the release on Monday.';

/// A block of [n] numbered lines.
String _lines(int n) => [for (var i = 1; i <= n; i++) 'line $i'].join('\n');

/// The device preference of a test: what [DsCodeBlockScope] holds and what the toggle wrote.
final class _Device {
  _Device({this.wrap = true});

  bool wrap;
  final List<bool> written = [];
}

Future<_Recorder> _pump(
  WidgetTester tester,
  String source, {
  bool touch = false,
  double width = 400,
  _Device? device,
  int collapseLines = DsCodeBlockScope.defaultCollapseLines,
  Locale locale = const Locale('en'),
  bool selectable = false,
}) async {
  final recorder = _Recorder();
  Widget body(StateSetter setState) {
    final markdown = DsMarkdown(source, onOpenLink: (_) {});
    final content = device == null
        ? DsCodeBlockScope(
            wrap: true,
            collapseLines: collapseLines,
            child: markdown,
          )
        : DsCodeBlockScope(
            wrap: device.wrap,
            collapseLines: collapseLines,
            onWrapChanged: (wrap) => setState(() {
              device.written.add(wrap);
              device.wrap = wrap;
            }),
            child: markdown,
          );
    return selectable ? SelectionArea(child: content) : content;
  }

  await tester.pumpWidget(
    MaterialApp(
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: DsThemeScope(
        dsContext: testContext(
          density: touch ? DsDensityMode.touch : DsDensityMode.comfortable,
        ),
        child: Scaffold(
          body: Align(
            alignment: Alignment.topLeft,
            child: SizedBox(
              width: width,
              child: SingleChildScrollView(
                child: DsClipboardScope(
                  clipboard: recorder,
                  child: StatefulBuilder(
                    builder: (context, setState) => body(setState),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump();
  return recorder;
}

Finder get _block => find.byType(DsMarkdownCodeBlock).first;

/// The horizontal scroll of the block, or nothing while it wraps.
Finder get _scroll => find.descendant(
  of: _block,
  matching: find.byWidgetPredicate(
    (w) => w is Scrollable && w.axisDirection == AxisDirection.right,
  ),
);

double _offset(WidgetTester tester) =>
    tester.state<ScrollableState>(_scroll).position.pixels;

/// The height of one line of `type.code` at text scale 1.
double _lineHeight() {
  final painter = TextPainter(
    text: TextSpan(text: 'X', style: dsTextStyle(DsType.code)),
    textDirection: TextDirection.ltr,
  )..layout();
  final height = painter.preferredLineHeight;
  painter.dispose();
  return height;
}

/// The rectangle of the text that contains [part], inside the block.
Rect _textRect(WidgetTester tester, String part) => tester.getRect(
  find.descendant(of: _block, matching: find.textContaining(part)).first,
);

/// A point on the first line of code, inside the visible part of the block.
Offset _onCode(WidgetTester tester) => Offset(
  tester.getRect(_block).left + 60,
  _textRect(tester, 'Please').top + _lineHeight() / 2,
);

Future<void> _toggle(WidgetTester tester) async {
  final l = await en();
  await tester.tap(find.bySemanticsLabel(l.markdownCodeWrap));
  await tester.pump();
  await tester.pump();
}

void _haptics(WidgetTester tester) {
  tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
    SystemChannels.platform,
    (call) async => null,
  );
}

void main() {
  group('wrap by default', () {
    for (final touch in [false, true]) {
      final layout = touch ? 'phone' : 'desktop';
      testWidgets('$layout: a long single line wraps and shows all of it', (
        tester,
      ) async {
        if (touch) _haptics(tester);
        await _pump(tester, '```\n$_sentence\n```', touch: touch);
        final block = tester.getRect(_block);
        final text = _textRect(tester, 'Please copy');
        // No horizontal scroll: the whole sentence is inside the block, on several lines.
        expect(_scroll, findsNothing);
        expect(text.right, lessThanOrEqualTo(block.right));
        expect(text.height, greaterThan(_lineHeight() * 2));
      }, variant: touch ? _phone : _desktop);
    }

    testWidgets('a wrapped block copies the exact original text', (
      tester,
    ) async {
      final recorder = await _pump(tester, '```\n$_sentence\n```');
      final l = await en();
      await tester.tap(find.bySemanticsLabel(l.markdownCopyCode));
      await tester.pump();
      expect(recorder.writes, [const DsCopyData(_sentence)]);
    }, variant: _desktop);

    testWidgets('a wrapped indented line keeps its indentation on every '
        'line (hanging indent)', (tester) async {
      final code = 'fn main() {\n        $_sentence\n}';
      final recorder = await _pump(tester, '```\n$code\n```');
      final first = _textRect(tester, 'fn main');
      final rest = _textRect(tester, 'Please copy');
      // The wrapped text starts right of the code's left edge and keeps that start on each of
      // its lines, because the whole rest of the line is one box beside the indentation.
      expect(rest.left, greaterThan(first.left + 20));
      expect(rest.height, greaterThan(_lineHeight() * 2));
      final l = await en();
      await tester.tap(find.bySemanticsLabel(l.markdownCopyCode));
      expect(recorder.writes.single.text, code);
    }, variant: _desktop);

    testWidgets('a deep indentation takes at most DsCodeLines.maxIndentColumns '
        'columns of the wrapped line', (tester) async {
      final code = 'top\n${' ' * 40}$_sentence';
      final recorder = await _pump(tester, '```\n$code\n```');
      final painter = TextPainter(
        text: TextSpan(
          text: '0' * DsCodeLines.maxIndentColumns,
          style: dsTextStyle(DsType.code),
        ),
        textDirection: TextDirection.ltr,
      )..layout();
      final indent =
          _textRect(tester, 'Please').left - _textRect(tester, 'top').left;
      expect(DsCodeLines.maxIndentColumns, 16);
      expect(indent, moreOrLessEquals(painter.width, epsilon: 1));
      painter.dispose();
      final l = await en();
      await tester.tap(find.bySemanticsLabel(l.markdownCopyCode));
      expect(recorder.writes.single.text, code);
    }, variant: _desktop);

    testWidgets('blank lines keep their height and the order of the lines', (
      tester,
    ) async {
      await _pump(tester, '```\nfirst\n\n\nlast\n```');
      final first = _textRect(tester, 'first');
      final last = _textRect(tester, 'last');
      expect(
        last.top - first.top,
        moreOrLessEquals(_lineHeight() * 3, epsilon: 1),
      );
    }, variant: _desktop);

    testWidgets('the system selection of a wrapped block keeps its line '
        'breaks and indentation', (tester) async {
      String? copied;
      tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        (call) async {
          if (call.method == 'Clipboard.setData') {
            copied = (call.arguments as Map)['text'] as String;
          }
          return null;
        },
      );
      await _pump(tester, '```\nalpha\n  beta\ngamma\n```', selectable: true);
      final start = tester.getTopLeft(find.text('alpha'));
      final end = tester.getBottomRight(find.text('gamma'));
      final gesture = await tester.startGesture(
        start + const Offset(1, 2),
        kind: PointerDeviceKind.mouse,
      );
      await tester.pump();
      await gesture.moveTo(end - const Offset(1, 2));
      await tester.pump();
      await gesture.up();
      await tester.pump();
      final mod = defaultTargetPlatform == TargetPlatform.macOS
          ? LogicalKeyboardKey.meta
          : LogicalKeyboardKey.control;
      await tester.sendKeyDownEvent(mod);
      await tester.sendKeyEvent(LogicalKeyboardKey.keyC);
      await tester.sendKeyUpEvent(mod);
      await tester.pump();
      expect(copied, 'alpha\n  beta\ngamma');
    }, variant: _desktop);
  });

  group('toggle', () {
    testWidgets('desktop: the toggle sits in the header beside the copy '
        'button and switches to no wrap with a visible scrollbar', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      final device = _Device();
      await _pump(tester, '```\n$_sentence\n```', device: device);
      final l = await en();
      final toggle = find.bySemanticsLabel(l.markdownCodeWrap);
      final copy = find.bySemanticsLabel(l.markdownCopyCode);
      expect(toggle, findsOneWidget);
      // In the header: the same row as the copy button, left of it.
      expect(
        tester.getCenter(toggle).dy,
        moreOrLessEquals(tester.getCenter(copy).dy, epsilon: 1),
      );
      expect(tester.getCenter(toggle).dx, lessThan(tester.getCenter(copy).dx));
      expect(
        tester
            .getSemantics(toggle)
            .getSemanticsData()
            .flagsCollection
            .isToggled,
        Tristate.isTrue,
      );

      await _toggle(tester);
      expect(device.written, [false]);
      expect(_scroll, findsOneWidget);
      expect(
        _textRect(tester, 'Please copy').height,
        moreOrLessEquals(_lineHeight(), epsilon: 1),
      );
      final bar = tester.widget<RawScrollbar>(
        find.descendant(
          of: _block,
          matching: find.byWidgetPredicate((w) => w is RawScrollbar),
        ),
      );
      expect(bar.thumbVisibility, isTrue);
      expect(
        tester
            .getSemantics(toggle)
            .getSemanticsData()
            .flagsCollection
            .isToggled,
        Tristate.isFalse,
      );

      // And back.
      await _toggle(tester);
      expect(device.written, [false, true]);
      expect(_scroll, findsNothing);
      handle.dispose();
    }, variant: _desktop);

    testWidgets('the device choice applies to every block and to new ones', (
      tester,
    ) async {
      final device = _Device(wrap: false);
      await _pump(
        tester,
        '```\n$_sentence\n```\n\n```\n$_sentence\n```',
        device: device,
      );
      expect(
        find.descendant(
          of: find.byType(DsMarkdownCodeBlock),
          matching: find.byWidgetPredicate(
            (w) => w is Scrollable && w.axisDirection == AxisDirection.right,
          ),
        ),
        findsNWidgets(2),
      );
      final l = await en();
      await tester.tap(find.bySemanticsLabel(l.markdownCodeWrap).first);
      await tester.pump();
      await tester.pump();
      expect(device.written, [true]);
      expect(_scroll, findsNothing);
      expect(
        find.descendant(
          of: find.byType(DsMarkdownCodeBlock).last,
          matching: find.byType(Scrollable),
        ),
        findsNothing,
      );
    }, variant: _desktop);

    testWidgets('without a callback the toggle switches the block itself', (
      tester,
    ) async {
      await _pump(tester, '```\n$_sentence\n```');
      await _toggle(tester);
      expect(_scroll, findsOneWidget);
    }, variant: _desktop);

    testWidgets('the copy button stays first for the keyboard; the toggle '
        'follows', (tester) async {
      final device = _Device();
      final recorder = await _pump(tester, '```\nx\n```', device: device);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pump();
      expect(recorder.writes, [const DsCopyData('x')]);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pump();
      expect(device.written, [false]);
    }, variant: _desktop);

    testWidgets('phone: a tap on the toggle switches and copies nothing', (
      tester,
    ) async {
      _haptics(tester);
      final device = _Device();
      final recorder = await _pump(
        tester,
        '```\n$_sentence\n```',
        touch: true,
        device: device,
      );
      final l = await en();
      // No copy button on the phone, but the toggle is there.
      expect(find.bySemanticsLabel(l.markdownCopyCode), findsOneWidget);
      expect(find.text(l.markdownCopyCode), findsNothing);
      await _toggle(tester);
      expect(device.written, [false]);
      expect(recorder.writes, isEmpty);
      expect(_scroll, findsOneWidget);
      // A tap on the code still copies the exact text.
      await tester.tapAt(_onCode(tester));
      await tester.pump();
      expect(recorder.writes, [const DsCopyData(_sentence)]);
    }, variant: _phone);

    testWidgets('the German label', (tester) async {
      await _pump(tester, '```\nx\n```', locale: const Locale('de'));
      final l = await de();
      expect(l.markdownCodeWrap, 'Zeilen umbrechen');
      expect(find.text(l.markdownCodeWrap), findsOneWidget);
    }, variant: _desktop);
  });

  group('scroll without wrap', () {
    testWidgets('desktop: shift and the mouse wheel scroll sideways', (
      tester,
    ) async {
      await _pump(tester, '```\n$_sentence\n```', device: _Device(wrap: false));
      final pointer = TestPointer(1, PointerDeviceKind.mouse);
      final at = _onCode(tester);
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendEventToBinding(pointer.hover(at));
      await tester.sendEventToBinding(pointer.scroll(const Offset(0, 120)));
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await tester.pumpAndSettle();
      expect(_offset(tester), greaterThan(0));
    }, variant: _desktop);

    testWidgets('desktop: a horizontal trackpad pan scrolls sideways', (
      tester,
    ) async {
      await _pump(tester, '```\n$_sentence\n```', device: _Device(wrap: false));
      final at = _onCode(tester);
      final gesture = await tester.createGesture(
        kind: PointerDeviceKind.trackpad,
      );
      await gesture.panZoomStart(at);
      await gesture.panZoomUpdate(at, pan: const Offset(-150, 0));
      await gesture.panZoomEnd();
      await tester.pumpAndSettle();
      expect(_offset(tester), greaterThan(0));
    }, variant: _desktop);

    testWidgets('desktop: dragging the scrollbar thumb scrolls sideways', (
      tester,
    ) async {
      await _pump(tester, '```\n$_sentence\n```', device: _Device(wrap: false));
      // The always visible thumb fades in once; until then it takes no press.
      await tester.pumpAndSettle();
      final area = tester.getRect(_scroll);
      // The thumb runs along the bottom edge of the scroll area, from its left end.
      final thumb = Offset(area.left + 20, area.bottom - 3);
      // The pointer comes to the thumb first, as a mouse does.
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
      expect(_offset(tester), greaterThan(0));
    }, variant: _desktop);

    testWidgets('phone: a drag scrolls sideways and copies nothing', (
      tester,
    ) async {
      _haptics(tester);
      final recorder = await _pump(
        tester,
        '```\n$_sentence\n```',
        touch: true,
        device: _Device(wrap: false),
      );
      await tester.dragFrom(_onCode(tester), const Offset(-200, 0));
      await tester.pumpAndSettle();
      expect(_offset(tester), greaterThan(0));
      expect(recorder.writes, isEmpty);
    }, variant: _phone);

    testWidgets('a block without wrap still copies the exact original text', (
      tester,
    ) async {
      final code = '\tindented $_sentence  ';
      final recorder = await _pump(
        tester,
        '```\n$code\n```',
        device: _Device(wrap: false),
      );
      final l = await en();
      await tester.tap(find.bySemanticsLabel(l.markdownCopyCode));
      expect(recorder.writes, [DsCopyData(code)]);
    }, variant: _desktop);
  });

  group('long blocks', () {
    for (final touch in [false, true]) {
      final layout = touch ? 'phone' : 'desktop';
      testWidgets('$layout: a block above the limit shows collapsed with a '
          'fade and expands in place', (tester) async {
        if (touch) _haptics(tester);
        final recorder = await _pump(
          tester,
          '```\n${_lines(40)}\n```',
          touch: touch,
        );
        final l = await en();
        final block = tester.getRect(_block);
        final line = _lineHeight();
        // About 20 lines of code plus the header, far less than 40 lines.
        expect(block.height, lessThan(line * 20 + 120));
        expect(block.height, greaterThan(line * 20));
        expect(find.byKey(DsMarkdownCodeBlock.fadeKey), findsOneWidget);
        expect(find.text(l.markdownCodeShowAll), findsOneWidget);
        // The last line is cut off below the visible part.
        expect(_textRect(tester, 'line 40').top, greaterThan(block.bottom));

        await tester.tap(find.text(l.markdownCodeShowAll));
        await tester.pump();
        await tester.pump();
        final open = tester.getRect(_block);
        expect(open.height, greaterThan(line * 40));
        expect(_textRect(tester, 'line 40').bottom, lessThan(open.bottom));
        expect(find.byKey(DsMarkdownCodeBlock.fadeKey), findsNothing);
        expect(find.text(l.markdownCodeShowLess), findsOneWidget);
        // Expanding copies nothing, also on the phone.
        expect(recorder.writes, isEmpty);

        await tester.ensureVisible(find.text(l.markdownCodeShowLess));
        await tester.pump();
        await tester.tap(find.text(l.markdownCodeShowLess));
        await tester.pump();
        await tester.pump();
        expect(tester.getRect(_block).height, moreOrLessEquals(block.height));
        expect(find.text(l.markdownCodeShowAll), findsOneWidget);
      }, variant: touch ? _phone : _desktop);
    }

    testWidgets('a block within the limit shows whole without a control', (
      tester,
    ) async {
      await _pump(tester, '```\n${_lines(20)}\n```');
      final l = await en();
      expect(find.text(l.markdownCodeShowAll), findsNothing);
      expect(find.byKey(DsMarkdownCodeBlock.fadeKey), findsNothing);
      expect(
        _textRect(tester, 'line 20').bottom,
        lessThan(tester.getRect(_block).bottom),
      );
    }, variant: _desktop);

    testWidgets('the limit comes from the scope', (tester) async {
      await _pump(tester, '```\n${_lines(8)}\n```', collapseLines: 5);
      final l = await en();
      expect(find.text(l.markdownCodeShowAll), findsOneWidget);
    }, variant: _desktop);

    testWidgets('one long wrapped line above the limit collapses too', (
      tester,
    ) async {
      await _pump(
        tester,
        '```\n${List.filled(12, _sentence).join(' ')}\n```',
        width: 300,
      );
      final l = await en();
      expect(find.text(l.markdownCodeShowAll), findsOneWidget);
    }, variant: _desktop);

    testWidgets('a collapsed block copies all of its lines', (tester) async {
      final recorder = await _pump(tester, '```\n${_lines(40)}\n```');
      final l = await en();
      await tester.tap(find.bySemanticsLabel(l.markdownCopyCode));
      expect(recorder.writes, [DsCopyData(_lines(40))]);
    }, variant: _desktop);

    testWidgets('the screen reader reads the expand control with its state', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      await _pump(tester, '```\n${_lines(40)}\n```');
      final l = await en();
      final data = tester
          .getSemantics(find.bySemanticsLabel(l.markdownCodeShowAll))
          .getSemanticsData();
      expect(data.hasAction(SemanticsAction.tap), isTrue);
      expect(data.flagsCollection.isExpanded, Tristate.isFalse);
      handle.dispose();
    }, variant: _desktop);

    testWidgets('the German labels', (tester) async {
      await _pump(
        tester,
        '```\n${_lines(40)}\n```',
        locale: const Locale('de'),
      );
      final l = await de();
      expect(l.markdownCodeShowAll, 'Alles anzeigen');
      expect(l.markdownCodeShowLess, 'Weniger');
      await tester.tap(find.text('Alles anzeigen'));
      await tester.pump();
      await tester.pump();
      expect(find.text('Weniger'), findsOneWidget);
    }, variant: _desktop);
  });
}
