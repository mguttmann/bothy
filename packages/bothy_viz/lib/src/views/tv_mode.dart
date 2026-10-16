// SPDX-License-Identifier: Apache-2.0

/// TV mode (docs/spec/25-visualisierung.md 6.6; screen D13; brief M6-12, design note 4).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../marks/viz_texts.dart';
import '../sources/live_frame_source.dart';
import '../sources/stats_query_source.dart';
import '../sources/viz_clock.dart';
import 'view_scaffold.dart';
import 'view_texts.dart';
import 'viz_host_options.dart';
import 'viz_view_kind.dart';

/// The text scale step of TV mode: from the body size to the next larger one of the type
/// tokens (`type.body-sm` to `type.body`), so the font is one step larger without a literal
/// factor (components.md 11).
final double tvTextScaleStep = DsType.body.fontSize! / DsType.bodySm.fontSize!;

/// Full screen, dark, density `comfortable` with the font one step larger, no controls:
/// rotates through [views] every `viz.tv_rotation_interval` and ends on any key press or
/// pointer down through [onExit].
///
/// A route-less overlay: the host places it over its screen (desktop app and console only).
/// Costs appear only with [VizHostOptions.showCostsInTv] (`viz.tv_show_costs`). The switch
/// between views uses `motion.*` and has no transition in the reduced context. It runs in the
/// session of the person and shows only what the sources deliver to them (25 6.6).
///
/// Constructing it with [VizHostOptions.allowTvMode] `false` or on mobile fails an assertion
/// in debug builds; the registry never offers it there ([vizViewsFor]).
class TvMode extends StatefulWidget {
  /// Creates TV mode over [views] (at least one, never [VizViewKind.tvMode]).
  TvMode({
    super.key,
    required this.views,
    required this.options,
    required this.live,
    required this.queries,
    required this.onExit,
  }) : assert(
         options.allowTvMode && options.formFactor != VizFormFactor.mobile,
         'TV mode only where the host allows it, never on mobile',
       ),
       assert(
         views.isNotEmpty && !views.contains(VizViewKind.tvMode),
         'TV mode rotates through at least one other view',
       );

  /// The views in the order of the rotation.
  final List<VizViewKind> views;

  /// What the host knows.
  final VizHostOptions options;

  /// The live stream for the live views.
  final LiveFrameSource? live;

  /// The REST queries.
  final StatsQuerySource queries;

  /// Ends TV mode; called once, on the first key press or pointer down.
  final VoidCallback onExit;

  @override
  State<TvMode> createState() => _TvModeState();
}

class _TvModeState extends State<TvMode> {
  int _index = 0;
  VizTimer? _timer;
  bool _ended = false;
  final FocusNode _focus = FocusNode(debugLabel: 'tv-mode');

  @override
  void initState() {
    super.initState();
    _schedule();
  }

  void _schedule() {
    _timer = widget.options.clock.schedule(
      widget.options.config.tvRotationInterval,
      () {
        if (!mounted) return;
        setState(() => _index = (_index + 1) % widget.views.length);
        _schedule();
      },
    );
  }

  void _exit() {
    if (_ended) return;
    _ended = true;
    _timer?.cancel();
    widget.onExit();
  }

  @override
  void dispose() {
    _timer?.cancel();
    _focus.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final reduced = context.dsMotion.durationMoveSlow == Duration.zero;
    final scale = MediaQuery.textScalerOf(context);
    return Focus(
      focusNode: _focus,
      autofocus: true,
      onKeyEvent: (_, event) {
        if (event is KeyDownEvent) _exit();
        return KeyEventResult.handled;
      },
      child: Listener(
        behavior: HitTestBehavior.opaque,
        onPointerDown: (_) => _exit(),
        child: DsThemeScope(
          dsContext: DsContext(
            theme: DsThemeMode.dark,
            density: DsDensityMode.comfortable,
            motion: reduced ? DsMotionMode.reduced : DsMotionMode.full,
            accent: dsDefaultAccent,
          ),
          child: MediaQuery(
            data: MediaQuery.of(context).copyWith(
              textScaler: TextScaler.linear(scale.scale(1) * tvTextScaleStep),
            ),
            child: Builder(builder: _content),
          ),
        ),
      ),
    );
  }

  Widget _content(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    final kind = widget.views[_index];
    final title = vizViewSpec(kind).title(texts.l10n);
    return ColoredBox(
      color: colors.surfaceCanvas,
      child: SizedBox.expand(
        child: Padding(
          padding: const EdgeInsets.all(DsSpace.s24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                texts.l10n.vizTvPosition(
                  title,
                  _index + 1,
                  widget.views.length,
                ),
                style: dsTextStyle(DsType.label)
                    .copyWith(color: colors.textSecondary),
              ),
              Text(
                vizClockTime(
                  texts,
                  widget.options.clock.now(),
                  widget.options.timeZone,
                ),
                style: dsTextStyle(DsType.label)
                    .copyWith(color: colors.textSecondary),
              ),
              const SizedBox(height: DsSpace.s16),
              Expanded(
                child: AnimatedSwitcher(
                  duration: context.dsMotion.durationMoveSlow,
                  child: KeyedSubtree(
                    key: ValueKey(_index),
                    child: VizTvScope(
                      showCosts: widget.options.showCostsInTv,
                      child: SingleChildScrollView(
                        child: buildVizView(
                          kind,
                          live: widget.live,
                          queries: widget.queries,
                          options: widget.options,
                          onOpenThread: _ignoreThread,
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  // A click ends TV mode before a card could open its thread.
  static void _ignoreThread(String threadId) {}
}
