// SPDX-License-Identifier: Apache-2.0

/// The instance settings of the configuration reference section 6: each key with its type,
/// allowed values, current value, version and who changed it, and a typed editor that sends
/// `putInstanceSetting` with `If-Match` on the version (brief M6-09 Design note 1; Ruling
/// D-M6-09-1).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/instance_repository.dart';
import '../self/self_feedback.dart';
import 'instance_common.dart';
import 'setting_kinds.dart';

/// The name of the kind [kind] with its allowed values.
String settingKindText(AppLocalizations l, SettingKind kind) => switch (kind) {
  SettingKind.text => l.instanceKindText,
  SettingKind.paletteKey => l.instanceKindPalette(accentPaletteKeys.join(', ')),
  SettingKind.uuid => l.instanceKindUuid,
  SettingKind.locale => l.instanceKindLocale(supportedLanguageTags.join(', ')),
};

/// The language tags of the product (01 9), the values `locale.default` takes; the server's
/// `400` decides.
List<String> get supportedLanguageTags => [
  for (final locale in AppLocalizations.supportedLocales)
    locale.toLanguageTag(),
];

/// The list of the instance settings ([InstanceRepository.instanceSettings]).
///
/// A key of the configuration reference section 6 that `putInstanceSetting` writes has "Change"
/// with its typed editor ([SettingEditorDialog]); a branding key points to the branding screen
/// (only `updateBranding` writes it, DECISIONS M1-46/1); a key this build does not know shows
/// its value read-only with the note to use the API. [changes] (the desktop's
/// `instance.updated`) loads the list again.
///
/// Keys: `settings.list`, `setting.<key>`, `setting.<key>.value`, `setting.<key>.meta`,
/// `setting.<key>.edit`, `setting.<key>.readOnly`.
class SettingsSection extends StatefulWidget {
  /// Creates the section.
  const SettingsSection({
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
  State<SettingsSection> createState() => _SettingsSectionState();
}

class _SettingsSectionState extends State<SettingsSection> {
  List<InstanceSettingEntry>? _settings;
  Map<String, String> _names = const {};
  Object? _error;
  SelfNotice? _notice;
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
      final settings = await widget.repository.instanceSettings();
      final names = await peopleNames(
        widget.repository,
        settings.map((s) => s.updatedBy),
      );
      if (!mounted) return;
      setState(() {
        _settings = settings;
        _names = names;
        _error = null;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _error = e);
    }
  }

  Future<void> _edit(InstanceSettingEntry entry, SettingSchema schema) async {
    final l = AppLocalizations.of(context);
    final saved = await SettingEditorDialog.show(
      context,
      repository: widget.repository,
      entry: entry,
      schema: schema,
    );
    if (saved == null || !mounted) return;
    setState(
      () => _notice = SelfNotice(
        l.instanceSettingSaved(entry.key, settingValueText(l, saved.value)),
      ),
    );
    widget.onChanged?.call();
    await _load();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final settings = _settings;
    final error = _error;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Column(
      key: const ValueKey('settings.list'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(l.instanceSettingsIntro, style: secondary),
        const SizedBox(height: DsSpace.s12),
        if (_notice case final n?) selfNoticeBanner(n),
        if (error != null)
          ProblemBanner(problem: loadProblem(error), onRetry: _load)
        else if (settings == null)
          const Center(child: CircularProgressIndicator())
        else
          for (final s in settings) _row(context, s),
      ],
    );
  }

  Widget _row(BuildContext context, InstanceSettingEntry s) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final schema = settingSchemaOf(s.key);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final editable = schema != null && schema.route == SettingRoute.settings;
    return Padding(
      key: ValueKey('setting.${s.key}'),
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
                  s.key,
                  style: dsTextStyle(DsType.code)
                      .copyWith(color: colors.textPrimary),
                ),
                Text(
                  settingValueText(l, s.value),
                  key: ValueKey('setting.${s.key}.value'),
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
                if (schema != null)
                  Text(settingKindText(l, schema.kind), style: secondary),
                Text(
                  changedText(
                    context,
                    version: s.version,
                    byName: s.updatedBy == null ? null : _names[s.updatedBy],
                    at: s.updatedAt,
                    now: widget.clock(),
                    timeZone: widget.timeZone,
                  ),
                  key: ValueKey('setting.${s.key}.meta'),
                  style: secondary,
                ),
              ],
            ),
          ),
          if (editable)
            OutlinedButton(
              key: ValueKey('setting.${s.key}.edit'),
              onPressed: () => _edit(s, schema),
              child: Text(l.instanceSettingChange),
            )
          else
            ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 320),
              child: Text(
                schema == null
                    ? l.instanceSettingUnknownKey
                    : l.instanceSettingOnBranding,
                key: ValueKey('setting.${s.key}.readOnly'),
                style: secondary,
              ),
            ),
        ],
      ),
    );
  }
}

/// The typed editor of one setting: a choice for a key with a closed list of values (the
/// language, the accent), a text field otherwise. "Save" first states the change ("from, to")
/// and asks for confirmation; the write names the version the editor started from. The
/// server's `400` stands at the input; a `412` shows the newer value next to the person's input,
/// takes its version and asks again (fixed requirement 14).
///
/// Keys: `setting.editor`, `setting.input`, `setting.option.<value>`, `setting.conflict`,
/// `setting.error`, `setting.save`, `setting.cancel`.
class SettingEditorDialog extends StatefulWidget {
  /// Creates the dialog for [entry] with its [schema].
  const SettingEditorDialog({
    super.key,
    required this.repository,
    required this.entry,
    required this.schema,
  });

  /// The administration's calls.
  final InstanceRepository repository;

  /// The setting as the list showed it.
  final InstanceSettingEntry entry;

  /// Its kind.
  final SettingSchema schema;

  /// Shows the dialog; returns the saved setting, or null when nothing was saved.
  static Future<InstanceSettingEntry?> show(
    BuildContext context, {
    required InstanceRepository repository,
    required InstanceSettingEntry entry,
    required SettingSchema schema,
  }) => showDialog<InstanceSettingEntry>(
    context: context,
    builder: (_) => SettingEditorDialog(
      repository: repository,
      entry: entry,
      schema: schema,
    ),
  );

  @override
  State<SettingEditorDialog> createState() => _SettingEditorDialogState();
}

class _SettingEditorDialogState extends State<SettingEditorDialog> {
  late InstanceSettingEntry _current = widget.entry;
  late final TextEditingController _text = TextEditingController(
    text: widget.entry.value is String ? widget.entry.value! as String : '',
  );
  late String? _choice = widget.entry.value is String
      ? widget.entry.value! as String
      : null;
  String? _fieldError;
  String? _failure;
  InstanceSettingEntry? _conflict;
  bool _busy = false;

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  bool get _closed =>
      widget.schema.kind == SettingKind.locale ||
      widget.schema.kind == SettingKind.paletteKey;

  List<String> get _options => switch (widget.schema.kind) {
    SettingKind.locale => supportedLanguageTags,
    SettingKind.paletteKey => accentPaletteKeys,
    _ => const [],
  };

  String? get _input => _closed ? _choice : _text.text;

  Future<void> _save() async {
    final l = AppLocalizations.of(context);
    final input = _input;
    final confirmed = await ConfirmChange.show(
      context,
      title: l.instanceSettingConfirmTitle(widget.entry.key),
      body: l.instanceChangeFromTo(
        settingValueText(l, _current.value),
        settingValueText(l, input),
      ),
      confirmLabel: l.instanceSave,
    );
    if (confirmed == null || !mounted) return;
    setState(() {
      _busy = true;
      _fieldError = null;
      _failure = null;
    });
    try {
      final saved = await widget.repository.putInstanceSetting(
        widget.entry.key,
        input,
        version: _current.version,
        purpose: l.stepUpPurposeInstanceSetting(widget.entry.key),
      );
      if (mounted) Navigator.of(context).pop(saved);
    } on InstanceVersionConflict<InstanceSettingEntry> catch (c) {
      // The newer value is shown next to the input; the next save names its version.
      if (mounted) {
        setState(() {
          _conflict = c.current;
          _current = c.current;
        });
      }
    } on Object catch (e) {
      if (!mounted) return;
      final field = fieldErrorAt(l, fieldErrorsOf(e), '/value');
      final failure = field == null ? selfFailure(l, e)?.text : null;
      setState(() {
        _fieldError = field;
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
    final conflict = _conflict;
    final input = _input;
    final changed = input != null && input != _current.value;
    return AlertDialog(
      key: const ValueKey('setting.editor'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        widget.entry.key,
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
                settingKindText(l, widget.schema.kind),
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: colors.textSecondary),
              ),
              const SizedBox(height: DsSpace.s12),
              if (conflict != null)
                Padding(
                  padding: const EdgeInsets.only(bottom: DsSpace.s12),
                  child: DsBanner(
                    key: const ValueKey('setting.conflict'),
                    tone: DsTone.warning,
                    text: l.instanceConflict(
                      settingValueText(l, conflict.value),
                      settingValueText(l, input),
                    ),
                    liveRegion: true,
                  ),
                ),
              if (_failure case final f?)
                Padding(
                  padding: const EdgeInsets.only(bottom: DsSpace.s12),
                  child: DsBanner(
                    key: const ValueKey('setting.error'),
                    tone: DsTone.danger,
                    text: f,
                    liveRegion: true,
                  ),
                ),
              if (_closed)
                DropdownButtonFormField<String>(
                  key: const ValueKey('setting.input'),
                  initialValue: _options.contains(_choice) ? _choice : null,
                  decoration: InputDecoration(
                    labelText: l.instanceSettingValue,
                    errorText: _fieldError,
                  ),
                  items: [
                    for (final o in _options)
                      DropdownMenuItem(
                        key: ValueKey('setting.option.$o'),
                        value: o,
                        child: Text(o),
                      ),
                  ],
                  onChanged: _busy ? null : (v) => setState(() => _choice = v),
                )
              else
                TextField(
                  key: const ValueKey('setting.input'),
                  controller: _text,
                  enabled: !_busy,
                  decoration: InputDecoration(
                    labelText: l.instanceSettingValue,
                    errorText: _fieldError,
                  ),
                  onChanged: (_) => setState(() {}),
                ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('setting.cancel'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: const ValueKey('setting.save'),
          onPressed: _busy || !changed ? null : _save,
          child: Text(l.instanceSave),
        ),
      ],
    );
  }
}
