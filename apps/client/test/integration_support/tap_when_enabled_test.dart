// SPDX-License-Identifier: Apache-2.0

// tapWhenEnabled, the tap of the integration tests: a button that is shown before its screen
// finished loading is disabled, and a tap then does nothing (the macOS app job failed on
// main when `people.add` was tapped before the people list had loaded). The helper waits
// until the target is shown and enabled.

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/ready.dart';

/// A button that is disabled until [enableAfter] passed and counts its taps.
class _LateButton extends StatefulWidget {
  const _LateButton({required this.kind, this.enableAfter, this.showAfter});

  final String kind;
  final Duration? enableAfter;
  final Duration? showAfter;

  @override
  State<_LateButton> createState() => _LateButtonState();
}

class _LateButtonState extends State<_LateButton> {
  static int taps = 0;
  bool enabled = false;
  bool shown = true;
  final timers = <Timer>[];

  @override
  void initState() {
    super.initState();
    if (widget.enableAfter case final after?) {
      timers.add(Timer(after, () => setState(() => enabled = true)));
    }
    if (widget.showAfter case final after?) {
      shown = false;
      timers.add(Timer(after, () => setState(() => shown = true)));
    }
  }

  @override
  void dispose() {
    for (final t in timers) {
      t.cancel();
    }
    super.dispose();
  }

  void _tap() => taps++;

  @override
  Widget build(BuildContext context) {
    if (!shown) return const SizedBox.shrink();
    final onTap = enabled ? _tap : null;
    const key = ValueKey('late');
    return switch (widget.kind) {
      'filled' => FilledButton(
        key: key,
        onPressed: onTap,
        child: const Text('Add'),
      ),
      'icon' => IconButton(
        key: key,
        onPressed: onTap,
        icon: const Icon(Icons.add),
      ),
      'inkwell' => Padding(
        key: key,
        padding: const EdgeInsets.all(8),
        child: InkWell(onTap: onTap, child: const Text('Row')),
      ),
      _ => throw ArgumentError(widget.kind),
    };
  }
}

Future<void> _host(WidgetTester tester, Widget child) => tester.pumpWidget(
  MaterialApp(
    home: Scaffold(body: Center(child: child)),
  ),
);

void main() {
  setUp(() => _LateButtonState.taps = 0);

  for (final kind in ['filled', 'icon', 'inkwell']) {
    testWidgets('a $kind target shown disabled is tapped once it is enabled', (
      tester,
    ) async {
      await _host(
        tester,
        _LateButton(kind: kind, enableAfter: const Duration(seconds: 2)),
      );
      // The old helpers waited only until the key was found and tapped at once.
      expect(find.byKey(const ValueKey('late')), findsOneWidget);
      await tapWhenEnabled(tester, find.byKey(const ValueKey('late')));
      expect(_LateButtonState.taps, 1);
    });
  }

  testWidgets('a target that is not shown yet is waited for', (tester) async {
    await _host(
      tester,
      const _LateButton(
        kind: 'filled',
        showAfter: Duration(seconds: 1),
        enableAfter: Duration(seconds: 3),
      ),
    );
    expect(find.byKey(const ValueKey('late')), findsNothing);
    await tapWhenEnabled(tester, find.byKey(const ValueKey('late')));
    expect(_LateButtonState.taps, 1);
  });

  testWidgets('a target without a known button is tapped once it is shown', (
    tester,
  ) async {
    var taps = 0;
    await _host(
      tester,
      GestureDetector(
        key: const ValueKey('plain'),
        onTap: () => taps++,
        child: const Text('Plain'),
      ),
    );
    await tapWhenEnabled(tester, find.byKey(const ValueKey('plain')));
    expect(taps, 1);
  });

  testWidgets('a target that slides in is tapped once it stands still', (
    tester,
  ) async {
    // A pushed page slides in from the right edge: its button is found and enabled while its
    // centre is still beyond the screen, and a tap there misses (`apps (iOS)` and
    // `apps (macOS)`, a tap on a row or button of a page still in its transition).
    var taps = 0;
    final controller = AnimationController(
      vsync: const TestVSync(),
      duration: const Duration(seconds: 1),
    );
    addTearDown(controller.dispose);
    await _host(
      tester,
      SlideTransition(
        position: Tween(
          begin: const Offset(20, 0),
          end: Offset.zero,
        ).animate(controller),
        child: FilledButton(
          key: const ValueKey('sliding'),
          onPressed: () => taps++,
          child: const Text('Log in again'),
        ),
      ),
    );
    unawaited(controller.forward());
    await tester.pump();
    final screen = tester.getRect(find.byType(Scaffold));
    expect(
      screen.contains(tester.getCenter(find.byKey(const ValueKey('sliding')))),
      isFalse,
      reason: 'the button starts beyond the screen',
    );
    await tapWhenEnabled(tester, find.byKey(const ValueKey('sliding')));
    expect(taps, 1);
  });

  testWidgets(
    'untilStill fails after the timeout for a target off the screen',
    (tester) async {
      await _host(
        tester,
        FractionalTranslation(
          translation: const Offset(20, 0),
          child: FilledButton(
            key: const ValueKey('away'),
            onPressed: () {},
            child: const Text('Away'),
          ),
        ),
      );
      Object? failure;
      try {
        await untilStill(
          tester,
          find.byKey(const ValueKey('away')),
          timeout: const Duration(seconds: 2),
        );
      } on TestFailure catch (e) {
        failure = e;
      }
      expect(
        failure,
        isA<TestFailure>().having(
          (f) => f.message,
          'message',
          contains('stand still on the screen'),
        ),
      );
    },
  );

  testWidgets('a target that stays disabled fails after the timeout', (
    tester,
  ) async {
    await _host(tester, const _LateButton(kind: 'filled'));
    Object? failure;
    try {
      await tapWhenEnabled(
        tester,
        find.byKey(const ValueKey('late')),
        timeout: const Duration(seconds: 5),
      );
    } on TestFailure catch (e) {
      failure = e;
    }
    expect(
      failure,
      isA<TestFailure>().having(
        (f) => f.message,
        'message',
        contains('shown and enabled'),
      ),
    );
    expect(_LateButtonState.taps, 0);
  });
}
