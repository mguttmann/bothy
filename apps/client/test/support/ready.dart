// SPDX-License-Identifier: Apache-2.0

// The tap of the integration tests. A screen shows its buttons before its data has loaded and
// keeps them disabled until then; a tap on a disabled button does nothing, so a test that taps
// as soon as the key is found loses the tap and fails later (the macOS app job failed on main
// when `people.add` was tapped before the people list had loaded). [tapWhenEnabled] waits until
// the target is shown and enabled. A page that is pushed slides in, and a tap during the slide
// lands beside its target, often beyond the screen; [untilStill] waits until the target stands
// still on the screen, and [tapWhenEnabled] does that before it taps.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'until.dart';

/// Pumps until [finder] finds a widget whose button is enabled, then makes it visible, waits
/// until it stands still on the screen ([untilStill]), taps it and settles; fails after
/// [timeout] of pumping.
///
/// The button is the first [ButtonStyleButton], [IconButton] or [InkResponse] at or below the
/// found widget; a widget without one counts as enabled once it is shown. The wait counts the
/// pumped time, so a headless test with fake time and a live test on a device wait alike.
Future<void> tapWhenEnabled(
  WidgetTester tester,
  Finder finder, {
  Duration timeout = testPatience,
}) async {
  const step = Duration(milliseconds: 50);
  var waited = Duration.zero;
  while (!_enabled(finder)) {
    if (waited >= timeout) {
      fail('timed out waiting for $finder to be shown and enabled');
    }
    await tester.pump(step);
    waited += step;
  }
  await tester.ensureVisible(finder.first);
  await untilStill(tester, finder.first, timeout: timeout);
  await tester.tap(finder.first);
  await tester.pumpAndSettle();
}

/// Pumps until [finder]'s widget stands still with its centre on the screen, so a tap or a
/// press at its centre lands on it; fails after [timeout] of pumping.
///
/// A pushed page slides in (on iOS from the right edge, `CupertinoPage`): its rows and buttons
/// are found, and enabled, while the page is still moving, and a tap at a centre beyond the
/// screen does nothing. "Still" means the same rectangle in two frames one step apart. The wait
/// counts the pumped time, as [tapWhenEnabled] does.
Future<void> untilStill(
  WidgetTester tester,
  Finder finder, {
  Duration timeout = testPatience,
}) async {
  const step = Duration(milliseconds: 50);
  final view = tester.view;
  final screen = Offset.zero & (view.physicalSize / view.devicePixelRatio);
  var waited = Duration.zero;
  Rect? last;
  while (true) {
    final now = tester.getRect(finder);
    if (now == last && screen.contains(now.center)) return;
    if (waited >= timeout) {
      fail('timed out waiting for $finder to stand still on the screen ($now)');
    }
    last = now;
    await tester.pump(step);
    waited += step;
  }
}

bool _enabled(Finder finder) {
  final found = finder.evaluate();
  if (found.isEmpty) return false;
  final element = found.first;
  bool? enabled = _buttonEnabled(element.widget);
  void visit(Element child) {
    if (enabled != null) return;
    enabled = _buttonEnabled(child.widget);
    if (enabled == null) child.visitChildren(visit);
  }

  if (enabled == null) element.visitChildren(visit);
  return enabled ?? true;
}

bool? _buttonEnabled(Widget widget) => switch (widget) {
  ButtonStyleButton() => widget.enabled,
  IconButton() => widget.onPressed != null,
  InkResponse() => widget.onTap != null,
  _ => null,
};
