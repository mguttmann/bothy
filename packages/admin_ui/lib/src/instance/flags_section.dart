// SPDX-License-Identifier: Apache-2.0

/// The feature flags of the instance: key, description, the state evaluated for the signed-in
/// person and the stored value, and a switch per flag with a required reason (configuration
/// reference 7; 16 6.2; brief M6-09 Design note 4; Ruling D-M6-09-4).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/instance_repository.dart';
import '../self/self_feedback.dart';
import 'instance_common.dart';

/// The feature flags ([InstanceRepository.featureFlags]) with the evaluated state of the caller
/// (`getInstance`).
///
/// The switch of a flag opens [FlagSwitchDialog]: it states the new global value, takes the
/// required reason and sends `putFeatureFlag` with `If-Match` (step-up, 05
/// `feature_flag.update`); the rollout stays. A `412` shows the newer state of the flag and
/// nothing is changed, so the person can switch again with what is now stored. An orphaned
/// flag (stored, no longer registered) is shown off and cannot be switched. [changes] (the
/// desktop's `instance.updated`) loads the list again; the console reloads after its own
/// change, as it has no live updates.
///
/// Keys: `flags.list`, `flags.conflict`, `flag.<key>`, `flag.<key>.evaluated`,
/// `flag.<key>.stored`, `flag.<key>.meta`, `flag.<key>.switch`.
class FlagsSection extends StatefulWidget {
  /// Creates the section.
  const FlagsSection({
    super.key,
    required this.repository,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.changes,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Opens the events after which the list loads again (a new stream per call), or null.
  final Stream<void> Function()? changes;

  /// Runs after a saved change.
  final VoidCallback? onChanged;

  @override
  State<FlagsSection> createState() => _FlagsSectionState();
}

class _FlagsSectionState extends State<FlagsSection> {
  List<FeatureFlagEntry>? _flags;
  Map<String, bool> _evaluated = const {};
  Map<String, String> _names = const {};
  Object? _error;
  SelfNotice? _notice;
  String? _conflict;
  StreamSubscription<void>? _changes;

  @override
  void initState() {
    super.initState();
    _changes = widget.changes?.call().listen((_) => _load());
    _load();
  }

  @override
  void dispose() {
    _changes?.cancel();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final flags = await widget.repository.featureFlags();
      final overview = await widget.repository.instanceOverview();
      final names = await peopleNames(
        widget.repository,
        flags.map((f) => f.updatedBy),
      );
      if (!mounted) return;
      setState(() {
        _flags = flags;
        _evaluated = overview.features;
        _names = names;
        _error = null;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _error = e);
    }
  }

  Future<void> _switch(FeatureFlagEntry flag) async {
    final l = AppLocalizations.of(context);
    setState(() {
      _conflict = null;
      _notice = null;
    });
    final result = await FlagSwitchDialog.show(
      context,
      repository: widget.repository,
      flag: flag,
    );
    if (result == null || !mounted) return;
    switch (result) {
      case FlagSwitched(:final flag):
        setState(
          () => _notice = SelfNotice(
            flag.enabled
                ? l.instanceFlagSwitchedOn(flag.key)
                : l.instanceFlagSwitchedOff(flag.key),
          ),
        );
        widget.onChanged?.call();
      case FlagConflict(:final current):
        setState(
          () => _conflict = l.instanceFlagConflict(
            current.key,
            current.enabled ? l.instanceFlagOn : l.instanceFlagOff,
          ),
        );
    }
    await _load();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final flags = _flags;
    final error = _error;
    return Column(
      key: const ValueKey('flags.list'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          l.instanceFlagsIntro,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textSecondary),
        ),
        const SizedBox(height: DsSpace.s12),
        if (_notice case final n?) selfNoticeBanner(n),
        if (_conflict case final text?)
          Padding(
            padding: const EdgeInsets.only(bottom: DsSpace.s8),
            child: DsBanner(
              key: const ValueKey('flags.conflict'),
              tone: DsTone.warning,
              text: text,
              liveRegion: true,
            ),
          ),
        if (error != null)
          ProblemBanner(problem: loadProblem(error), onRetry: _load)
        else if (flags == null)
          const Center(child: CircularProgressIndicator())
        else
          for (final f in flags) _row(context, f),
      ],
    );
  }

  Widget _row(BuildContext context, FeatureFlagEntry f) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final evaluated = _evaluated[f.key];
    final rollout = f.rolloutUserIds.length + f.rolloutGroupIds.length;
    return Padding(
      key: ValueKey('flag.${f.key}'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Wrap(
        spacing: DsSpace.s12,
        runSpacing: DsSpace.s8,
        alignment: WrapAlignment.spaceBetween,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          ConstrainedBox(
            constraints: const BoxConstraints(minWidth: 280, maxWidth: 560),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  f.key,
                  style: dsTextStyle(DsType.code)
                      .copyWith(color: colors.textPrimary),
                ),
                if (f.description case final d?)
                  Text(
                    d,
                    style: dsTextStyle(DsType.body)
                        .copyWith(color: colors.textPrimary),
                  ),
                Text(
                  [
                    switch (f.kind) {
                      FeatureFlagKind.release => l.instanceFlagKindRelease,
                      FeatureFlagKind.option => l.instanceFlagKindOption,
                      FeatureFlagKind.unknown => l.instanceFlagKindUnknown,
                    },
                    ?f.module,
                    if (f.orphaned) l.instanceFlagOrphaned,
                  ].join(' · '),
                  style: secondary,
                ),
                Text(
                  evaluated == null
                      ? l.instanceFlagForYouUnknown
                      : l.instanceFlagForYou(
                          evaluated ? l.instanceFlagOn : l.instanceFlagOff,
                        ),
                  key: ValueKey('flag.${f.key}.evaluated'),
                  style: secondary,
                ),
                Text(
                  [
                    l.instanceFlagStored(
                      f.enabled ? l.instanceFlagOn : l.instanceFlagOff,
                    ),
                    if (f.defaultValue case final d?)
                      l.instanceFlagDefault(
                        d ? l.instanceFlagOn : l.instanceFlagOff,
                      ),
                    if (rollout > 0) l.instanceFlagRollout(rollout),
                  ].join(' · '),
                  key: ValueKey('flag.${f.key}.stored'),
                  style: secondary,
                ),
                Text(
                  changedText(
                    context,
                    version: f.version,
                    byName: f.updatedBy == null ? null : _names[f.updatedBy],
                    at: f.updatedAt,
                    now: widget.clock(),
                    timeZone: widget.timeZone,
                  ),
                  key: ValueKey('flag.${f.key}.meta'),
                  style: secondary,
                ),
              ],
            ),
          ),
          Semantics(
            label: f.key,
            child: Switch(
              key: ValueKey('flag.${f.key}.switch'),
              value: f.enabled,
              onChanged: f.orphaned ? null : (_) => _switch(f),
            ),
          ),
        ],
      ),
    );
  }
}

/// The outcome of [FlagSwitchDialog].
sealed class FlagSwitchResult {
  const FlagSwitchResult();
}

/// The flag was set; [flag] is the stored state.
final class FlagSwitched extends FlagSwitchResult {
  /// Creates the result.
  const FlagSwitched(this.flag);

  /// The stored flag.
  final FeatureFlagEntry flag;
}

/// Another owner changed the flag meanwhile; nothing was set and [current] is the newer state.
final class FlagConflict extends FlagSwitchResult {
  /// Creates the result.
  const FlagConflict(this.current);

  /// The newer flag.
  final FeatureFlagEntry current;
}

/// The switch of one flag: states the new global value for everyone (the rollout stays), takes
/// the required reason and sends `putFeatureFlag`.
///
/// Keys: `flag.dialog`, `flag.body`, `flag.reason`, `flag.error`, `flag.submit`, `flag.cancel`.
class FlagSwitchDialog extends StatefulWidget {
  /// Creates the dialog for [flag].
  const FlagSwitchDialog({
    super.key,
    required this.repository,
    required this.flag,
  });

  /// The administration's calls.
  final InstanceRepository repository;

  /// The flag as the list showed it.
  final FeatureFlagEntry flag;

  /// Shows the dialog; returns the outcome, or null when it was cancelled or failed.
  static Future<FlagSwitchResult?> show(
    BuildContext context, {
    required InstanceRepository repository,
    required FeatureFlagEntry flag,
  }) => showDialog<FlagSwitchResult>(
    context: context,
    builder: (_) => FlagSwitchDialog(repository: repository, flag: flag),
  );

  @override
  State<FlagSwitchDialog> createState() => _FlagSwitchDialogState();
}

class _FlagSwitchDialogState extends State<FlagSwitchDialog> {
  final TextEditingController _reason = TextEditingController();
  String? _reasonError;
  String? _failure;
  bool _busy = false;

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final l = AppLocalizations.of(context);
    final on = !widget.flag.enabled;
    setState(() {
      _busy = true;
      _reasonError = null;
      _failure = null;
    });
    try {
      final saved = await widget.repository.putFeatureFlag(
        widget.flag,
        enabled: on,
        reason: _reason.text.trim(),
        purpose: on
            ? l.stepUpPurposeFlagOn(widget.flag.key)
            : l.stepUpPurposeFlagOff(widget.flag.key),
      );
      if (mounted) Navigator.of(context).pop(FlagSwitched(saved));
    } on InstanceVersionConflict<FeatureFlagEntry> catch (c) {
      if (mounted) Navigator.of(context).pop(FlagConflict(c.current));
    } on Object catch (e) {
      if (!mounted) return;
      final field = fieldErrorAt(l, fieldErrorsOf(e), '/reason');
      final failure = field == null ? selfFailure(l, e)?.text : null;
      setState(() {
        _reasonError = field;
        _failure = failure;
      });
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final on = !widget.flag.enabled;
    final rollout =
        widget.flag.rolloutUserIds.length + widget.flag.rolloutGroupIds.length;
    return AlertDialog(
      key: const ValueKey('flag.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        on
            ? l.instanceFlagOnTitle(widget.flag.key)
            : l.instanceFlagOffTitle(widget.flag.key),
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                [
                  l.instanceChangeFromTo(
                    widget.flag.enabled ? l.instanceFlagOn : l.instanceFlagOff,
                    on ? l.instanceFlagOn : l.instanceFlagOff,
                  ),
                  l.instanceFlagEveryone,
                  if (rollout > 0) l.instanceFlagRolloutStays(rollout),
                ].join('\n'),
                key: const ValueKey('flag.body'),
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.textSecondary),
              ),
              if (_failure case final f?)
                Padding(
                  padding: const EdgeInsets.only(top: DsSpace.s12),
                  child: DsBanner(
                    key: const ValueKey('flag.error'),
                    tone: DsTone.danger,
                    text: f,
                    liveRegion: true,
                  ),
                ),
              const SizedBox(height: DsSpace.s16),
              TextField(
                key: const ValueKey('flag.reason'),
                controller: _reason,
                enabled: !_busy,
                decoration: InputDecoration(
                  labelText: l.instanceFlagReason,
                  errorText: _reasonError,
                ),
                onChanged: (_) => setState(() {}),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('flag.cancel'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: const ValueKey('flag.submit'),
          onPressed: _busy || _reason.text.trim().isEmpty ? null : _submit,
          child: Text(on ? l.instanceFlagSwitchOn : l.instanceFlagSwitchOff),
        ),
      ],
    );
  }
}
