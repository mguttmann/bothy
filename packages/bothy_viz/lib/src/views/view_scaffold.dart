// SPDX-License-Identifier: Apache-2.0

/// The shared frame of the six views: header with the switch "As table", the notes (stale,
/// more runs than shown, errors, "Only for the owner") and the body (brief M6-12, design
/// notes 1 and 3).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../marks/viz_texts.dart';
import '../sources/live_frame_source.dart';
import 'table/viz_table.dart';
import 'viz_host_options.dart';

/// Marks the views below it as shown in TV mode: no controls, costs only with
/// [showCosts] (25 6.6).
class VizTvScope extends InheritedWidget {
  /// Creates the scope.
  const VizTvScope({super.key, required this.showCosts, required super.child});

  /// TV mode shows costs (`viz.tv_show_costs`).
  final bool showCosts;

  /// The scope above [context], or `null` outside TV mode.
  static VizTvScope? maybeOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<VizTvScope>();

  /// Whether the views below [context] may show costs: always outside TV mode, in TV mode
  /// only with `viz.tv_show_costs`.
  static bool costsVisible(BuildContext context) =>
      maybeOf(context)?.showCosts ?? true;

  /// Whether the views below [context] show their controls (never in TV mode).
  static bool controlsVisible(BuildContext context) => maybeOf(context) == null;

  @override
  bool updateShouldNotify(VizTvScope oldWidget) =>
      oldWidget.showCosts != showCosts;
}

/// What a view shows below its header.
sealed class VizViewStatus {
  const VizViewStatus();
}

/// The data is loading.
final class VizLoading extends VizViewStatus {
  /// Creates the status.
  const VizLoading();
}

/// The data is there.
final class VizReady extends VizViewStatus {
  /// Creates the status.
  const VizReady();
}

/// The source failed with the problem [code]; the view stopped polling (design note 1).
final class VizFailed extends VizViewStatus {
  /// Creates the status.
  const VizFailed(this.code);

  /// Wire code, for example `forbidden`.
  final String code;
}

/// An instance-only view in a space scope: "Only for the owner", no source was called.
final class VizOwnerOnly extends VizViewStatus {
  /// Creates the status.
  const VizOwnerOnly();
}

/// The wire code of an error a source threw: the code of an [api.ApiProblem], else
/// `internal`.
String vizProblemCode(Object error) =>
    error is api.ApiProblem ? error.code : 'internal';

/// The frame of a view: title, notes, the switch "As table", the link "Open in Grafana" and
/// either the chart or the tables with the same values.
///
/// In TV mode ([VizTvScope]) the switch, the link and [controls] are left out.
class VizViewScaffold extends StatefulWidget {
  /// Creates the frame.
  const VizViewScaffold({
    super.key,
    required this.title,
    required this.options,
    required this.status,
    required this.chart,
    required this.tables,
    this.subtitle,
    this.notes = const [],
    this.controls,
  });

  /// A line under the title in both modes, for example the time of the frame.
  final String? subtitle;

  /// Title of the view.
  final String title;

  /// The host options (Grafana link).
  final VizHostOptions options;

  /// Loading, ready, failed or only for the owner.
  final VizViewStatus status;

  /// Notes under the title, for example the stale marker, each with its tone.
  final List<({DsTone tone, String text})> notes;

  /// The chart of a ready view.
  final WidgetBuilder chart;

  /// The tables of a ready view, with the same values as [chart].
  final List<VizTableData> Function(VizTexts texts) tables;

  /// Controls above the body (timeline playback); hidden in TV mode.
  final Widget? controls;

  @override
  State<VizViewScaffold> createState() => _VizViewScaffoldState();
}

class _VizViewScaffoldState extends State<VizViewScaffold> {
  bool _asTable = false;

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final l10n = texts.l10n;
    final colors = context.dsColors;
    final controls = VizTvScope.controlsVisible(context);
    final status = widget.status;
    final grafana = widget.options.onOpenGrafana;
    final body = Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Wrap(
          spacing: DsSpace.s16,
          runSpacing: DsSpace.s8,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            Semantics(
              header: true,
              child: Text(
                widget.title,
                style: dsTextStyle(DsType.title)
                    .copyWith(color: colors.textPrimary),
              ),
            ),
            if (controls && status is VizReady)
              VizToggleButton(
                key: const ValueKey('viz-as-table'),
                label: l10n.vizAsTable,
                selected: _asTable,
                onPressed: () => setState(() => _asTable = !_asTable),
              ),
            if (controls && grafana != null)
              TextButton(
                key: const ValueKey('viz-open-grafana'),
                style: vizButtonStyle(context),
                onPressed: grafana,
                child: Text(l10n.vizOpenGrafana),
              ),
          ],
        ),
        if (widget.subtitle != null)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s4),
            child: Text(
              widget.subtitle!,
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: colors.textSecondary),
            ),
          ),
        for (final note in widget.notes)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s8),
            child: DsBanner(tone: note.tone, text: note.text),
          ),
        const SizedBox(height: DsSpace.s12),
        switch (status) {
          VizLoading() => Semantics(
            label: l10n.vizLoading,
            child: const SizedBox(height: DsSpace.s48),
          ),
          VizFailed(:final code) => DsBanner(
            tone: DsTone.danger,
            text: problemText(l10n, code),
          ),
          VizOwnerOnly() => DsBanner(
            tone: DsTone.info,
            text: l10n.vizOnlyForOwner,
          ),
          VizReady() => Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              if (controls && widget.controls != null) ...[
                widget.controls!,
                const SizedBox(height: DsSpace.s12),
              ],
              if (_asTable)
                VizTables(tables: widget.tables(texts))
              else
                Builder(builder: widget.chart),
            ],
          ),
        },
      ],
    );
    // Reduced motion: no ink splashes either (25 7.4).
    if (!vizReducedMotion(context)) return body;
    return Theme(
      data: Theme.of(context).copyWith(splashFactory: NoSplash.splashFactory),
      child: body,
    );
  }
}

/// Whether the motion context below [context] is `reduced`.
bool vizReducedMotion(BuildContext context) =>
    context.dsMotion.durationMove == Duration.zero;

/// The style of the buttons of the views: a tap target of at least 48 pixels in every
/// density (the tap target guidelines), and no animation or splash in the reduced context.
ButtonStyle vizButtonStyle(BuildContext context) {
  final reduced = vizReducedMotion(context);
  return ButtonStyle(
    visualDensity: VisualDensity.standard,
    tapTargetSize: MaterialTapTargetSize.padded,
    animationDuration: reduced ? Duration.zero : context.dsMotion.durationFade,
    splashFactory: reduced ? NoSplash.splashFactory : null,
  );
}

/// A button that is on or off, with its state as text style and as toggled semantics; it
/// changes without animation in the reduced context.
class VizToggleButton extends StatelessWidget {
  /// Creates the button.
  const VizToggleButton({
    super.key,
    required this.label,
    required this.selected,
    required this.onPressed,
  });

  /// The text.
  final String label;

  /// On or off.
  final bool selected;

  /// Called on a press.
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final style = vizButtonStyle(context).copyWith(
      backgroundColor: WidgetStatePropertyAll(
        selected ? colors.surfaceSunken : Colors.transparent,
      ),
      side: WidgetStatePropertyAll(
        BorderSide(
          color: selected ? colors.borderControl : colors.borderSubtle,
          width: selected ? DsBorderWidth.focus : DsBorderWidth.hairline,
        ),
      ),
    );
    return Semantics(
      toggled: selected,
      child: OutlinedButton(
        style: style,
        onPressed: onPressed,
        child: Text(
          label,
          style: dsTextStyle(DsType.label).copyWith(color: colors.textPrimary),
        ),
      ),
    );
  }
}

/// The live part of a view: subscribes to a [LiveFrameSource] for the scope, keeps the last
/// frame, the stale state and the first error, and stops the source on an error until the
/// host restarts it (design note 1).
mixin VizLiveSubscription<T extends StatefulWidget> on State<T> {
  StreamSubscription<LiveFrameEvent>? _subscription;
  LiveFrameSource? _source;

  /// The last frame; `null` before the first.
  LiveFrameFrame? liveFrame;

  /// No frame arrived for `viz.stale_after`.
  bool liveStale = false;

  /// The error code of the stream; the subscription ended with it.
  String? liveError;

  /// Called after every change of [liveFrame], [liveStale] or [liveError] inside `setState`.
  void onLiveChanged() {}

  /// Starts [source] for [scope]; `null` does nothing.
  void startLive(LiveFrameSource? source, VizScope scope) {
    _source = source;
    if (source == null) return;
    late final StreamSubscription<LiveFrameEvent> subscription;
    subscription = source
        .start(spaceId: scope.spaceId)
        .listen(
          _onEvent,
          // A newer start of the same source (the next view of TV mode or a tab switch, which
          // Flutter builds before it disposes this one) closed this stream: the source belongs
          // to the newer view now, so this view must not stop it on dispose.
          onDone: () {
            if (!identical(_subscription, subscription)) return;
            _subscription = null;
            _source = null;
          },
        );
    _subscription = subscription;
  }

  void _onEvent(LiveFrameEvent event) {
    if (!mounted) return;
    setState(() {
      switch (event) {
        case LiveFrameFrame():
          liveFrame = event;
          liveStale = false;
        case LiveFrameStale():
          liveStale = true;
        case LiveFrameError(:final code):
          liveError = code;
          unawaited(stopLive());
      }
      onLiveChanged();
    });
  }

  /// Ends the subscription and stops the source.
  Future<void> stopLive() async {
    final subscription = _subscription;
    final source = _source;
    _subscription = null;
    _source = null;
    // Stop the source first: its stop runs synchronously up to its first await, so the
    // stream ends in the same event that reported the error.
    final stopped = source?.stop();
    await subscription?.cancel();
    await stopped;
  }

  @override
  void dispose() {
    unawaited(stopLive());
    super.dispose();
  }
}

/// The notes of a live view: stale and "More runs than shown".
List<({DsTone tone, String text})> vizLiveNotes(
  AppLocalizations l10n, {
  required bool stale,
  bool truncated = false,
}) => [
  if (stale) (tone: DsTone.warning, text: l10n.vizStale),
  if (truncated) (tone: DsTone.info, text: l10n.vizMoreRunsThanShown),
];
