// SPDX-License-Identifier: Apache-2.0

import 'dart:async';
import 'dart:io';
import 'dart:ui' show CheckedState, Tristate;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../../design_system.dart';
import '../../testing.dart' show dsPumpIcons;
import 'semantics_audit.dart';

/// The logical size of the phone window of the accessibility tests (a 6.1 inch iPhone, three
/// device pixels per logical pixel).
const Size dsPhoneA11yWindow = Size(390, 844);

/// [child] on the phone for an accessibility test: the delegates of `package:l10n` in
/// [locale], the text scale [textScale], [wrap] around the app (the app's form factor scope,
/// for example), the design theme of [theme] in
/// the density `touch` (01 11) and the platform [platform] (`TargetPlatform.iOS` or
/// `TargetPlatform.android`, so the controls of that system are built). Motion is reduced and
/// the media query says so ([MediaQueryData.disableAnimations]), as on a phone with the system
/// setting "Reduce motion" or "Remove animations" on.
Widget dsPhoneA11yHost(
  Widget child, {
  Locale locale = const Locale('en'),
  double textScale = 1,
  DsThemeMode theme = DsThemeMode.light,
  TargetPlatform platform = TargetPlatform.iOS,
  Widget Function(Widget child)? wrap,
}) {
  final ds = DsContext(
    theme: theme,
    density: DsDensityMode.touch,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  return MaterialApp(
    locale: locale,
    theme: buildDsTheme(ds).copyWith(platform: platform),
    debugShowCheckedModeBanner: false,
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    builder: (context, app) => MediaQuery(
      data: MediaQuery.of(context).copyWith(
        textScaler: TextScaler.linear(textScale),
        disableAnimations: true,
      ),
      child: (wrap ?? _same)(DsThemeScope(dsContext: ds, child: app!)),
    ),
    home: Scaffold(body: child),
  );
}

Widget _same(Widget child) => child;

/// Registers the accessibility tests of one phone screen (brief M5-10 Design note 2).
///
/// Five tests, named for the checklist `docs/quality/accessibility-mobile.md`:
///
/// - `<screen> on the phone: roles, labels and states for screen readers`: no finding of
///   [dsSemanticsProblems] in light and English, the semantics snapshot (every node a screen
///   reader stops at, in reading order, with its role, label, value, state and actions) equals
///   the checked-in file `test/a11y/semantics/<snapshot>.txt` (written by
///   `flutter test --update-goldens`), so a lost label or a changed order fails; then
///   [semantics] asserts what the screen itself must say.
/// - `<screen> on the phone: contrast, labelled targets and touch targets in light and dark`:
///   `labeledTapTargetGuideline`, `androidTapTargetGuideline` (48 by 48, also
///   `density.target-min` of `touch`), `iOSTapTargetGuideline` (44 by 44) and, on macOS, the
///   text contrast (D-M4-07-10), in both themes.
/// - `<screen> on the phone at text scale 2 in de and en: no overflow, every control reachable`:
///   an overflow fails the test, and every finder of [reachable] still finds a
///   widget a finger can hit (WCAG 1.4.4).
/// - `<screen> on the phone with reduced motion: no animation runs`: after the first settled
///   frame no ticker is active (01 8 "Motion").
/// - `<screen> on the phone: the semantics tree speaks the language of the app in de and en`:
///   the application locale is the app's, and no node carries a locale of its own in another
///   language (01 8 "Language", 01 9).
///
/// [build] creates the screen; [open] runs after the first settled frame; [window] is the
/// logical size of the phone; [platform] the system whose controls are built; [snapshot]
/// names the snapshot file (default: [screen] with every run of other characters than letters
/// and digits as `_`).
///
/// [wrap] is passed to [dsPhoneA11yHost].
///
/// ```dart
/// dsPhoneAccessibilityTests(
///   'M1 Spaces',
///   build: spacesScene,
///   reachable: (l) => [find.byKey(const ValueKey('sidebar.jumpTo'))],
/// );
/// ```
void dsPhoneAccessibilityTests(
  String screen, {
  required Widget Function() build,
  Future<void> Function(WidgetTester tester)? open,
  FutureOr<void> Function(WidgetTester tester, AppLocalizations l)? semantics,
  List<Finder> Function(AppLocalizations l)? reachable,
  Size window = dsPhoneA11yWindow,
  TargetPlatform platform = TargetPlatform.iOS,
  String? snapshot,
  Widget Function(Widget child)? wrap,
}) {
  Future<void> pump(
    WidgetTester tester, {
    Locale locale = const Locale('en'),
    double textScale = 1,
    DsThemeMode theme = DsThemeMode.light,
  }) async {
    tester.view
      ..physicalSize = window * 3
      ..devicePixelRatio = 3;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      dsPhoneA11yHost(
        build(),
        locale: locale,
        textScale: textScale,
        theme: theme,
        platform: platform,
        wrap: wrap,
      ),
    );
    await dsPumpIcons(tester, timeout: const Duration(minutes: 3));
    if (open != null) {
      await open(tester);
      await tester.pumpAndSettle();
    }
  }

  testWidgets(
    '$screen on the phone: roles, labels and states for screen readers',
    (tester) async {
      final handle = tester.ensureSemantics();
      await pump(tester);
      final problems = dsSemanticsProblems(tester);
      expect(problems, isEmpty, reason: problems.join('\n'));
      await dsExpectSemanticsSnapshot(
        tester,
        snapshot ??
            screen.replaceAll(RegExp('[^A-Za-z0-9]+'), '_').toLowerCase(),
      );
      await semantics?.call(tester, lookupAppLocalizations(const Locale('en')));
      handle.dispose();
    },
  );

  testWidgets(
    '$screen on the phone: contrast, labelled targets and touch targets in light and dark',
    (tester) async {
      final handle = tester.ensureSemantics();
      for (final theme in DsThemeMode.values) {
        await pump(tester, theme: theme);
        await dsExpectAccessible(tester, targets: false);
        await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
        await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
      }
      handle.dispose();
    },
  );

  testWidgets(
    '$screen on the phone at text scale 2 in de and en: no overflow, every control reachable',
    (tester) async {
      for (final locale in const [Locale('de'), Locale('en')]) {
        await pump(tester, locale: locale, textScale: 2);
        final error = tester.takeException();
        expect(
          error,
          isNull,
          reason: error is FlutterError ? error.toStringDeep() : '$error',
        );
        for (final finder
            in reachable?.call(lookupAppLocalizations(locale)) ??
                const <Finder>[]) {
          await _scrollTo(tester, finder);
          expect(finder, findsWidgets, reason: '$finder in $locale');
          await tester.ensureVisible(finder.first);
          await tester.pumpAndSettle();
          expect(
            finder.first.hitTestable(),
            findsOneWidget,
            reason: '$finder in $locale',
          );
        }
      }
    },
  );

  testWidgets('$screen on the phone with reduced motion: no animation runs', (
    tester,
  ) async {
    await pump(tester);
    expect(
      tester.hasRunningAnimations,
      isFalse,
      reason: 'a ticker still runs with reduced motion',
    );
  });

  testWidgets(
    '$screen on the phone: the semantics tree speaks the language of the app in de and en',
    (tester) async {
      final handle = tester.ensureSemantics();
      for (final language in const ['de', 'en']) {
        await pump(tester, locale: Locale(language));
        expect(
          tester.platformDispatcher.applicationLocale?.languageCode,
          language,
        );
        final foreign = <String>[];
        _walk(tester, (node) {
          final locale = node.getSemanticsData().locale;
          if (locale != null && locale.languageCode != language) {
            foreign.add('${node.id} $locale "${node.label}"');
          }
        });
        expect(foreign, isEmpty, reason: foreign.join('\n'));
      }
      handle.dispose();
    },
  );
}

/// Scrolls the vertical scrollables of the screen until [finder] is built: a list builds only
/// the items near the screen, and at text scale 2 fewer of them fit.
Future<void> _scrollTo(WidgetTester tester, Finder finder) async {
  if (finder.evaluate().isNotEmpty) return;
  final scrollables = find.byWidgetPredicate(
    (w) => w is Scrollable && w.axisDirection == AxisDirection.down,
  );
  for (final scrollable in scrollables.evaluate().toList()) {
    try {
      await tester.scrollUntilVisible(
        finder,
        100,
        scrollable: find.byElementPredicate((e) => e == scrollable),
        maxScrolls: 200,
      );
      return;
    } on StateError {
      // Not in this scrollable: the next one.
    }
  }
}

void _walk(WidgetTester tester, void Function(SemanticsNode node) visit) {
  void go(SemanticsNode node) {
    visit(node);
    for (final child in node.debugListChildrenInOrder(
      DebugSemanticsDumpOrder.traversalOrder,
    )) {
      go(child);
    }
  }

  for (final view in tester.binding.renderViews) {
    final root = view.owner?.semanticsOwner?.rootSemanticsNode;
    if (root != null) go(root);
  }
}

/// The actions a snapshot names: what a person does with a node. Scrolling, focus and
/// show-on-screen depend on the layout, not on the screen's meaning, and stay out.
const Set<SemanticsAction> _snapshotActions = {
  SemanticsAction.tap,
  SemanticsAction.longPress,
  SemanticsAction.increase,
  SemanticsAction.decrease,
  SemanticsAction.dismiss,
  SemanticsAction.expand,
  SemanticsAction.collapse,
};

/// The nodes a screen reader stops at in reading order, one line each: role, label, value,
/// hint, state and actions (brief M5-10 Design note 2).
///
/// A node merged into its parent, an invisible or hidden node and a node that says nothing
/// (no text, no action, no role) is left out; a text's line breaks are written as `\n`.
List<String> dsSemanticsSnapshot(WidgetTester tester) {
  final lines = <String>[];
  _walk(tester, (node) {
    if (node.isMergedIntoParent || node.isInvisible) return;
    final data = node.getSemanticsData();
    final flags = data.flagsCollection;
    if (flags.isHidden) return;
    final parts = <String>[
      if (flags.isButton) 'button',
      if (flags.isLink) 'link',
      if (flags.isTextField) 'text field',
      if (flags.isHeader) 'header',
      if (flags.isImage) 'image',
      if (flags.isSlider) 'slider',
      if (flags.isLiveRegion) 'live region',
      if (flags.isToggled != Tristate.none)
        flags.isToggled == Tristate.isTrue ? 'switch on' : 'switch off',
      if (flags.isChecked != CheckedState.none)
        'checked ${flags.isChecked.name}',
      if (flags.isSelected == Tristate.isTrue) 'selected',
      if (flags.isExpanded != Tristate.none)
        flags.isExpanded == Tristate.isTrue ? 'expanded' : 'collapsed',
      if (flags.isEnabled == Tristate.isFalse) 'disabled',
      if (data.label.isNotEmpty) 'label "${_escape(data.label)}"',
      if (data.value.isNotEmpty) 'value "${_escape(data.value)}"',
      if (data.hint.isNotEmpty) 'hint "${_escape(data.hint)}"',
      if (data.tooltip.isNotEmpty) 'tooltip "${_escape(data.tooltip)}"',
      for (final action in _snapshotActions)
        if (data.hasAction(action)) 'action ${action.name}',
    ];
    if (parts.isNotEmpty) lines.add(parts.join(', '));
  });
  return lines;
}

String _escape(String text) => text.replaceAll('\n', r'\n');

/// Expects [dsSemanticsSnapshot] to equal the file `test/a11y/semantics/<name>.txt` of the
/// package under test; with `flutter test --update-goldens` ([autoUpdateGoldenFiles]) the file
/// is written instead.
Future<void> dsExpectSemanticsSnapshot(WidgetTester tester, String name) async {
  final file = File('test/a11y/semantics/$name.txt');
  final actual = '${dsSemanticsSnapshot(tester).join('\n')}\n';
  if (autoUpdateGoldenFiles) {
    file
      ..parent.createSync(recursive: true)
      ..writeAsStringSync(actual);
    return;
  }
  expect(
    file.existsSync(),
    isTrue,
    reason:
        '${file.path} is missing: run flutter test --update-goldens and check it in',
  );
  expect(
    actual,
    file.readAsStringSync(),
    reason: 'the semantics of $name changed: ${file.path}',
  );
}
