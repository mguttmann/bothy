// SPDX-License-Identifier: Apache-2.0

/// The incoming webhooks per channel (11 8; brief M6-08 Design notes 4 and 5).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../data/apps_repository.dart';
import '../secrets/once_only_secret.dart';
import '../self/device_tile.dart' show activityText;
import '../self/self_feedback.dart';
import 'create_incoming_dialog.dart';

/// The incoming webhooks of one channel: a channel picker over the channels of
/// `repository.targets()` (shown as their path), the active webhooks of the chosen channel
/// with name, the app they write as (when `repository.apps()` lists it), created and last used,
/// "Create incoming webhook" ([CreateIncomingDialog]) and "Revoke" after one confirmation.
///
/// The address of a new webhook holds its token: it goes from the dialog's answer straight
/// into [OnceOnlySecret] and this view keeps no reference, so after "Close" nothing shows it
/// and a reload lists the webhook without any part of the token (08 12; Design note 4). The
/// chosen channel lives in this state only, never in a route (the console keeps no content in
/// its routes, ADR 0028). "New incoming app" is offered when `callerIsOwner()` answers true;
/// the server decides every change (15 1 principle 1).
///
/// Keys: `incoming.view`, `incoming.channel` with items `incoming.channel.<channelId>`,
/// `incoming.create`, row `incoming.row.<id>`, `incoming.revoke.<id>`; the secret dialog
/// `incoming.*` of [OnceOnlySecret] with the detail line `incoming.bearer`.
///
/// Example: `IncomingWebhooksView(repository: repository, timeZone: 'Europe/Berlin')`.
class IncomingWebhooksView extends StatefulWidget {
  /// Creates the view.
  const IncomingWebhooksView({
    super.key,
    required this.repository,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
    this.newSlugSuffix,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after every change, or null.
  final VoidCallback? onChanged;

  /// The random suffix of the slug of an "incoming app", or null for eight random characters
  /// of `[a-z0-9]` (tests pass a fixed one).
  final String Function()? newSlugSuffix;

  @override
  State<IncomingWebhooksView> createState() => _IncomingWebhooksViewState();
}

class _IncomingWebhooksViewState extends State<IncomingWebhooksView>
    with AdminActionState<IncomingWebhooksView> {
  List<AdminTarget>? _channels;
  ApiProblem? _channelsProblem;
  AdminTarget? _channel;
  List<IncomingWebhookInfo>? _hooks;
  ApiProblem? _hooksProblem;
  List<AppRecord> _apps = const [];
  bool _owner = false;

  @override
  void initState() {
    super.initState();
    _loadChannels();
  }

  Future<void> _loadChannels() async {
    try {
      final targets = await widget.repository.targets();
      final channels = [
        for (final t in targets)
          if (t.kind == AdminTargetKind.channel && t.id != null) t,
      ];
      await _loadApps();
      if (!mounted) return;
      setState(() {
        _channels = channels;
        _channelsProblem = null;
        _channel ??= channels.isEmpty ? null : channels.first;
      });
      await _loadHooks();
    } on Object catch (e) {
      if (mounted) setState(() => _channelsProblem = loadProblem(e));
    }
  }

  /// The apps (for the names and the app choice) and whether the caller is an owner. A refusal
  /// of either only hides what it would add: names, the existing apps or "New incoming app".
  Future<void> _loadApps() async {
    List<AppRecord> apps;
    bool owner;
    try {
      apps = await widget.repository.apps();
    } on Object {
      apps = const [];
    }
    try {
      owner = await widget.repository.callerIsOwner();
    } on Object {
      owner = false;
    }
    if (!mounted) return;
    setState(() {
      _apps = apps;
      _owner = owner;
    });
  }

  Future<void> _loadHooks() async {
    final channel = _channel;
    if (channel == null) return;
    try {
      final hooks = await widget.repository.incomingWebhooks(channel.id!);
      if (!mounted || _channel != channel) return;
      setState(() {
        _hooks = hooks;
        _hooksProblem = null;
      });
    } on Object catch (e) {
      if (mounted && _channel == channel) {
        setState(() => _hooksProblem = loadProblem(e));
      }
    }
  }

  void _choose(String? channelId) {
    final channel = _channels?.where((c) => c.id == channelId).firstOrNull;
    if (channel == null || channel == _channel) return;
    setState(() {
      _channel = channel;
      _hooks = null;
      _hooksProblem = null;
      notice = null;
    });
    _loadHooks();
  }

  Future<void> _create(AdminTarget channel) async {
    final l = AppLocalizations.of(context);
    var created = await CreateIncomingDialog.show(
      context,
      repository: widget.repository,
      channel: channel,
      apps: [
        for (final a in _apps)
          if (a.state == AppInstallState.active) a,
      ],
      offerIncomingApp: _owner,
      newSlugSuffix: widget.newSlugSuffix,
      onChanged: widget.onChanged,
    );
    if (created == null || !mounted) {
      if (mounted) await _loadApps();
      return;
    }
    final name = created.webhook.name;
    final url = created.url;
    // Dropped before the dialog opens, so only the dialog holds the address (Design note 4).
    created = null;
    setState(() => notice = SelfNotice(l.adminIncomingCreatedNotice(name)));
    await OnceOnlySecret.show(
      context,
      title: l.adminSecretIncomingTitle(name),
      label: l.adminSecretIncomingLabel,
      notice: l.adminSecretIncomingOnce,
      secret: url,
      keyPrefix: 'incoming',
      details: [
        const SizedBox(height: DsSpace.s8),
        Text(
          l.adminSecretIncomingBearer,
          key: const ValueKey('incoming.bearer'),
          style: dsTextStyle(DsType.bodySm),
        ),
      ],
    );
    widget.onChanged?.call();
    if (!mounted) return;
    await _loadApps();
    await _loadHooks();
  }

  Future<void> _revoke(IncomingWebhookInfo hook) async {
    final l = AppLocalizations.of(context);
    final ok = await ConfirmChange.show(
      context,
      title: l.adminIncomingRevokeTitle(hook.name),
      body: l.adminIncomingRevokeBody,
      confirmLabel: l.adminIncomingRevoke,
      destructive: true,
    );
    if (ok == null || !mounted) return;
    final done = await runChange('${hook.id}.revoke', () async {
      await widget.repository.revokeIncomingWebhook(
        hook.id,
        purpose: l.adminIncomingPurposeRevoke(hook.name),
      );
      return SelfNotice(l.adminIncomingRevokedNotice(hook.name));
    });
    if (!done) return;
    widget.onChanged?.call();
    if (mounted) await _loadHooks();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final channels = _channels;
    final channel = _channel;
    final hooks = _hooks;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textSecondary);
    return Column(
      key: const ValueKey('incoming.view'),
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // A Wrap, not a Row: at text scale 2 the button moves below the title instead of
        // squeezing it.
        Wrap(
          alignment: WrapAlignment.spaceBetween,
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: DsSpace.s12,
          runSpacing: DsSpace.s8,
          children: [
            Semantics(
              header: true,
              child: Text(
                l.adminIncomingTitle,
                style: dsTextStyle(DsType.heading)
                    .copyWith(color: colors.textPrimary),
              ),
            ),
            if (channel != null && hooks != null)
              OutlinedButton(
                key: const ValueKey('incoming.create'),
                onPressed: busy ? null : () => _create(channel),
                child: Text(l.adminIncomingCreate),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s8),
        if (_channelsProblem case final problem?)
          ProblemBanner(problem: problem, onRetry: _loadChannels)
        else if (channels != null) ...[
          if (channels.isEmpty)
            Text(l.adminIncomingNoChannels, style: body)
          else
            Row(
              children: [
                Text(
                  l.adminIncomingChannel,
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
                const SizedBox(width: DsSpace.s12),
                Flexible(
                  child: DropdownButton<String>(
                    key: const ValueKey('incoming.channel'),
                    isExpanded: true,
                    value: channel?.id,
                    onChanged: busy ? null : _choose,
                    items: [
                      for (final c in channels)
                        DropdownMenuItem(
                          key: ValueKey('incoming.channel.${c.id}'),
                          value: c.id,
                          child: Text(c.path),
                        ),
                    ],
                  ),
                ),
              ],
            ),
          const SizedBox(height: DsSpace.s8),
          noticeBanner(),
          if (_hooksProblem case final problem?)
            ProblemBanner(problem: problem, onRetry: _loadHooks)
          else if (hooks != null) ...[
            if (hooks.isEmpty) Text(l.adminIncomingEmpty, style: body),
            for (final h in hooks) ...[
              _row(context, l, colors, h),
              Divider(height: 1, color: colors.borderSubtle),
            ],
          ],
        ],
      ],
    );
  }

  Widget _row(
    BuildContext context,
    AppLocalizations l,
    DsColors colors,
    IncomingWebhookInfo h,
  ) {
    String when(DateTime at) => activityText(
      context,
      at,
      now: widget.clock(),
      timeZone: widget.timeZone,
    );
    final app = _apps.where((a) => a.id == h.appId).firstOrNull;
    final details = [
      if (app != null) l.adminIncomingWritesAs(app.displayName),
      l.adminIncomingCreatedAt(when(h.createdAt)),
      switch (h.lastUsedAt) {
        final used? => l.adminIncomingLastUsed(when(used)),
        null => l.adminIncomingNeverUsed,
      },
    ];
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Padding(
      key: ValueKey('incoming.row.${h.id}'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  h.name,
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
                Wrap(
                  spacing: DsSpace.s12,
                  children: [
                    for (final d in details) Text(d, style: secondary),
                  ],
                ),
              ],
            ),
          ),
          if (!denied.contains('${h.id}.revoke'))
            OutlinedButton(
              key: ValueKey('incoming.revoke.${h.id}'),
              onPressed: busy ? null : () => _revoke(h),
              child: Text(l.adminIncomingRevoke),
            ),
        ],
      ),
    );
  }
}
