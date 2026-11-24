// SPDX-License-Identifier: Apache-2.0

/// "Create channel..." and "Create space..." (01 6.25): a short dialog asks for a name and
/// creates a text channel in the space (`createChannel`, kind `text`) or a space
/// (`createSpace`), each with one `Idempotency-Key` per name. "Create channel..." of the space
/// menu shows when the stored access to the space has `manage` (`channel.create`, 05) and opens
/// the new channel; "Create space..." shows for an owner (`space.create`, owner only) in the
/// space menu and in the empty sidebar. A refusal stays in the dialog with the server's reason
/// and drops its key; a lost answer and `409 request_in_progress` keep it, so the next press is
/// the same request (docs/decisions.d/FEAT-SPACE-CHANNEL-CREATE.md).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../notifications_settings/providers.dart' show isOwnerProvider;
import '../thread/providers.dart' show spaceAccessProvider;
import 'providers.dart';
import 'structure_port.dart';

/// The entry "Create channel..." of the menu of [space] when the stored access to the space
/// has `manage` (`channel.create`), or none. Hidden until the server reported the access.
List<Widget> channelCreateMenuItems(
  BuildContext context,
  WidgetRef ref,
  Space space,
) {
  final access = ref.watch(spaceAccessProvider(space.id.value)).value;
  if (!(access?.capabilities.contains(Capability.manage) ?? false)) {
    return const [];
  }
  // Taken now: the entry runs after the menu closed and its ref is gone (D-TEAM-APP-6).
  final container = ProviderScope.containerOf(context, listen: false);
  return [
    DsMenuItem(
      key: const ValueKey('menu.createChannel'),
      onPressed: () =>
          unawaited(showChannelCreateDialog(context, container, space)),
      child: Text(AppLocalizations.of(context).structureCreateChannelMenu),
    ),
  ];
}

/// The entry "Create space..." of a space menu for an owner (`space.create`), or none.
List<Widget> spaceCreateMenuItems(BuildContext context, WidgetRef ref) {
  if (!(ref.watch(isOwnerProvider).value ?? false)) return const [];
  final container = ProviderScope.containerOf(context, listen: false);
  return [
    DsMenuItem(
      key: const ValueKey('menu.createSpace'),
      onPressed: () => unawaited(showSpaceCreateDialog(context, container)),
      child: Text(AppLocalizations.of(context).structureCreateSpaceMenu),
    ),
  ];
}

/// Opens the dialog that creates a text channel in [space] and then the new channel. Does
/// nothing without a session.
Future<void> showChannelCreateDialog(
  BuildContext context,
  ProviderContainer container,
  Space space,
) async {
  final port = container.read(structurePortProvider);
  if (port == null) return;
  final l = AppLocalizations.of(context);
  final created = await showDialog<String>(
    context: context,
    builder: (_) => StructureCreateDialog(
      title: l.structureCreateChannelTitle(dsIsolate(space.name)),
      newKey: container.read(structureKeysProvider),
      create: (name, key) async => (await port.createTextChannel(
        space.id,
        name,
        idempotencyKey: key,
      )).value,
    ),
  );
  if (created == null || !context.mounted) return;
  // The channel view waits for the sync to store the new channel (like "New project").
  GoRouter.maybeOf(context)?.go('/c/$created');
}

/// Opens the dialog that creates a space; the space shows in the sidebar once the sync
/// delivers it. Does nothing without a session.
Future<void> showSpaceCreateDialog(
  BuildContext context,
  ProviderContainer container,
) async {
  final port = container.read(structurePortProvider);
  if (port == null) return;
  final l = AppLocalizations.of(context);
  await showDialog<String>(
    context: context,
    builder: (_) => StructureCreateDialog(
      title: l.structureCreateSpaceTitle,
      newKey: container.read(structureKeysProvider),
      create: (name, key) async =>
          (await port.createSpace(name, idempotencyKey: key)).value,
    ),
  );
}

/// The dialog that asks for a name and runs [create]; it pops the ID [create] answers.
class StructureCreateDialog extends StatefulWidget {
  /// Creates the dialog; [newKey] makes the `Idempotency-Key` of [create].
  const StructureCreateDialog({
    super.key,
    required this.title,
    required this.create,
    required this.newKey,
  });

  /// The title.
  final String title;

  /// Creates the item with the trimmed name and the key and answers its ID; throws
  /// [StructureFailure].
  final Future<String> Function(String name, String idempotencyKey) create;

  /// Makes an idempotency key.
  final String Function() newKey;

  @override
  State<StructureCreateDialog> createState() => _StructureCreateDialogState();
}

class _StructureCreateDialogState extends State<StructureCreateDialog> {
  final TextEditingController _name = TextEditingController();

  String? _key;
  String? _keyFor;
  bool _busy = false;
  StructureFailure? _failure;

  String get _trimmed => _name.text.trim();

  bool get _canCreate => !_busy && _trimmed.isNotEmpty;

  Future<void> _create() async {
    final name = _trimmed;
    // One key per name: a repeated press after a lost answer is the same request.
    if (_key == null || _keyFor != name) {
      _key = widget.newKey();
      _keyFor = name;
    }
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      final id = await widget.create(name, _key!);
      if (mounted) Navigator.of(context).pop(id);
    } on StructureFailure catch (e) {
      // A refusal is kept under its key (04 5.2): the next press is a new request.
      if (!e.keepsKey) _key = null;
      if (mounted) setState(() => _failure = e);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final failure = _failure;
    // No way out while a call runs: the result would be lost to the person.
    return PopScope(
      canPop: !_busy,
      child: AlertDialog(
        key: const ValueKey('structureCreate.dialog'),
        title: Text(widget.title),
        content: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 420),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                key: const ValueKey('structureCreate.name'),
                controller: _name,
                autofocus: true,
                enabled: !_busy,
                onChanged: (_) => setState(() {}),
                onSubmitted: (_) {
                  if (_canCreate) unawaited(_create());
                },
                decoration: InputDecoration(labelText: l.structureCreateName),
              ),
              if (failure != null) ...[
                const SizedBox(height: DsSpace.s8),
                Semantics(
                  liveRegion: true,
                  child: Text(
                    structureFailureText(l, failure),
                    key: const ValueKey('structureCreate.failure'),
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: context.dsColors.dangerFg),
                  ),
                ),
              ],
            ],
          ),
        ),
        actions: [
          DsButton(
            key: const ValueKey('structureCreate.cancel'),
            label: l.structureCreateCancel,
            onPressed: _busy ? null : () => Navigator.of(context).pop(),
          ),
          DsButton(
            key: const ValueKey('structureCreate.submit'),
            label: l.structureCreateSubmit,
            style: DsButtonStyle.primary,
            onPressed: _canCreate ? () => unawaited(_create()) : null,
          ),
        ],
      ),
    );
  }
}

/// The text of a refused or failed create: "Refused: `reason`", the reason translated where
/// this app knows it, else the problem's text; or the note that no answer arrived.
String structureFailureText(AppLocalizations l, StructureFailure failure) {
  if (failure.offline) return l.structureCreateOffline;
  final reason = failure.reason;
  return l.structureCreateRefused(
    (reason == null ? null : dsReasonText(l, reason)) ??
        problemText(l, failure.code!, reason: reason),
  );
}
