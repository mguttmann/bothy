// SPDX-License-Identifier: Apache-2.0

/// Components on messages: buttons, select and form (components.md section 8).
library;

import 'package:flutter/cupertino.dart' show CupertinoPicker;
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_banner.dart';
import 'ds_common.dart';
import 'ds_menu_item.dart';
import 'markdown/ds_markdown.dart';
import 'view_models.dart';

/// A button in one of the style roles of `component.v1` (components.md section 8).
///
/// Primary on `color.accent.bg`, success on `color.success.bg`, danger on `color.danger.bg`,
/// secondary with the border `color.border.control`, ghost as text only. [loading] replaces
/// the leading space with the spinning `trigger.working` icon; [onPressed] null disables the
/// button (muted text, no hover). The hit area is at least `density.target-min`; only Enter or
/// Space on the focused button and a tap activate it.
///
/// ```dart
/// DsButton(label: 'Allow once', style: DsButtonStyle.primary, onPressed: allow)
/// ```
class DsButton extends StatelessWidget {
  /// Creates the button.
  const DsButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.style = DsButtonStyle.secondary,
    this.loading = false,
    this.expand = false,
  });

  /// The label (agent text or an `l10n` text).
  final String label;

  /// Called on activation; null disables the button.
  final VoidCallback? onPressed;

  /// The style role.
  final DsButtonStyle style;

  /// Whether the button shows the loading icon.
  final bool loading;

  /// Whether the button takes the full width (stacked on mobile devices).
  final bool expand;

  @override
  Widget build(BuildContext context) {
    final c = context.dsColors;
    final a = context.dsAccent;
    final l = dsL10n(context);
    final enabled = onPressed != null;
    final (Color? bg, Color fg, Color? border) = !enabled
        ? (c.surfaceSunken, c.textMuted, c.borderSubtle)
        : switch (style) {
            DsButtonStyle.primary => (a.accentBg, c.textOnAccent, null),
            DsButtonStyle.success => (c.successBg, c.successOnBg, null),
            DsButtonStyle.danger => (c.dangerBg, c.dangerOnBg, null),
            DsButtonStyle.ghost => (null, a.textLink, null),
            DsButtonStyle.secondary ||
            DsButtonStyle.unknown => (null, c.textPrimary, c.borderControl),
          };
    return DsTappable(
      onTap: onPressed,
      label: label,
      hint: loading ? l.componentsSending : null,
      radius: DsRadius.md,
      excludeChildSemantics: true,
      child: Container(
        constraints: BoxConstraints(minHeight: context.dsDensity.targetMin),
        padding: const EdgeInsets.symmetric(horizontal: DsSpace.s12),
        decoration: BoxDecoration(
          color: bg,
          borderRadius: BorderRadius.circular(DsRadius.md),
          border: border == null
              ? null
              : Border.all(color: border, width: DsBorderWidth.hairline),
        ),
        child: Row(
          mainAxisSize: expand ? MainAxisSize.max : MainAxisSize.min,
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            if (loading) ...[
              DsIcon(
                DsIcons.triggerWorking,
                size: DsIconSizeRole.sm,
                color: fg,
                decorative: true,
              ),
              const SizedBox(width: DsSpace.s6),
            ],
            Flexible(
              child: Text(
                label,
                style: dsTextStyle(DsType.label).copyWith(color: fg),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// The components of a card with the separated header (SEC-OUT-4, SEC-T14).
///
/// On top a framed header with the `security.step-up` icon, built only from [header]: the
/// agent, the run, the required capability and the "who may approve" note. Below a divider
/// labeled "Text from {agent}" the agent's [agentText] renders as Markdown, so a line that
/// imitates a header stays visibly agent text. Then the components by
/// [DsComponentData.state]: active and [permitted] operable; active and not permitted visible
/// but disabled with the lock note [deniedReasonText]; sent and waiting (the pressed button
/// loads, every other one is locked); used ("{Name} chose '{Choice}', {Time}"); expired;
/// disabled ("No longer available"); an error banner, after which an active card is operable
/// again.
///
/// [onActivate] receives the `custom_id` and the value: null for a button, the chosen values
/// for a select, the field map for a form. After the first activation the widget ignores
/// every further one until the caller passes a new [data] (a double click creates nothing
/// twice, 01 6.5). No keyboard shortcut activates; only Enter or Space on the focused
/// button (R-PR-04). [stacked] puts the buttons below each other at full width (mobile).
/// [sheets] is the rest of the mobile form (01 5.3 M4): a form opens as a sheet over the
/// keyboard, a select opens the system picker in a sheet (`CupertinoPicker` on iOS, a list
/// elsewhere); the values and the activation lock are the same as inline.
///
/// ```dart
/// DsMessageComponents(card, permitted: answer.permitted, deniedReasonText: reason,
///     header: header, agentText: message.content, onOpenLink: open,
///     onActivate: (id, value) => outbox.interact(message.id, id, value))
/// ```
class DsMessageComponents extends StatefulWidget {
  /// Creates the card body.
  const DsMessageComponents(
    this.data, {
    super.key,
    required this.permitted,
    required this.deniedReasonText,
    required this.header,
    required this.agentText,
    required this.onOpenLink,
    required this.onActivate,
    this.stacked = false,
    this.sheets = false,
  });

  /// The key of the framed header area (tests check that no agent text is inside).
  static const Key headerKey = ValueKey('ds-components-header');

  /// The components and their common state.
  final DsComponentData data;

  /// Whether the reader may use the components (from the server's answer).
  final bool permitted;

  /// The translated lock note when not [permitted], or null.
  final String? deniedReasonText;

  /// The server fields of the header.
  final DsComponentHeader header;

  /// The agent's Markdown text of the message, rendered below the divider.
  final String agentText;

  /// Opens an activated link of [agentText].
  final void Function(Uri) onOpenLink;

  /// Called once per activation with the `custom_id` and the value.
  final void Function(String customId, Object? value) onActivate;

  /// Whether buttons stack at full width.
  final bool stacked;

  /// Whether forms and selects open in sheets (mobile).
  final bool sheets;

  @override
  State<DsMessageComponents> createState() => DsMessageComponentsState();
}

/// State of [DsMessageComponents]: the activation lock and the form and select values.
class DsMessageComponentsState extends State<DsMessageComponents> {
  String? _pressed;
  final Map<String, Set<String>> _selected = {};
  final Map<String, TextEditingController> _text = {};
  final Map<String, bool> _switches = {};
  final Map<String, String> _choices = {};
  final Map<String, String> _fieldErrors = {};

  /// The texts of the header lines in order, for tests.
  @visibleForTesting
  List<String> get headerTexts => _headerLines(dsL10n(context));

  @override
  void didUpdateWidget(DsMessageComponents oldWidget) {
    super.didUpdateWidget(oldWidget);
    // The lock ends only when the caller reports a new state of the card; an equal view
    // model rebuilt by the parent keeps it (a double click creates nothing twice, 01 6.5).
    final o = oldWidget.data;
    final n = widget.data;
    if (o.messageId != n.messageId ||
        o.state != n.state ||
        o.pendingCustomId != n.pendingCustomId ||
        o.errorText != n.errorText) {
      _pressed = null;
    }
  }

  @override
  void dispose() {
    for (final c in _text.values) {
      c.dispose();
    }
    super.dispose();
  }

  String? get _pending => widget.data.pendingCustomId ?? _pressed;

  bool get _operable =>
      widget.permitted &&
      widget.data.state == DsComponentState.active &&
      _pending == null;

  void _activate(String customId, Object? value) {
    if (!_operable) return;
    setState(() => _pressed = customId);
    widget.onActivate(customId, value);
  }

  List<String> _headerLines(AppLocalizations l, {bool isolate = true}) {
    final h = widget.header;
    final capability = h.requiredCapability;
    String iso(String name) => isolate ? dsIsolate(name) : name;
    return [
      l.componentsHeaderRequest(iso(h.agentName)),
      if (h.runLabel != null) l.componentsHeaderRun(h.runLabel!),
      if (capability != null)
        l.componentsHeaderCapability(
          capability == 'approve_actions'
              ? l.capabilityApproveActions
              : capability,
        ),
      ...switch (h.approver) {
        DsApprover.runStarter when h.runStarterName != null => [
          l.componentsApproverRunStarter(iso(h.runStarterName!)),
        ],
        DsApprover.owner => [l.componentsApproverOwner],
        DsApprover.any => [l.componentsApproverAny],
        _ => <String>[],
      },
    ];
  }

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final secondary = dsTextStyle(DsType.caption)
        .copyWith(color: c.textSecondary);
    final lines = _headerLines(l);
    final plain = _headerLines(l, isolate: false);
    // One semantics node with the plain names (isolation marks are for display only).
    final header = Semantics(
      container: true,
      label: plain.join('. '),
      excludeSemantics: true,
      child: Container(
        key: DsMessageComponents.headerKey,
        padding: const EdgeInsets.all(DsSpace.s8),
        decoration: BoxDecoration(
          color: c.surfaceSunken,
          border: Border.all(
            color: c.borderDefault,
            width: DsBorderWidth.hairline,
          ),
          borderRadius: BorderRadius.circular(DsRadius.sm),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            DsIcon(
              DsIcons.securityStepUp,
              size: DsIconSizeRole.sm,
              color: c.textSecondary,
              decorative: true,
            ),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    lines.first,
                    style: dsTextStyle(DsType.label)
                        .copyWith(color: c.textPrimary),
                  ),
                  for (final line in lines.skip(1))
                    Text(line, style: secondary),
                ],
              ),
            ),
          ],
        ),
      ),
    );
    final divider = Padding(
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Row(
        children: [
          Text(
            l.componentsTextFrom(dsIsolate(widget.header.agentName)),
            semanticsLabel: l.componentsTextFrom(widget.header.agentName),
            style: secondary,
          ),
          const SizedBox(width: DsSpace.s8),
          Expanded(
            child: Container(
              height: DsBorderWidth.hairline,
              color: c.borderDefault,
            ),
          ),
        ],
      ),
    );
    // The group is labeled by the server header; the agent's card title is read only after
    // the divider, with the rest of the agent text, so it cannot precede the header.
    return Semantics(
      container: true,
      explicitChildNodes: true,
      label: plain.first,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          header,
          divider,
          if (widget.data.title != null)
            Semantics(
              container: true,
              label: widget.data.title,
              child: const SizedBox.shrink(),
            ),
          if (widget.agentText.isNotEmpty)
            DsMarkdown(widget.agentText, onOpenLink: widget.onOpenLink),
          const SizedBox(height: DsSpace.s8),
          ..._body(context),
        ],
      ),
    );
  }

  List<Widget> _body(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final d = widget.data;
    final caption = dsTextStyle(DsType.bodySm);
    switch (d.state) {
      case DsComponentState.used:
        return [
          Row(
            children: [
              DsIcon(
                DsIcons.triggerDone,
                size: DsIconSizeRole.sm,
                color: c.successFg,
                decorative: true,
              ),
              const SizedBox(width: DsSpace.s6),
              Expanded(
                child: Text(
                  l.componentsUsed(
                    dsIsolate(d.usedByName ?? ''),
                    d.usedChoice ?? '',
                    d.usedAtLabel ?? '',
                  ),
                  style: caption.copyWith(color: c.textSecondary),
                ),
              ),
            ],
          ),
        ];
      case DsComponentState.expired:
        return [
          Text(
            l.componentsExpired,
            style: caption.copyWith(color: c.textMuted),
          ),
        ];
      case DsComponentState.disabled || DsComponentState.unknown:
        return [
          Text(
            l.componentsDisabled,
            style: caption.copyWith(color: c.textMuted),
          ),
        ];
      case DsComponentState.active:
        break;
    }
    final out = <Widget>[];
    if (d.errorText != null) {
      out
        ..add(
          DsBanner(tone: DsTone.danger, text: d.errorText!, liveRegion: true),
        )
        ..add(const SizedBox(height: DsSpace.s8));
    }
    if (!widget.permitted && widget.deniedReasonText != null) {
      out
        ..add(
          Row(
            children: [
              DsIcon(
                DsIcons.securityLock,
                size: DsIconSizeRole.sm,
                color: c.textSecondary,
                decorative: true,
              ),
              const SizedBox(width: DsSpace.s6),
              Expanded(
                child: Text(
                  widget.deniedReasonText!,
                  style: caption.copyWith(color: c.textSecondary),
                ),
              ),
            ],
          ),
        )
        ..add(const SizedBox(height: DsSpace.s8));
    }
    final buttons = <Widget>[];
    for (final item in d.items) {
      switch (item.type) {
        case DsComponentType.button:
          buttons.add(
            DsButton(
              key: ValueKey('ds-component-${item.customId}'),
              label: item.label,
              style: item.style,
              loading: _pending == item.customId,
              expand: widget.stacked,
              onPressed: _operable
                  ? () => _activate(item.customId, null)
                  : null,
            ),
          );
        case DsComponentType.select:
          out
            ..add(_select(context, item))
            ..add(const SizedBox(height: DsSpace.s8));
        case DsComponentType.form:
          out
            ..add(_form(context, item))
            ..add(const SizedBox(height: DsSpace.s8));
        case DsComponentType.unknown:
          out.add(
            Text(
              l.componentsUnsupported,
              style: caption.copyWith(color: c.textSecondary),
            ),
          );
      }
    }
    if (buttons.isNotEmpty) {
      out.add(
        widget.stacked
            ? Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  for (final b in buttons)
                    Padding(
                      padding: const EdgeInsets.only(bottom: DsSpace.s6),
                      child: b,
                    ),
                ],
              )
            : Wrap(
                spacing: DsSpace.s8,
                runSpacing: DsSpace.s6,
                children: buttons,
              ),
      );
    }
    return out;
  }

  Widget _select(BuildContext context, DsComponentItem item) {
    final c = context.dsColors;
    final multiple = item.maxValues > 1;
    final chosen = _selected.putIfAbsent(item.customId, () => <String>{});
    final labels = [
      for (final o in item.options)
        if (chosen.contains(o.value)) o.label,
    ];
    final enabled = _operable;
    if (widget.sheets) {
      return _selectField(
        context,
        item,
        labels,
        enabled ? () => _openSelectSheet(context, item) : null,
      );
    }
    return MenuAnchor(
      style: MenuStyle(
        backgroundColor: WidgetStatePropertyAll(c.surfaceOverlay),
      ),
      menuChildren: [
        for (final o in item.options)
          if (multiple)
            CheckboxMenuButton(
              value: chosen.contains(o.value),
              closeOnActivate: false,
              onChanged: (_) => setState(() => _toggle(item, o.value)),
              child: Text(o.label),
            )
          else
            DsMenuItem(
              selected: chosen.contains(o.value),
              onPressed: () => _chooseOne(item, o.value),
              child: Text(o.label),
            ),
      ],
      builder: (context, controller, _) => Row(
        children: [
          Expanded(
            child: _selectField(
              context,
              item,
              labels,
              enabled
                  ? () => controller.isOpen
                        ? controller.close()
                        : controller.open()
                  : null,
            ),
          ),
          if (multiple) ...[
            const SizedBox(width: DsSpace.s8),
            _selectSubmit(item),
          ],
        ],
      ),
    );
  }

  /// Adds or removes [value] of a multiple select, within `max_values`.
  void _toggle(DsComponentItem item, String value) {
    final chosen = _selected.putIfAbsent(item.customId, () => <String>{});
    if (!chosen.remove(value) && chosen.length < item.maxValues) {
      chosen.add(value);
    }
  }

  /// Chooses [value] of a single select and activates it.
  void _chooseOne(DsComponentItem item, String value) {
    setState(
      () => _selected.putIfAbsent(item.customId, () => <String>{})
        ..clear()
        ..add(value),
    );
    _activate(item.customId, [value]);
  }

  /// The submit button of a multiple select.
  Widget _selectSubmit(DsComponentItem item, {VoidCallback? after}) {
    final chosen = _selected.putIfAbsent(item.customId, () => <String>{});
    return DsButton(
      key: ValueKey('ds-component-submit-${item.customId}'),
      label: dsL10n(context).componentsSubmit,
      style: DsButtonStyle.primary,
      loading: _pending == item.customId,
      expand: after != null,
      onPressed:
          _operable &&
              chosen.length >= item.minValues &&
              chosen.length <= item.maxValues
          ? () {
              _activate(item.customId, [
                for (final o in item.options)
                  if (chosen.contains(o.value)) o.value,
              ]);
              after?.call();
            }
          : null,
    );
  }

  /// The closed field of a select: the chosen labels or the placeholder; [onTap] opens it.
  Widget _selectField(
    BuildContext context,
    DsComponentItem item,
    List<String> labels,
    VoidCallback? onTap,
  ) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final text = labels.isEmpty
        ? (item.placeholder ?? l.componentsSelectPlaceholder)
        : labels.join(', ');
    return DsTappable(
      key: ValueKey('ds-component-select-${item.customId}'),
      onTap: onTap,
      label: text,
      radius: DsRadius.md,
      excludeChildSemantics: true,
      child: Container(
        constraints: BoxConstraints(minHeight: context.dsDensity.targetMin),
        padding: const EdgeInsets.symmetric(horizontal: DsSpace.s12),
        decoration: BoxDecoration(
          color: c.surfaceInput,
          border: Border.all(
            color: c.borderControl,
            width: DsBorderWidth.hairline,
          ),
          borderRadius: BorderRadius.circular(DsRadius.md),
        ),
        child: Row(
          children: [
            Expanded(
              child: Text(
                text,
                style: dsTextStyle(
                  DsType.bodySm,
                ).copyWith(color: onTap != null ? c.textPrimary : c.textMuted),
              ),
            ),
            DsIcon(
              DsIcons.actionExpand,
              size: DsIconSizeRole.sm,
              color: c.textSecondary,
              decorative: true,
            ),
          ],
        ),
      ),
    );
  }

  /// The select in a sheet (mobile): a single select in the system picker of the platform
  /// (`CupertinoPicker` on iOS, a list elsewhere), a multiple select as a list of check boxes;
  /// both activate only with a tap on an option or on the submit button.
  Future<void> _openSelectSheet(BuildContext context, DsComponentItem item) {
    final iOS = Theme.of(context).platform == TargetPlatform.iOS;
    final multiple = item.maxValues > 1;
    final chosen = _selected.putIfAbsent(item.customId, () => <String>{});
    return showModalBottomSheet<void>(
      context: context,
      useSafeArea: true,
      isScrollControlled: true,
      builder: (sheet) => StatefulBuilder(
        builder: (sheet, setSheet) {
          void close() => Navigator.of(sheet).pop();
          final Widget body;
          if (multiple) {
            body = Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                for (final o in item.options)
                  CheckboxListTile(
                    key: ValueKey('ds-component-option-${o.value}'),
                    value: chosen.contains(o.value),
                    title: Text(o.label),
                    onChanged: (_) {
                      setState(() => _toggle(item, o.value));
                      setSheet(() {});
                    },
                  ),
                const SizedBox(height: DsSpace.s8),
                _selectSubmit(item, after: close),
              ],
            );
          } else if (iOS) {
            var index = item.options.indexWhere(
              (o) => chosen.contains(o.value),
            );
            if (index < 0) index = 0;
            body = Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                SizedBox(
                  height: context.dsDensity.targetMin * 4,
                  child: CupertinoPicker(
                    key: ValueKey('ds-component-picker-${item.customId}'),
                    itemExtent: context.dsDensity.targetMin,
                    scrollController: FixedExtentScrollController(
                      initialItem: index,
                    ),
                    onSelectedItemChanged: (i) => index = i,
                    children: [
                      for (final o in item.options)
                        Center(child: Text(o.label)),
                    ],
                  ),
                ),
                const SizedBox(height: DsSpace.s8),
                DsButton(
                  key: ValueKey('ds-component-submit-${item.customId}'),
                  label: dsL10n(sheet).componentsSubmit,
                  style: DsButtonStyle.primary,
                  expand: true,
                  onPressed: () {
                    close();
                    _chooseOne(item, item.options[index].value);
                  },
                ),
              ],
            );
          } else {
            body = Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                for (final o in item.options)
                  ListTile(
                    key: ValueKey('ds-component-option-${o.value}'),
                    title: Text(o.label),
                    selected: chosen.contains(o.value),
                    onTap: () {
                      close();
                      _chooseOne(item, o.value);
                    },
                  ),
              ],
            );
          }
          return SingleChildScrollView(
            padding: const EdgeInsets.all(DsSpace.s16),
            child: body,
          );
        },
      ),
    );
  }

  Widget _form(BuildContext context, DsComponentItem item) {
    if (!widget.sheets) return _formBody(context, item, setState);
    final l = dsL10n(context);
    final c = context.dsColors;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (item.title != null) ...[
          Text(
            item.title!,
            style: dsTextStyle(DsType.bodyStrong)
                .copyWith(color: c.textPrimary),
          ),
          const SizedBox(height: DsSpace.s8),
        ],
        DsButton(
          key: ValueKey('ds-component-form-${item.customId}'),
          label: l.componentsOpenForm,
          style: DsButtonStyle.primary,
          expand: true,
          loading: _pending == item.customId,
          onPressed: _operable ? () => _openFormSheet(context, item) : null,
        ),
      ],
    );
  }

  /// The form in a sheet over the keyboard (mobile): the same fields and checks as inline; a
  /// submit without errors activates and closes the sheet.
  Future<void> _openFormSheet(BuildContext context, DsComponentItem item) =>
      showModalBottomSheet<void>(
        context: context,
        useSafeArea: true,
        isScrollControlled: true,
        builder: (sheet) => StatefulBuilder(
          builder: (sheet, setSheet) => Padding(
            // The sheet sits above the keyboard.
            padding: EdgeInsets.only(
              bottom: MediaQuery.viewInsetsOf(sheet).bottom,
            ),
            child: SingleChildScrollView(
              key: ValueKey('ds-component-form-sheet-${item.customId}'),
              padding: const EdgeInsets.all(DsSpace.s16),
              child: _formBody(sheet, item, (fn) {
                setState(fn);
                setSheet(() {});
              }, onActivated: () => Navigator.of(sheet).pop()),
            ),
          ),
        ),
      );

  /// The fields and the submit button of a form; [update] rebuilds what shows them (the card
  /// inline, the sheet and the card on mobile); [onActivated] runs after a submit activated.
  Widget _formBody(
    BuildContext context,
    DsComponentItem item,
    void Function(VoidCallback) update, {
    VoidCallback? onActivated,
  }) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final enabled = _operable;
    final labelStyle = dsTextStyle(DsType.label).copyWith(color: c.textPrimary);
    final fields = <Widget>[];
    for (final f in item.fields) {
      final key = '${item.customId}/${f.name}';
      final error = _fieldErrors[key];
      Widget input;
      switch (f.type) {
        case DsFieldType.boolean:
          input = Align(
            alignment: Alignment.centerLeft,
            child: Switch(
              value: _switches[key] ?? false,
              onChanged: enabled
                  ? (v) => update(() => _switches[key] = v)
                  : null,
            ),
          );
        case DsFieldType.choice:
          input = Wrap(
            spacing: DsSpace.s6,
            children: [
              for (final choice in f.choices)
                ChoiceChip(
                  label: Text(choice),
                  selected: _choices[key] == choice,
                  onSelected: enabled
                      ? (_) => update(() => _choices[key] = choice)
                      : null,
                ),
            ],
          );
        case DsFieldType.text ||
            DsFieldType.textarea ||
            DsFieldType.number ||
            DsFieldType.unknown:
          input = TextField(
            key: ValueKey('ds-field-${f.name}'),
            controller: _text.putIfAbsent(key, TextEditingController.new),
            enabled: enabled,
            minLines: f.type == DsFieldType.textarea ? 3 : 1,
            maxLines: f.type == DsFieldType.textarea ? null : 1,
            keyboardType: f.type == DsFieldType.number
                ? TextInputType.number
                : null,
            style: dsTextStyle(DsType.bodySm).copyWith(color: c.textPrimary),
            decoration: InputDecoration(
              isDense: true,
              constraints: BoxConstraints(
                minHeight: context.dsDensity.targetMin,
              ),
              filled: true,
              fillColor: c.surfaceInput,
              errorText: error,
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(DsRadius.md),
                borderSide: BorderSide(color: c.borderControl),
              ),
            ),
          );
      }
      fields
        ..add(
          Semantics(
            label: f.label,
            child: ExcludeSemantics(child: Text(f.label, style: labelStyle)),
          ),
        )
        ..add(const SizedBox(height: DsSpace.s4))
        ..add(input)
        ..add(const SizedBox(height: DsSpace.s8));
    }
    return Column(
      crossAxisAlignment: onActivated == null
          ? CrossAxisAlignment.start
          : CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (item.title != null) ...[
          Text(
            item.title!,
            style: dsTextStyle(DsType.bodyStrong)
                .copyWith(color: c.textPrimary),
          ),
          const SizedBox(height: DsSpace.s8),
        ],
        ...fields,
        DsButton(
          key: onActivated == null
              ? null
              : ValueKey('ds-component-submit-${item.customId}'),
          label: item.submitLabel ?? l.componentsSubmit,
          style: DsButtonStyle.primary,
          loading: _pending == item.customId,
          expand: onActivated != null,
          onPressed: enabled
              ? () {
                  final activated = _submit(item, update);
                  if (activated) onActivated?.call();
                }
              : null,
        ),
      ],
    );
  }

  /// Checks the fields of [item] and activates it with their values; returns whether it
  /// activated. [update] shows the errors where the fields are.
  bool _submit(DsComponentItem item, void Function(VoidCallback) update) {
    final l = dsL10n(context);
    final values = <String, Object?>{};
    final errors = <String, String>{};
    for (final f in item.fields) {
      final key = '${item.customId}/${f.name}';
      switch (f.type) {
        case DsFieldType.boolean:
          values[f.name] = _switches[key] ?? false;
        case DsFieldType.choice:
          final v = _choices[key];
          if (v == null && f.required) errors[key] = l.componentsFieldRequired;
          values[f.name] = v;
        case DsFieldType.text ||
            DsFieldType.textarea ||
            DsFieldType.number ||
            DsFieldType.unknown:
          final text = _text[key]?.text ?? '';
          if (text.isEmpty) {
            if (f.required) errors[key] = l.componentsFieldRequired;
            values[f.name] = null;
          } else if (f.maxLength != null && text.length > f.maxLength!) {
            errors[key] = l.componentsFieldTooLong(f.maxLength!);
          } else if (f.type == DsFieldType.number) {
            final n = num.tryParse(text);
            if (n == null) {
              errors[key] = l.componentsFieldNumber;
            } else {
              values[f.name] = n;
            }
          } else {
            values[f.name] = text;
          }
      }
    }
    update(() {
      _fieldErrors
        ..removeWhere((k, _) => k.startsWith('${item.customId}/'))
        ..addAll(errors);
    });
    if (errors.isNotEmpty || !_operable) return false;
    _activate(item.customId, values);
    return true;
  }
}
