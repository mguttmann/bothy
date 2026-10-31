// SPDX-License-Identifier: Apache-2.0

/// Building blocks the core components share: Unicode isolation, the tappable surface with
/// the focus ring, and the translations of state codes.
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_focus_ring.dart';
import 'view_models.dart';

/// U+2068 FIRST STRONG ISOLATE.
const String _fsi = '\u2068';

/// U+2069 POP DIRECTIONAL ISOLATE.
const String _pdi = '\u2069';

/// [name] wrapped in a first-strong isolate (U+2068 ... U+2069) for display (SEC-IN-4).
///
/// A foreign name (display name, channel name, title, file name) with a right-to-left override
/// can then not reorder the text around it. Only the visual text is isolated; semantics labels
/// use the plain name, because a screen reader would read the control characters.
///
/// ```dart
/// Text(dsIsolate(data.authorName));
/// ```
String dsIsolate(String name) => '$_fsi$name$_pdi';

/// The texts of the ambient locale.
AppLocalizations dsL10n(BuildContext context) => AppLocalizations.of(context);

/// The formatter of the ambient locale.
L10nFormat dsFormat(BuildContext context) =>
    L10nFormat(Localizations.localeOf(context));

/// The translated text of a run or trigger reason [code], or null for a code this app does
/// not translate (the caller then shows the code in `type.code-sm`).
///
/// Covers the run reasons of components.md 2.1 and the acceptance reasons of 09 3.5. [name]
/// fills `user_cancelled` ("Canceled by Sam"); without it the text names no one.
String? dsReasonText(AppLocalizations l, String? code, {String? name}) {
  return switch (code) {
    'billing_mode_mismatch' => l.liveBoxReasonBillingModeMismatch,
    'access_revoked' => l.liveBoxReasonAccessRevoked,
    'stale' => l.liveBoxReasonStale,
    'interrupted' => l.liveBoxReasonInterrupted,
    'user_cancelled' =>
      name == null
          ? l.liveBoxReasonUserCancelledNoName
          : l.liveBoxReasonUserCancelled(name),
    'multiple_agents_mentioned' => l.triggerReasonMultipleAgentsMentioned,
    'agent_not_allowed_here' => l.triggerReasonAgentNotAllowedHere,
    'no_trigger_right' => l.triggerReasonNoTriggerRight,
    'run_active_other_agent' => l.triggerReasonRunActiveOtherAgent,
    'agent_disabled' => l.triggerReasonAgentDisabled,
    'billing_mode_personal_not_holder' =>
      l.triggerReasonBillingModePersonalNotHolder,
    'voice_unsupported' => l.triggerReasonVoiceUnsupported,
    'run_active' => l.triggerReasonRunActive,
    'operator_not_in_channel' => l.triggerReasonOperatorNotInChannel,
    'adapter_error' => l.liveBoxReasonAdapterError,
    'cancel_timeout' => l.liveBoxReasonCancelTimeout,
    'project_not_ready' => l.liveBoxReasonProjectNotReady,
    _ => null,
  };
}

/// The state word of a run in the live box (components.md section 2.1).
String dsRunStateWord(AppLocalizations l, DsRunData run) {
  return switch (run.state) {
    DsRunState.queued => l.liveBoxStateQueued,
    DsRunState.running => l.liveBoxStateRunning,
    DsRunState.waitingInput =>
      run.waitingForName == null
          ? l.liveBoxStateWaitingForYou
          : l.liveBoxStateWaitingFor(run.waitingForName!),
    DsRunState.completed => l.liveBoxStateCompleted,
    DsRunState.failed => l.liveBoxStateFailed,
    DsRunState.cancelled => l.liveBoxStateCancelled,
    DsRunState.unknown => l.liveBoxStateUnknown(run.state.code),
  };
}

/// The spine and icon color of a run state (`color.run.*`); `running` from `DsAccent`.
Color dsRunStateColor(BuildContext context, DsRunState state) {
  final c = context.dsColors;
  return switch (state) {
    DsRunState.queued => c.runQueued,
    DsRunState.running => context.dsAccent.runRunning,
    DsRunState.waitingInput => c.runWaiting,
    DsRunState.completed => c.runCompleted,
    DsRunState.failed => c.runFailed,
    DsRunState.cancelled => c.runCancelled,
    DsRunState.unknown => c.textSecondary,
  };
}

/// The icon of a run state (`run.*`).
DsGlyph dsRunStateGlyph(DsRunState state) {
  return switch (state) {
    DsRunState.queued => DsIcons.runQueued,
    DsRunState.running => DsIcons.runRunning,
    DsRunState.waitingInput => DsIcons.runWaiting,
    DsRunState.completed => DsIcons.runCompleted,
    DsRunState.failed => DsIcons.runFailed,
    DsRunState.cancelled => DsIcons.runCancelled,
    DsRunState.unknown => DsIcons.stateInfo,
  };
}

/// The colors of a [DsBanner] and other tinted parts.
enum DsTone {
  /// `color.info.*`.
  info,

  /// `color.warning.*`.
  warning,

  /// `color.danger.*`.
  danger,
}

/// Subtle background, border and foreground of [tone].
({Color subtle, Color border, Color fg}) dsToneColors(
  BuildContext context,
  DsTone tone,
) {
  final c = context.dsColors;
  return switch (tone) {
    DsTone.info => (subtle: c.infoSubtle, border: c.infoBorder, fg: c.infoFg),
    DsTone.warning => (
      subtle: c.warningSubtle,
      border: c.warningBorder,
      fg: c.warningFg,
    ),
    DsTone.danger => (
      subtle: c.dangerSubtle,
      border: c.dangerBorder,
      fg: c.dangerFg,
    ),
  };
}

/// The icon of [tone].
DsGlyph dsToneGlyph(DsTone tone) => switch (tone) {
  DsTone.info => DsIcons.stateInfo,
  DsTone.warning => DsIcons.stateWarning,
  DsTone.danger => DsIcons.stateError,
};

/// A tappable surface with the states of components.md "Common rules".
///
/// Rest, hover ([hoverColor], pointer only), pressed (the ink splash), focus (a ring
/// `color.focus.ring` in `border-width.focus` around the child, always visible), disabled
/// ([onTap] null). The hit area is at least `density.target-min` in both directions, also
/// when the child is smaller. Enter and Space activate the focused surface (the default
/// `ActivateIntent` of the app); no other key does.
///
/// [label] is the semantics label of the button; [selected] and [expanded] mark the states of
/// tree and list rows; [onSidebar] draws the ring in `color.focus.ring-on-sidebar`.
class DsTappable extends StatefulWidget {
  /// Creates the surface around [child].
  const DsTappable({
    super.key,
    required this.child,
    required this.onTap,
    this.label,
    this.hint,
    this.hoverColor,
    this.radius = DsRadius.sm,
    this.selected,
    this.expanded,
    this.toggled,
    this.button = true,
    this.onSidebar = false,
    this.focusNode,
    this.autofocus = false,
    this.excludeChildSemantics = false,
  });

  /// The content.
  final Widget child;

  /// Called on tap, Enter or Space; null disables the surface.
  final VoidCallback? onTap;

  /// The semantics label, or null to use the child's semantics.
  final String? label;

  /// The semantics hint, or null.
  final String? hint;

  /// The hover background, or null for `color.surface.hover`.
  final Color? hoverColor;

  /// The corner radius of the hover, splash and ring.
  final double radius;

  /// The selected state for screen readers, or null when the surface is not selectable.
  final bool? selected;

  /// The expanded state of a tree node, or null.
  final bool? expanded;

  /// The on or off state of a toggle button for screen readers, or null when the surface is
  /// no toggle (the "Wrap lines" toggle of a code block, M4-49).
  final bool? toggled;

  /// Whether the surface reads as a button.
  final bool button;

  /// Whether the ring uses `color.focus.ring-on-sidebar`.
  final bool onSidebar;

  /// An optional focus node.
  final FocusNode? focusNode;

  /// Whether the surface takes focus on first build.
  final bool autofocus;

  /// Whether the child's own semantics (its texts and icons) give way to [label]; the tap
  /// and focus of the surface stay for screen readers.
  final bool excludeChildSemantics;

  @override
  State<DsTappable> createState() => _DsTappableState();
}

class _DsTappableState extends State<DsTappable> {
  bool _focused = false;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final ring = widget.onSidebar
        ? colors.focusRingOnSidebar
        : context.dsAccent.focusRing;
    final target = context.dsDensity.targetMin;
    final shape = BorderRadius.circular(widget.radius);
    // Only the child's own semantics go when [label] replaces them: the tap and focus of the
    // InkWell stay, so a screen reader can press the surface (WCAG 4.1.2; brief M4-32).
    Widget content = ConstrainedBox(
      constraints: BoxConstraints(minWidth: target, minHeight: target),
      child: widget.excludeChildSemantics
          ? ExcludeSemantics(child: widget.child)
          : widget.child,
    );
    content = Material(
      type: MaterialType.transparency,
      child: InkWell(
        onTap: widget.onTap,
        focusNode: widget.focusNode,
        autofocus: widget.autofocus,
        borderRadius: shape,
        hoverColor: widget.hoverColor ?? colors.surfaceHover,
        focusColor: Colors.transparent,
        highlightColor: colors.surfaceSelected,
        splashFactory: NoSplash.splashFactory,
        onFocusChange: (focused) => setState(() => _focused = focused),
        child: content,
      ),
    );
    content = DecoratedBox(
      position: DecorationPosition.foreground,
      decoration: BoxDecoration(
        borderRadius: shape,
        border: _focused
            ? Border.all(
                color: ring,
                width: DsBorderWidth.focus,
                strokeAlign: BorderSide.strokeAlignOutside,
              )
            : null,
      ),
      child: content,
    );
    // The ring above is this surface's own; the app-wide ring of DsFocusRing stays away.
    content = DsDrawsFocusRing(child: content);
    return Semantics(
      container: true,
      button: widget.button,
      enabled: widget.onTap != null,
      label: widget.label,
      hint: widget.hint,
      selected: widget.selected,
      expanded: widget.expanded,
      toggled: widget.toggled,
      child: content,
    );
  }
}

/// A small badge with an icon and a text in `type.label` (`size.badge-height`, `radius.pill`).
///
/// Used for origin badges, the backend policy badge and the "Waiting for you" badge; the text
/// always comes with an icon or stands alone, so the badge never says something only by color.
class DsBadge extends StatelessWidget {
  /// Creates the badge.
  const DsBadge({
    super.key,
    required this.text,
    required this.background,
    required this.foreground,
    this.glyph,
    this.semanticsLabel,
  });

  /// The text.
  final String text;

  /// The background color.
  final Color background;

  /// The text and icon color.
  final Color foreground;

  /// An optional icon in front of the text.
  final DsGlyph? glyph;

  /// The screen reader label, or null for [text].
  final String? semanticsLabel;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: semanticsLabel ?? text,
      excludeSemantics: true,
      child: Container(
        constraints: const BoxConstraints(minHeight: DsSize.badgeHeight),
        padding: const EdgeInsets.symmetric(horizontal: DsSpace.s6),
        decoration: BoxDecoration(
          color: background,
          borderRadius: BorderRadius.circular(DsRadius.pill),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (glyph != null) ...[
              DsIcon(
                glyph!,
                size: DsIconSizeRole.sm,
                color: foreground,
                decorative: true,
              ),
              const SizedBox(width: DsSpace.s4),
            ],
            Flexible(
              child: Text(
                text,
                style: dsTextStyle(DsType.label).copyWith(color: foreground),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
