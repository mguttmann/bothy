// SPDX-License-Identifier: Apache-2.0

// The Spaces page M1 of the phone in both themes (FIX-light-theme-contrast; D-DS-02): the page
// of the running phone shell, not a scene, stands on the brand surface `surface.sidebar`, which
// every `text.on-sidebar*` and `category.*.on-sidebar` role pairs with in design-rules.json, and
// all its texts reach the WCAG AA contrast. Before the fix the page stood on `surface.canvas`:
// in the light theme the light spruce texts and icons were nearly invisible on near white.

import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/new_chat_fixtures.dart';
import 'support/phone_threads.dart';

/// Whether a box between [row] and the page paints `surface.sidebar` of [colors].
bool _onSidebarSurface(WidgetTester tester, Finder row, DsColors colors) {
  final surfaces = find.ancestor(
    of: row,
    matching: find.byWidgetPredicate(
      (w) => w is ColoredBox && w.color == colors.surfaceSidebar,
    ),
  );
  return surfaces.evaluate().isNotEmpty;
}

void main() {
  for (final platform in const ['ios', 'android']) {
    for (final theme in DsThemeMode.values) {
      testPhone('M1 stands on surface.sidebar in ${theme.name}', platform, (
        tester,
      ) async {
        final host = await ColumnHost.open(tester);
        await tester.runAsync(() => fillThreads(host.db));
        await pumpPhone(tester, host, platform: platform, theme: theme);
        final colors = theme == DsThemeMode.dark
            ? DsColors.dark
            : DsColors.light;
        for (final row in [
          find.byKey(const ValueKey('sidebar.productName')),
          find.byKey(const ValueKey('sidebar.runningNow')),
          channelRow(server),
          allPostsRow(server),
        ]) {
          expect(row, findsOneWidget);
          expect(_onSidebarSurface(tester, row, colors), isTrue);
        }
        await host.dispose(tester);
      });

      testPhone('M1 texts meet WCAG AA in ${theme.name}', platform, (
        tester,
      ) async {
        final handle = tester.ensureSemantics();
        final host = await ColumnHost.open(tester);
        await tester.runAsync(() => fillThreads(host.db));
        await pumpPhone(tester, host, platform: platform, theme: theme);
        // The guideline measures the rendered glyphs, whose anti-aliasing depends on the host;
        // it runs on macOS like the design system's (docs/DECISIONS.md D-M4-07-10), the surface
        // test above and the token contrast test run on every host.
        if (Platform.isMacOS) {
          await expectLater(tester, meetsGuideline(textContrastGuideline));
        }
        await host.dispose(tester);
        handle.dispose();
      });
    }
  }
}
