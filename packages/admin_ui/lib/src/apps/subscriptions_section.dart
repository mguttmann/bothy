// SPDX-License-Identifier: Apache-2.0

/// The outgoing webhook subscriptions of an app and their deliveries (11 6, 7; brief M6-08
/// Design note 3, Ruling 4).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/apps_repository.dart';
import '../secrets/once_only_secret.dart';
import '../self/device_tile.dart' show activityText;
import '../self/self_feedback.dart';
import 'deliveries_view.dart';
import 'subscription_labels.dart';
import 'webhook_keys_card.dart';

/// The subscriptions of [app] (`listWebhookSubscriptions`): address, event types, state,
/// failures in a row, signature schemes and key ID; a disabled one shows when and why
/// (Ruling 4, read from its newest delivery). "New subscription" offers only the events the
/// installation granted and an `https` address; the server checks the host against
/// `webhook.allowed_hosts` and its refusal is shown in the form. With "Also sign with HMAC"
/// the secret of the answer appears once in [OnceOnlySecret] and is kept nowhere else.
/// Pause, resume and delete run after one confirmation; `webhook_out.manage` has step-up,
/// which the interceptor asks when the server demands it. "Deliveries" opens the
/// [DeliveriesView]; the [WebhookKeysCard] below shows the keys receivers verify with.
///
/// Keys: `subs.section`, `subs.create`, row `subs.row.<id>`, `subs.state.<id>`,
/// `subs.reason.<id>`, `subs.pause.<id>`, `subs.resume.<id>`, `subs.delete.<id>`,
/// `subs.deliveries.<id>`; form `subs.form`, `subs.url`, `subs.event.<type>`, `subs.hmac`,
/// `subs.submit`; the secret dialog `hmac.*`.
///
/// Example: `AppSubscriptionsSection(repository: repository, app: app, timeZone: 'Europe/Berlin')`.
class AppSubscriptionsSection extends StatefulWidget {
  /// Creates the section.
  const AppSubscriptionsSection({
    super.key,
    required this.repository,
    required this.app,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The app with its granted events.
  final AppRecord app;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after every change, or null.
  final VoidCallback? onChanged;

  @override
  State<AppSubscriptionsSection> createState() =>
      _AppSubscriptionsSectionState();
}

class _AppSubscriptionsSectionState extends State<AppSubscriptionsSection>
    with AdminActionState<AppSubscriptionsSection> {
  List<AppSubscription>? _subs;
  ApiProblem? _problem;

  // The newest delivery of each disabled subscription, the source of its reason (Ruling 4).
  Map<String, AppDelivery?> _newest = const {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(AppSubscriptionsSection old) {
    super.didUpdateWidget(old);
    if (old.app.id != widget.app.id) _load();
  }

  Future<void> _load() async {
    try {
      final subs = await widget.repository.subscriptions(widget.app.id);
      final newest = <String, AppDelivery?>{};
      for (final s in subs) {
        if (s.state != SubscriptionState.disabled) continue;
        newest[s.id] = await _newestOf(s.id);
      }
      if (mounted) {
        setState(() {
          _subs = subs;
          _newest = newest;
          _problem = null;
        });
      }
    } on Object catch (e) {
      if (mounted) setState(() => _problem = loadProblem(e));
    }
  }

  /// The newest finished (`failed` or `dead`) delivery on the first page of
  /// [subscriptionId], or null when there is none or the list is refused (the reason then
  /// falls back to the failures in a row). The server lists by sequence, newest first, and the
  /// events after the one that ended the subscription still wait as `pending`, so the newest
  /// row alone would hide the cause.
  Future<AppDelivery?> _newestOf(String subscriptionId) async {
    try {
      final page = await widget.repository.deliveries(subscriptionId);
      return page.items
          .where(
            (d) =>
                d.status == DeliveryStatus.failed ||
                d.status == DeliveryStatus.dead,
          )
          .firstOrNull;
    } on Object {
      return null;
    }
  }

  Future<void> _changed() async {
    widget.onChanged?.call();
    if (mounted) await _load();
  }

  Future<void> _create() async {
    final l = AppLocalizations.of(context);
    final app = widget.app;
    var saved = await showDialog<SubscriptionSaved>(
      context: context,
      // Closed only by its buttons: an answer that arrives after a tap outside would be
      // dropped, and with it an HMAC secret the server never shows again.
      barrierDismissible: false,
      builder: (_) => _SubscriptionForm(
        appName: app.displayName,
        events: app.eventSubscriptions,
        create: (url, events, hmac) => widget.repository.createSubscription(
          app.id,
          url: url,
          eventTypes: events,
          enableHmac: hmac,
          purpose: l.adminHookPurposeCreate(app.displayName),
        ),
      ),
    );
    if (saved == null || !mounted) return;
    final url = saved.subscription.url;
    final secret = saved.hmacSecret;
    // Dropped before the dialog opens, so only the dialog holds the secret (Design note 4).
    saved = null;
    setState(() => notice = SelfNotice(l.adminHookCreatedNotice(url)));
    if (secret != null) {
      await OnceOnlySecret.show(
        context,
        title: l.adminSecretHmacTitle(url),
        label: l.adminSecretHmacLabel,
        notice: l.adminSecretHmacOnce,
        secret: secret,
        keyPrefix: 'hmac',
      );
    }
    await _changed();
  }

  Future<void> _setState(AppSubscription s, SubscriptionState state) async {
    final l = AppLocalizations.of(context);
    final pause = state == SubscriptionState.paused;
    final ok = await ConfirmChange.show(
      context,
      title: pause
          ? l.adminHookPauseTitle(s.url)
          : l.adminHookResumeTitle(s.url),
      body: pause ? l.adminHookPauseBody : l.adminHookResumeBody,
      confirmLabel: pause ? l.adminHookPause : l.adminHookResume,
    );
    if (ok == null || !mounted) return;
    final done = await runChange('${s.id}.state', () async {
      await widget.repository.updateSubscription(
        s.id,
        state: state,
        purpose: l.adminHookPurposeChange(s.url),
      );
      return SelfNotice(
        pause
            ? l.adminHookPausedNotice(s.url)
            : l.adminHookResumedNotice(s.url),
      );
    });
    if (done) await _changed();
  }

  Future<void> _delete(AppSubscription s) async {
    final l = AppLocalizations.of(context);
    final ok = await ConfirmChange.show(
      context,
      title: l.adminHookDeleteTitle(s.url),
      body: l.adminHookDeleteBody,
      confirmLabel: l.adminHookDelete,
      destructive: true,
    );
    if (ok == null || !mounted) return;
    final done = await runChange('${s.id}.delete', () async {
      await widget.repository.deleteSubscription(
        s.id,
        purpose: l.adminHookPurposeDelete(s.url),
      );
      return SelfNotice(l.adminHookDeletedNotice(s.url));
    });
    if (done) await _changed();
  }

  Future<void> _deliveries(AppSubscription s) => showDialog<void>(
    context: context,
    builder: (context) {
      final l = AppLocalizations.of(context);
      return Dialog(
        backgroundColor: context.dsColors.surfaceRaised,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 720),
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(DsSpace.s16),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                DeliveriesView(
                  repository: widget.repository,
                  subscription: s,
                  timeZone: widget.timeZone,
                  clock: widget.clock,
                ),
                Align(
                  alignment: AlignmentDirectional.centerEnd,
                  child: FilledButton(
                    key: const ValueKey('deliveries.close'),
                    onPressed: () => Navigator.of(context).pop(),
                    child: Text(l.adminCodeClose),
                  ),
                ),
              ],
            ),
          ),
        ),
      );
    },
  );

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final subs = _subs;
    return Column(
      key: const ValueKey('subs.section'),
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Semantics(
                header: true,
                child: Text(
                  l.adminHookTitle,
                  style: dsTextStyle(DsType.heading)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ),
            if (subs != null)
              OutlinedButton(
                key: const ValueKey('subs.create'),
                onPressed: busy ? null : _create,
                child: Text(l.adminHookCreate),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s8),
        noticeBanner(),
        if (_problem case final problem?)
          ProblemBanner(problem: problem, onRetry: _load)
        else if (subs != null) ...[
          if (subs.isEmpty)
            Text(
              l.adminHookEmpty,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
          for (final s in subs) ...[
            _row(context, l, colors, s),
            Divider(height: 1, color: colors.borderSubtle),
          ],
        ],
        const SizedBox(height: DsSpace.s24),
        WebhookKeysCard(
          repository: widget.repository,
          timeZone: widget.timeZone,
          clock: widget.clock,
        ),
      ],
    );
  }

  Widget _row(
    BuildContext context,
    AppLocalizations l,
    DsColors colors,
    AppSubscription s,
  ) {
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final disabled = s.state == SubscriptionState.disabled;
    final details = [
      l.adminHookEvents(s.eventTypes.join(', ')),
      if (s.consecutiveFailures > 0) l.adminHookFailures(s.consecutiveFailures),
      l.adminHookSigned(s.signatureSchemes.join(', '), s.signingKeyKid),
      if (disabled && s.disabledAt != null)
        l.adminHookDisabledAt(
          activityText(
            context,
            s.disabledAt!,
            now: widget.clock(),
            timeZone: widget.timeZone,
          ),
        ),
    ];
    final actions = <Widget>[
      if (s.state == SubscriptionState.active &&
          !denied.contains('${s.id}.state'))
        OutlinedButton(
          key: ValueKey('subs.pause.${s.id}'),
          onPressed: busy ? null : () => _setState(s, SubscriptionState.paused),
          child: Text(l.adminHookPause),
        ),
      if ((s.state == SubscriptionState.paused || disabled) &&
          !denied.contains('${s.id}.state'))
        OutlinedButton(
          key: ValueKey('subs.resume.${s.id}'),
          onPressed: busy ? null : () => _setState(s, SubscriptionState.active),
          child: Text(l.adminHookResume),
        ),
      OutlinedButton(
        key: ValueKey('subs.deliveries.${s.id}'),
        onPressed: () => _deliveries(s),
        child: Text(l.adminHookDeliveries),
      ),
      if (!denied.contains('${s.id}.delete'))
        OutlinedButton(
          key: ValueKey('subs.delete.${s.id}'),
          onPressed: busy ? null : () => _delete(s),
          child: Text(l.adminHookDelete),
        ),
    ];
    return Padding(
      key: ValueKey('subs.row.${s.id}'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: DsSpace.s12,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Text(
                s.url,
                style: dsTextStyle(DsType.bodyStrong)
                    .copyWith(color: colors.textPrimary),
              ),
              Text(
                subscriptionStateText(l, s.state),
                key: ValueKey('subs.state.${s.id}'),
                style: dsTextStyle(DsType.bodySm).copyWith(
                  color: disabled ? colors.dangerFg : colors.textSecondary,
                ),
              ),
            ],
          ),
          Wrap(
            spacing: DsSpace.s12,
            children: [for (final d in details) Text(d, style: secondary)],
          ),
          if (disabled)
            Text(
              disabledReasonText(l, s, _newest[s.id]),
              key: ValueKey('subs.reason.${s.id}'),
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textPrimary),
            ),
          const SizedBox(height: DsSpace.s4),
          Wrap(spacing: DsSpace.s8, runSpacing: DsSpace.s4, children: actions),
        ],
      ),
    );
  }
}

/// Creates the subscription and answers it, or throws the server's refusal.
typedef _Create = Future<SubscriptionSaved> Function(
  String url,
  List<String> events,
  bool hmac,
);

/// The form "New subscription": an `https` address, the granted event types and HMAC. It runs
/// the call itself, so a refusal of the server (for example a host that is not allowed) is
/// shown in the form and the person can correct the address; pops the answer on success.
class _SubscriptionForm extends StatefulWidget {
  const _SubscriptionForm({
    required this.appName,
    required this.events,
    required this.create,
  });

  final String appName;

  final List<String> events;

  final _Create create;

  @override
  State<_SubscriptionForm> createState() => _SubscriptionFormState();
}

class _SubscriptionFormState extends State<_SubscriptionForm> {
  final TextEditingController _url = TextEditingController();
  final Set<String> _events = {};
  bool _hmac = false;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _url.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _url.dispose();
    super.dispose();
  }

  /// Whether the typed address is an absolute `https` address; the host is the server's check.
  bool get _https {
    final uri = Uri.tryParse(_url.text.trim());
    return uri != null && uri.scheme == 'https' && uri.host.isNotEmpty;
  }

  Future<void> _submit() async {
    final l = AppLocalizations.of(context);
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final saved = await widget.create(_url.text.trim(), [
        for (final e in widget.events)
          if (_events.contains(e)) e,
      ], _hmac);
      if (mounted) Navigator.of(context).pop(saved);
    } on Object catch (e) {
      // A cancelled step-up says nothing; the form stays as it was.
      final failure = adminFailure(l, e);
      if (mounted) setState(() => _error = failure?.text);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final typed = _url.text.trim();
    final canSubmit = !_busy && _https && _events.isNotEmpty;
    return AlertDialog(
      key: const ValueKey('subs.form'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        l.adminHookFormTitle(widget.appName),
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
                l.adminHookFormBody,
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.textSecondary),
              ),
              if (_error case final error?) ...[
                const SizedBox(height: DsSpace.s8),
                DsBanner(tone: DsTone.danger, text: error, liveRegion: true),
              ],
              const SizedBox(height: DsSpace.s16),
              TextField(
                key: const ValueKey('subs.url'),
                controller: _url,
                keyboardType: TextInputType.url,
                decoration: InputDecoration(
                  labelText: l.adminHookUrl,
                  hintText: 'https://',
                  errorText: typed.isEmpty || _https
                      ? null
                      : l.adminHookUrlHttps,
                ),
              ),
              const SizedBox(height: DsSpace.s16),
              Text(
                l.adminHookEventTypes,
                style: dsTextStyle(DsType.bodyStrong)
                    .copyWith(color: colors.textPrimary),
              ),
              if (widget.events.isEmpty)
                Text(
                  l.adminHookNoEvents,
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: colors.textSecondary),
                ),
              for (final e in widget.events)
                CheckboxListTile(
                  key: ValueKey('subs.event.$e'),
                  contentPadding: EdgeInsets.zero,
                  controlAffinity: ListTileControlAffinity.leading,
                  value: _events.contains(e),
                  onChanged: (v) => setState(
                    () => v == true ? _events.add(e) : _events.remove(e),
                  ),
                  title: Text(e),
                ),
              CheckboxListTile(
                key: const ValueKey('subs.hmac'),
                contentPadding: EdgeInsets.zero,
                controlAffinity: ListTileControlAffinity.leading,
                value: _hmac,
                onChanged: (v) => setState(() => _hmac = v ?? false),
                title: Text(l.adminHookHmac),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('subs.cancel'),
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: const ValueKey('subs.submit'),
          onPressed: canSubmit ? _submit : null,
          child: Text(l.adminHookSubmit),
        ),
      ],
    );
  }
}
