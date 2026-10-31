// SPDX-License-Identifier: Apache-2.0

// The phone density of the thread (brief M5-17; 01 5.3 M3, addendum of 2026-10-31; components.md
// 1, 2.3 and 9): in the density `touch` a message uses the full width with a small avatar in its
// header line, the input's text field fills the field between its buttons, starts at the left,
// grows to `maxLines` lines and then scrolls, and the mobile live box line is one line.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

const _first = DsMessageData(
  id: 'm1',
  authorName: 'Alex',
  origin: DsMessageOrigin.human,
  kind: DsMessageKind.text,
  timeLabel: '10:42',
  content: 'The build is green.',
);

DsRunData _run() => const DsRunData(
  id: 'r1',
  agentName: 'Robin',
  state: DsRunState.running,
  elapsed: Duration(seconds: 56),
  statusLine: 'compacting the conversation before the next step',
);

/// The phone width of the goldens (an iPhone of 390 points).
const double _phone = 390;

void main() {
  group('message gutter', () {
    for (final (density, inline) in const [
      (DsDensityMode.touch, true),
      (DsDensityMode.comfortable, false),
      (DsDensityMode.compact, false),
    ]) {
      testWidgets('${density.name}: content start and avatar', (tester) async {
        await pumpDs(
          tester,
          Column(
            children: [
              DsMessage(
                _first,
                key: const ValueKey('first'),
                collapseLines: 24,
                onOpenLink: (_) {},
              ),
              DsMessage(
                _first.copyWithContent('Deploying now.'),
                key: const ValueKey('next'),
                collapseLines: 24,
                isContinuation: true,
                onOpenLink: (_) {},
              ),
            ],
          ),
          density: density,
          width: _phone,
        );
        final first = tester.getTopLeft(find.byKey(const ValueKey('first')));
        final text = tester.getTopLeft(find.text('The build is green.'));
        final next = tester.getTopLeft(find.text('Deploying now.'));
        final avatar = tester.getSize(find.byType(DsAvatar));
        if (inline) {
          // The content starts at the row padding; the continuation aligns with it.
          expect(text.dx - first.dx, lessThanOrEqualTo(DsSpace.s12));
          expect(next.dx, text.dx);
          expect(avatar, const Size.square(DsSize.avatarSm));
          // The small avatar leads the header line, above the content.
          expect(
            tester.getBottomLeft(find.byType(DsAvatar)).dy,
            lessThanOrEqualTo(text.dy + 1),
          );
        } else {
          expect(
            text.dx - first.dx,
            greaterThanOrEqualTo(DsSize.avatarMd + DsSpace.s12),
          );
          expect(next.dx, text.dx);
          expect(avatar, const Size.square(DsSize.avatarMd));
        }
        // One avatar for the group: the continuation has none.
        expect(find.byType(DsAvatar), findsOneWidget);
      });
    }
  });

  group('input on the phone', () {
    Future<TextEditingController> pumpComposer(
      WidgetTester tester, {
      int maxLines = 4,
      bool trailing = true,
    }) async {
      final controller = TextEditingController();
      addTearDown(controller.dispose);
      await pumpDs(
        tester,
        DsComposer(
          targetName: 'Claude Code',
          maxLines: maxLines,
          enterSends: false,
          controller: controller,
          onSend: (_) {},
          onAttach: () {},
          trailing: trailing
              ? SizedBox.square(
                  key: ValueKey('mic'),
                  dimension: DsDensity.touch.targetMin,
                )
              : null,
        ),
        density: DsDensityMode.touch,
        width: _phone,
      );
      return controller;
    }

    Rect box(WidgetTester tester) =>
        tester.getRect(find.byKey(const ValueKey('ds-composer-field')));

    testWidgets('the text field fills the field between its buttons', (
      tester,
    ) async {
      final l = await en();
      await pumpComposer(tester);
      final field = tester.getRect(find.byType(TextField));
      final attach = tester.getRect(find.bySemanticsLabel(l.composerAttach));
      final mic = tester.getRect(find.byKey(const ValueKey('mic')));
      final send = tester.getRect(find.bySemanticsLabel(l.composerSend));
      // The trailing button sits inside the field, before Send.
      expect(box(tester).contains(mic.center), isTrue);
      expect(mic.right, lessThanOrEqualTo(send.left));
      // No gap of its own between the buttons and the text field: the field is the target.
      expect(field.left, closeTo(attach.right, 0.5));
      expect(field.right, closeTo(mic.left, 0.5));
      expect(field.height, greaterThanOrEqualTo(DsDensity.touch.targetMin));
      // Three buttons of 48 and the border: everything else is the text field.
      expect(
        field.width,
        greaterThanOrEqualTo(_phone - 3 * DsDensity.touch.targetMin - 2),
      );
    });

    testWidgets('the placeholder starts at the left of the text field', (
      tester,
    ) async {
      await pumpComposer(tester);
      final field = tester.getRect(find.byType(TextField));
      final hint = tester.getRect(
        find.text('Message to \u2068Claude Code\u2069'),
      );
      expect(
        hint.left - field.left,
        closeTo(DsDensity.touch.composerPadX, 0.5),
      );
      // Vertically centered on the buttons while the field is one line.
      expect((hint.center.dy - field.center.dy).abs(), lessThanOrEqualTo(2));
    });

    testWidgets('a tap anywhere in the text field focuses it', (tester) async {
      await pumpComposer(tester);
      final field = tester.getRect(find.byType(TextField));
      // The padding at the left edge and the empty space at the right.
      for (final at in [
        Offset(field.left + 1, field.top + 2),
        Offset(field.right - 2, field.bottom - 2),
      ]) {
        FocusManager.instance.primaryFocus?.unfocus();
        await tester.pump();
        await tester.tapAt(at);
        await tester.pump();
        expect(
          tester
              .widget<EditableText>(find.byType(EditableText))
              .focusNode
              .hasFocus,
          isTrue,
          reason: '$at',
        );
      }
    });

    testWidgets('the field grows to maxLines while typing and then scrolls', (
      tester,
    ) async {
      final controller = await pumpComposer(tester);
      final one = box(tester).height;
      controller.text = 'one\ntwo\nthree';
      await tester.pump();
      final three = box(tester).height;
      expect(three, greaterThan(one));
      controller.text = List.generate(12, (i) => 'line $i').join('\n');
      await tester.pump();
      final capped = box(tester).height;
      expect(capped, greaterThan(three));
      controller.text = List.generate(30, (i) => 'line $i').join('\n');
      await tester.pump();
      // Capped at four lines: more text scrolls inside the field.
      expect(box(tester).height, capped);
      final scrollable = tester.state<ScrollableState>(
        find.descendant(
          of: find.byType(TextField),
          matching: find.byType(Scrollable),
        ),
      );
      expect(scrollable.position.maxScrollExtent, greaterThan(0));
    });

    testWidgets('every button keeps the touch target', (tester) async {
      final l = await en();
      await pumpComposer(tester);
      for (final label in [l.composerAttach, l.composerSend]) {
        final size = tester.getSize(find.bySemanticsLabel(label));
        expect(size.width, greaterThanOrEqualTo(DsDensity.touch.targetMin));
        expect(size.height, greaterThanOrEqualTo(DsDensity.touch.targetMin));
      }
    });
  });

  testWidgets('mobile live box: one line, Stop at its end', (tester) async {
    final l = await en();
    await pumpDs(
      tester,
      DsLiveBox.mobile(
        _run(),
        key: const ValueKey('line'),
        announceMinInterval: const Duration(seconds: 10),
        onOpen: () {},
        onStop: () {},
      ),
      density: DsDensityMode.touch,
      width: _phone,
    );
    final line = tester.getRect(find.byKey(const ValueKey('line')));
    final text = find.byKey(const ValueKey('ds-livebox-mobile-text'));
    // State, run time and status line in this order in one line of text.
    final rendered = tester.widget<RichText>(
      find.descendant(of: text, matching: find.byType(RichText)),
    );
    expect(rendered.maxLines, 1);
    final plain = rendered.text.toPlainText();
    expect(plain.indexOf(l.liveBoxStateRunning), 0);
    expect(plain.indexOf('0:56'), greaterThan(0));
    expect(plain, contains('compacting the conversation'));
    expect(line.height, lessThanOrEqualTo(DsDensity.touch.targetMin + 2));
    final stop = tester.getRect(
      find.bySemanticsLabel(l.liveBoxStopRun('Robin')),
    );
    expect(stop.right, closeTo(line.right - DsSpace.s8, 1));
  });

  testWidgets('mobile live box at text scale 2 lays out in one line', (
    tester,
  ) async {
    tester.platformDispatcher.textScaleFactorTestValue = 2;
    addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
    await pumpDs(
      tester,
      DsLiveBox.mobile(
        _run(),
        announceMinInterval: const Duration(seconds: 10),
        onOpen: () {},
        onStop: () {},
      ),
      density: DsDensityMode.touch,
      width: _phone,
    );
    expect(tester.takeException(), isNull);
  });
}

extension on DsMessageData {
  DsMessageData copyWithContent(String text) => DsMessageData(
    id: '${id}b',
    authorName: authorName,
    origin: origin,
    kind: kind,
    timeLabel: timeLabel,
    content: text,
  );
}
