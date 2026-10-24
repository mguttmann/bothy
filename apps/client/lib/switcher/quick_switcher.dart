// SPDX-License-Identifier: Apache-2.0

/// The quick switcher D6 (docs/spec/01-product.md 5.1 D6, 7: Cmd+K or Ctrl+K; brief M4-23
/// Design note 3): a dialog over the shell with the search field, the result rows and the
/// filter row. The binding of Cmd+K is M4-24; the sidebar's "Jump to" and that binding call
/// [showQuickSwitcher].
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import 'providers.dart';
import 'switcher_sources.dart';

/// Opens the quick switcher over [context] and goes to the target the person picks.
///
/// The commands offered are those of the channel the router shows when the switcher opens.
/// Enter on a row with a path navigates there through the [GoRouter] above [context]; a row
/// without a path (a command, a person without a stored DM) only closes the switcher; Esc
/// closes it without navigating. Completes when the switcher is closed.
///
/// Example: `onTap: () => showQuickSwitcher(context)`.
Future<void> showQuickSwitcher(BuildContext context) async {
  final router = GoRouter.maybeOf(context);
  final location =
      router?.routerDelegate.currentConfiguration.uri.toString() ?? '/';
  final path = await showDialog<String>(
    context: context,
    barrierColor: context.dsColors.scrim,
    builder: (_) => QuickSwitcher(location: location),
  );
  if (path != null) router?.go(path);
}

/// The dialog of the quick switcher; pops with the path of the chosen row, or null.
///
/// Keys: `switcher.dialog`, `switcher.field`, `switcher.row.<kind>.<id>`,
/// `switcher.filter.<name>`, `switcher.close`. Tab and Shift+Tab cycle the filter row, the
/// arrow keys move the selection, Enter takes the selected row, Esc closes.
class QuickSwitcher extends ConsumerStatefulWidget {
  /// Creates the switcher for the route [location] (path and query); the field starts with
  /// [initialQuery].
  const QuickSwitcher({
    super.key,
    required this.location,
    this.initialQuery = '',
  });

  /// The route the shell shows; its channel decides the commands offered.
  final String location;

  /// The text the field starts with.
  final String initialQuery;

  @override
  ConsumerState<QuickSwitcher> createState() => _QuickSwitcherState();
}

class _QuickSwitcherState extends ConsumerState<QuickSwitcher> {
  late final TextEditingController _field = TextEditingController(
    text: widget.initialQuery,
  );
  final FocusNode _fieldFocus = FocusNode();
  final GlobalKey _selectedRow = GlobalKey();
  SwitcherFilter _filter = SwitcherFilter.all;
  int _selected = 0;
  List<SwitcherEntry> _rows = const [];

  @override
  void initState() {
    super.initState();
    _field.addListener(() => setState(() => _selected = 0));
  }

  @override
  void dispose() {
    _field.dispose();
    _fieldFocus.dispose();
    super.dispose();
  }

  void _setFilter(SwitcherFilter filter) => setState(() {
    _filter = filter;
    _selected = 0;
  });

  void _move(int delta) {
    if (_rows.isEmpty) return;
    setState(() => _selected = (_selected + delta).clamp(0, _rows.length - 1));
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final row = _selectedRow.currentContext;
      if (row != null && row.mounted) Scrollable.ensureVisible(row);
    });
  }

  void _take(SwitcherEntry entry) => Navigator.of(context).pop(entry.path);

  void _takeSelected() {
    if (_selected < _rows.length) _take(_rows[_selected]);
  }

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent && event is! KeyRepeatEvent) {
      return KeyEventResult.ignored;
    }
    final key = event.logicalKey;
    if (key == LogicalKeyboardKey.tab) {
      _setFilter(
        HardwareKeyboard.instance.isShiftPressed
            ? _filter.previous
            : _filter.next,
      );
    } else if (key == LogicalKeyboardKey.arrowDown) {
      _move(1);
    } else if (key == LogicalKeyboardKey.arrowUp) {
      _move(-1);
    } else if (key == LogicalKeyboardKey.enter ||
        key == LogicalKeyboardKey.numpadEnter) {
      if (event is KeyDownEvent) _takeSelected();
    } else if (key == LogicalKeyboardKey.escape) {
      Navigator.of(context).pop();
    } else {
      return KeyEventResult.ignored;
    }
    return KeyEventResult.handled;
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final current = ref
        .watch(switcherCurrentChannelProvider(widget.location))
        .value;
    final loading = ref.watch(switcherLocalDataProvider(current)) == null;
    _rows = ref.watch(switcherRowsProvider(_field.text, _filter, current));
    if (_selected >= _rows.length) _selected = 0;
    return Dialog(
      key: const ValueKey('switcher.dialog'),
      alignment: Alignment.topCenter,
      insetPadding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s24,
        vertical: DsSpace.s48,
      ),
      backgroundColor: c.surfaceOverlay,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(DsRadius.lg),
      ),
      child: Semantics(
        scopesRoute: true,
        namesRoute: true,
        explicitChildNodes: true,
        label: l.switcherLabel,
        child: Focus(
          canRequestFocus: false,
          skipTraversal: true,
          onKeyEvent: _onKey,
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _SearchField(
                  controller: _field,
                  focusNode: _fieldFocus,
                  onSubmitted: _takeSelected,
                  onClose: () => Navigator.of(context).pop(),
                ),
                Divider(height: 1, color: c.borderSubtle),
                Flexible(
                  child: _rows.isEmpty
                      ? Padding(
                          padding: const EdgeInsets.all(DsSpace.s16),
                          child: loading
                              ? const SizedBox.shrink()
                              : Text(
                                  l.switcherNothing,
                                  key: const ValueKey('switcher.nothing'),
                                  style: dsTextStyle(DsType.bodySm)
                                      .copyWith(color: c.textMuted),
                                ),
                        )
                      : SingleChildScrollView(
                          padding: const EdgeInsets.symmetric(
                            vertical: DsSpace.s4,
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              for (final (i, entry) in _rows.indexed)
                                KeyedSubtree(
                                  key: i == _selected ? _selectedRow : null,
                                  child: SwitcherRow(
                                    key: ValueKey(
                                      'switcher.row.${entry.kind.name}.${entry.id}',
                                    ),
                                    entry: entry,
                                    selected: i == _selected,
                                    onTap: () => _take(entry),
                                  ),
                                ),
                            ],
                          ),
                        ),
                ),
                Divider(height: 1, color: c.borderSubtle),
                _FilterRow(
                  filter: _filter,
                  onSelected: (f) {
                    _setFilter(f);
                    // The field keeps the focus, so typing and Tab go on working.
                    _fieldFocus.requestFocus();
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// The field with the lens and the close button.
class _SearchField extends StatelessWidget {
  const _SearchField({
    required this.controller,
    required this.focusNode,
    required this.onSubmitted,
    required this.onClose,
  });

  final TextEditingController controller;
  final FocusNode focusNode;
  final VoidCallback onSubmitted;
  final VoidCallback onClose;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s12,
        vertical: DsSpace.s8,
      ),
      child: Row(
        children: [
          DsIcon(
            DsIcons.actionSearch,
            size: DsIconSizeRole.sm,
            color: c.textSecondary,
            decorative: true,
          ),
          const SizedBox(width: DsSpace.s8),
          Expanded(
            child: TextField(
              key: const ValueKey('switcher.field'),
              controller: controller,
              focusNode: focusNode,
              autofocus: true,
              onSubmitted: (_) => onSubmitted(),
              style: dsTextStyle(DsType.body).copyWith(color: c.textPrimary),
              decoration: InputDecoration(
                hintText: l.switcherHint,
                border: InputBorder.none,
                isDense: true,
              ),
            ),
          ),
          const SizedBox(width: DsSpace.s8),
          IconButton(
            key: const ValueKey('switcher.close'),
            tooltip: l.switcherClose,
            onPressed: onClose,
            icon: DsIcon(
              DsIcons.actionDismiss,
              size: DsIconSizeRole.sm,
              color: c.textSecondary,
              decorative: true,
            ),
          ),
        ],
      ),
    );
  }
}

/// One result row: icon or avatar, the name, and on the right the context and the kind.
class SwitcherRow extends StatelessWidget {
  /// Creates the row of [entry].
  const SwitcherRow({
    super.key,
    required this.entry,
    required this.selected,
    required this.onTap,
  });

  /// The row's target.
  final SwitcherEntry entry;

  /// Whether the keyboard selection is on this row.
  final bool selected;

  /// Takes the row.
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final kind = switcherKindLabel(l, entry.kind);
    final where = entry.context;
    return DsTappable(
      onTap: onTap,
      label: [entry.name, ?where, kind].join(', '),
      selected: selected,
      excludeChildSemantics: true,
      child: Container(
        color: selected ? c.surfaceSelected : null,
        constraints: BoxConstraints(minHeight: context.dsDensity.targetMin),
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s12,
          vertical: DsSpace.s4,
        ),
        child: Row(
          children: [
            SizedBox(
              width: DsSize.avatarMd,
              child: Center(child: _leading(context)),
            ),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Text(
                dsIsolate(entry.name),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: dsTextStyle(selected ? DsType.bodyStrong : DsType.body)
                    .copyWith(color: c.textPrimary),
              ),
            ),
            const SizedBox(width: DsSpace.s12),
            if (where != null) ...[
              Flexible(
                child: Text(
                  dsIsolate(where),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  textAlign: TextAlign.end,
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: c.textSecondary),
                ),
              ),
              const SizedBox(width: DsSpace.s8),
            ],
            Text(
              kind,
              style: dsTextStyle(DsType.caption).copyWith(color: c.textMuted),
            ),
          ],
        ),
      ),
    );
  }

  Widget _leading(BuildContext context) {
    final c = context.dsColors;
    DsIcon icon(DsGlyph glyph) => DsIcon(
      glyph,
      size: DsIconSizeRole.sm,
      color: c.textSecondary,
      decorative: true,
    );
    return switch (entry.kind) {
      SwitcherKind.space => icon(
        DsGlyph(DsIcons.spaceIcons.first, DsIconWeight.regular),
      ),
      SwitcherKind.channel => icon(DsIcons.channelText),
      SwitcherKind.thread => icon(DsIcons.thread),
      SwitcherKind.person => DsAvatar(
        name: entry.name,
        origin: DsMessageOrigin.human,
      ),
      SwitcherKind.agent => DsAvatar(
        name: entry.name,
        origin: DsMessageOrigin.agent,
      ),
      SwitcherKind.command => icon(DsIcons.actionQuickSwitch),
    };
  }
}

/// The kind label of a row ("Channel", "Person", ...).
String switcherKindLabel(AppLocalizations l, SwitcherKind kind) =>
    switch (kind) {
      SwitcherKind.space => l.switcherKindSpace,
      SwitcherKind.channel => l.switcherKindChannel,
      SwitcherKind.thread => l.switcherKindThread,
      SwitcherKind.person => l.switcherKindPerson,
      SwitcherKind.agent => l.switcherKindAgent,
      SwitcherKind.command => l.switcherKindCommand,
    };

/// The label of a filter of the filter row.
String switcherFilterLabel(AppLocalizations l, SwitcherFilter filter) =>
    switch (filter) {
      SwitcherFilter.all => l.switcherFilterAll,
      SwitcherFilter.channels => l.switcherFilterChannels,
      SwitcherFilter.threads => l.switcherFilterThreads,
      SwitcherFilter.people => l.switcherFilterPeople,
      SwitcherFilter.agents => l.switcherFilterAgents,
      SwitcherFilter.commands => l.switcherFilterCommands,
    };

/// "Tab: Filter" and the chips All, Channels, Threads, People, Agents, Commands.
class _FilterRow extends StatelessWidget {
  const _FilterRow({required this.filter, required this.onSelected});

  final SwitcherFilter filter;
  final ValueChanged<SwitcherFilter> onSelected;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s12,
        vertical: DsSpace.s8,
      ),
      child: Wrap(
        spacing: DsSpace.s6,
        runSpacing: DsSpace.s6,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          Text(
            l.switcherTabHint,
            style: dsTextStyle(DsType.caption).copyWith(color: c.textMuted),
          ),
          for (final f in SwitcherFilter.values)
            ChoiceChip(
              key: ValueKey('switcher.filter.${f.name}'),
              label: Text(switcherFilterLabel(l, f)),
              selected: f == filter,
              showCheckmark: false,
              onSelected: (_) => onSelected(f),
            ),
        ],
      ),
    );
  }
}
