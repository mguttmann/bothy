// SPDX-License-Identifier: Apache-2.0

import 'dart:io' show Platform;
import 'dart:ui' show CheckedState, Tristate;

import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

import '../generated/design_tokens.g.dart' show DsDensity;

/// One finding of [dsSemanticsProblems]: a node of the semantics tree that a screen reader
/// cannot present as 01 8 asks.
class DsSemanticsProblem {
  /// A finding for the node with [nodeId] at [rect]; [rule] names the broken rule and
  /// [description] the node as a screen reader receives it.
  const DsSemanticsProblem({
    required this.rule,
    required this.nodeId,
    required this.rect,
    required this.description,
  });

  /// The broken rule: `label` (an interactive node without a name, WCAG 4.1.2), `role` (a
  /// tappable node without a role such as button or link, WCAG 4.1.2), `decorative` (an
  /// empty image or leaf that reaches the screen reader, 01 8 "decorative icons are excluded"),
  /// `repeated` (a label that says one of its parts twice), `action` (an enabled button that
  /// a screen reader cannot press) or `field` (a text field with children).
  final String rule;

  /// The id of the semantics node.
  final int nodeId;

  /// The node's rectangle in its own coordinates.
  final Rect rect;

  /// The node's flags, actions and texts.
  final String description;

  @override
  String toString() => '$rule: node $nodeId $rect $description';
}

/// Every node of the current semantics tree that breaks the screen reader rules of 01 8.
///
/// The rules, each the class of a gap a screen reader user meets on any screen:
///
/// - `label`: a node with a tap, long press, increase, decrease or custom action, a text field
///   and a control (button, link, slider, switch, check box), enabled or not, has a label, tooltip
///   or hint (WCAG 4.1.2 "Name").
/// - `action`: an enabled button has a tap action, so a screen reader can press it (WCAG 4.1.2
///   "Value", operable by assistive technology); a button whose action was dropped together
///   with its child semantics looks right and does nothing.
/// - `role`: a node with a tap action says what it is: a button, link, text field, check box,
///   switch, radio, selectable item (tab, chip), expandable item, slider, keyboard key or an
///   explicit [SemanticsRole] (WCAG 4.1.2 "Role"); a plain `InkWell` or `GestureDetector`
///   without `Semantics(button: true)` fails here.
/// - `decorative`: an image without a label, or a leaf without label, value, hint, tooltip,
///   action or flag, reaches the screen reader as an empty stop; decorative content is
///   excluded from semantics instead (01 8, components.md).
/// - `field`: a text field is a node of its own without children; a field whose semantics
///   end up in the node of the column around it takes that node's content along, and macOS
///   presents a text field as a leaf, so VoiceOver would never reach the column's buttons.
/// - `repeated`: a part of a label (between commas or line breaks, three characters or more)
///   occurs twice, as when a widget gives its own label and merges the texts it was built from
///   below it; the screen reader would read the name twice.
///
/// Call it after `tester.ensureSemantics()`. Returns an empty list when the tree is clean.
///
/// ```dart
/// final handle = tester.ensureSemantics();
/// await tester.pumpWidget(host(screen));
/// expect(dsSemanticsProblems(tester), isEmpty);
/// handle.dispose();
/// ```
List<DsSemanticsProblem> dsSemanticsProblems(WidgetTester tester) {
  final problems = <DsSemanticsProblem>[];
  for (final view in tester.binding.renderViews) {
    final root = view.owner?.semanticsOwner?.rootSemanticsNode;
    if (root != null) _visit(root, problems);
  }
  return problems;
}

void _visit(SemanticsNode node, List<DsSemanticsProblem> problems) {
  if (!node.isMergedIntoParent && !node.isInvisible) {
    final problem = _check(node);
    if (problem != null) problems.add(problem);
  }
  node.visitChildren((child) {
    _visit(child, problems);
    return true;
  });
}

DsSemanticsProblem? _check(SemanticsNode node) {
  final data = node.getSemanticsData();
  final flags = data.flagsCollection;
  if (flags.isHidden) return null;
  final named =
      data.label.trim().isNotEmpty ||
      data.tooltip.trim().isNotEmpty ||
      data.hint.trim().isNotEmpty;
  final tap = data.hasAction(SemanticsAction.tap);
  final interactive =
      tap ||
      data.hasAction(SemanticsAction.longPress) ||
      data.hasAction(SemanticsAction.increase) ||
      data.hasAction(SemanticsAction.decrease) ||
      // A group with actions of its own ("Move up" of a reorderable list): VoiceOver stops at
      // it as an element.
      (data.customSemanticsActionIds?.isNotEmpty ?? false) ||
      flags.isTextField ||
      // A control is named also while it is disabled: a switch that cannot be used now
      // still says what it switches.
      flags.isButton ||
      flags.isLink ||
      flags.isSlider ||
      flags.isToggled != Tristate.none ||
      flags.isChecked != CheckedState.none;
  String? rule;
  if (flags.isTextField && node.hasChildren) {
    // macOS turns a text field into a leaf: VoiceOver never reaches what lies below it.
    rule = 'field';
  } else if (_repeats(data.label)) {
    rule = 'repeated';
  } else if (interactive && !named) {
    rule = 'label';
  } else if (flags.isButton && flags.isEnabled == Tristate.isTrue && !tap) {
    rule = 'action';
  } else if (tap && !_hasRole(data)) {
    rule = 'role';
  } else if (flags.isImage && !named) {
    rule = 'decorative';
  } else if (!node.hasChildren &&
      !named &&
      data.value.trim().isEmpty &&
      data.actions == 0 &&
      !_hasAnyFlag(data) &&
      !data.rect.isEmpty) {
    rule = 'decorative';
  }
  if (rule == null) return null;
  return DsSemanticsProblem(
    rule: rule,
    nodeId: node.id,
    rect: data.rect,
    description: _describe(data),
  );
}

/// Whether a part of [label] (between commas or line breaks, at least three characters,
/// without bidi isolates) occurs twice: a merged node that reads its own label and then the
/// texts it was made of says everything twice.
bool _repeats(String label) {
  final seen = <String>{};
  for (final part
      in label
          .replaceAll(RegExp('[\u2066-\u2069]'), '')
          .split(RegExp('[\n,]'))) {
    final text = part.trim();
    if (text.length >= 3 && !seen.add(text)) return true;
  }
  return false;
}

bool _hasRole(SemanticsData data) {
  final flags = data.flagsCollection;
  return flags.isButton ||
      flags.isLink ||
      flags.isTextField ||
      flags.isSlider ||
      flags.isKeyboardKey ||
      flags.isInMutuallyExclusiveGroup ||
      flags.isChecked != CheckedState.none ||
      flags.isToggled != Tristate.none ||
      flags.isSelected != Tristate.none ||
      flags.isExpanded != Tristate.none ||
      // The barrier of a dialog: a screen reader offers its dismiss action ("Escape").
      data.hasAction(SemanticsAction.dismiss) ||
      data.role != SemanticsRole.none;
}

bool _hasAnyFlag(SemanticsData data) {
  final flags = data.flagsCollection;
  return _hasRole(data) ||
      flags.isHeader ||
      flags.isLiveRegion ||
      flags.isImage ||
      flags.isFocused != Tristate.none ||
      flags.hasImplicitScrolling;
}

String _describe(SemanticsData data) {
  final flags = data.flagsCollection;
  final parts = <String>[
    if (data.label.isNotEmpty) 'label "${data.label}"',
    if (data.value.isNotEmpty) 'value "${data.value}"',
    if (data.hint.isNotEmpty) 'hint "${data.hint}"',
    if (data.tooltip.isNotEmpty) 'tooltip "${data.tooltip}"',
    for (final action in SemanticsAction.values)
      if (data.hasAction(action)) 'action ${action.name}',
    if (flags.isButton) 'button',
    if (flags.isTextField) 'text field',
    if (flags.isImage) 'image',
    if (flags.isHeader) 'header',
    if (data.role != SemanticsRole.none) 'role ${data.role.name}',
  ];
  return parts.isEmpty ? '(empty)' : parts.join(', ');
}

/// The smallest target of the desktop densities (`density.target-min`, WCAG 2.2 SC 2.5.8
/// "Target Size (Minimum)").
final Size dsDesktopTargetMin = Size.square(DsDensity.comfortable.targetMin);

/// The guideline of WCAG 2.2 SC 2.5.8 for the desktop: every tappable node is at least
/// [dsDesktopTargetMin].
final AccessibilityGuideline dsDesktopTargetGuideline =
    DsDesktopTargetGuideline();

/// WCAG 2.2 SC 2.5.8 "Target Size (Minimum)" on the desktop: a node with a tap action is at
/// least [dsDesktopTargetMin], measured as `MinimumTapTargetGuideline` measures it.
///
/// Read-only text (a transcript or code a person selects with a long press) is text, not a
/// target: SC 2.5.8 excepts inline targets whose size the line of text sets, so read-only text
/// fields are skipped, as links are by the base class.
// This library is test code (lib/testing.dart, imported only from tests); the guidelines of
// flutter_test are meant for it.
// ignore: invalid_use_of_visible_for_testing_member
class DsDesktopTargetGuideline extends MinimumTapTargetGuideline {
  /// The guideline with [dsDesktopTargetMin] and the link to SC 2.5.8.
  DsDesktopTargetGuideline()
    : super(
        size: dsDesktopTargetMin,
        link: 'https://www.w3.org/TR/WCAG22/#target-size-minimum',
      );

  @override
  bool shouldSkipNode(SemanticsNode node) {
    final flags = node.getSemanticsData().flagsCollection;
    return super.shouldSkipNode(node) ||
        (flags.isTextField && flags.isReadOnly);
  }
}

/// WCAG 2.2 SC 1.4.3 "Contrast (Minimum)" as `textContrastGuideline` measures it, without the
/// text of inactive controls, which SC 1.4.3 excepts.
///
/// The base class skips a disabled node by the node's own flags; an entry that merges its
/// descendants (a menu entry, a list tile) carries the disabled state only in its merged data,
/// so its dimmed label was measured as text. This guideline decides by the merged data.
// Test code, as DsDesktopTargetGuideline.
// ignore: invalid_use_of_visible_for_testing_member
class DsTextContrastGuideline extends MinimumTextContrastGuideline {
  /// The guideline.
  const DsTextContrastGuideline();

  @override
  bool shouldSkipNode(SemanticsData data) =>
      super.shouldSkipNode(data) ||
      data.flagsCollection.isEnabled == Tristate.isFalse;
}

/// The text contrast guideline of the desktop accessibility tests ([DsTextContrastGuideline]).
const AccessibilityGuideline dsTextContrastGuideline =
    DsTextContrastGuideline();

/// Expects the screen on the tester to meet the accessibility rules of 01 8 that a widget
/// test can check: no finding of [dsSemanticsProblems], every tap target labelled
/// (`labeledTapTargetGuideline`), every tap target at least [dsDesktopTargetMin]
/// ([dsDesktopTargetGuideline]; pass [targets] false for a phone screen with its own
/// guideline) and, on macOS, the text contrast ([dsTextContrastGuideline]).
///
/// The contrast guideline measures the most frequent colors of the rendered glyphs, which
/// depend on the anti-aliasing of the host; on the Linux runner a blended edge shade outnumbers
/// the text color, so it runs on macOS only and the token gate checks every pair on every host
/// (docs/DECISIONS.md D-M4-07-10). Call it after `tester.ensureSemantics()` and a settled pump.
///
/// ```dart
/// final handle = tester.ensureSemantics();
/// await tester.pumpWidget(host(screen));
/// await tester.pumpAndSettle();
/// await dsExpectAccessible(tester);
/// handle.dispose();
/// ```
Future<void> dsExpectAccessible(
  WidgetTester tester, {
  bool targets = true,
}) async {
  final problems = dsSemanticsProblems(tester);
  expect(problems, isEmpty, reason: problems.join('\n'));
  await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
  if (targets) {
    await expectLater(tester, meetsGuideline(dsDesktopTargetGuideline));
  }
  if (Platform.isMacOS) {
    await expectLater(tester, meetsGuideline(dsTextContrastGuideline));
  }
}
