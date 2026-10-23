// SPDX-License-Identifier: Apache-2.0

/// The login providers of the instance: list, add, change, disable and delete OIDC, proxy header
/// and local account providers (08 2, 2.6; R-ID-12; brief M6-09 Design note 3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/instance_repository.dart';
import '../data/self_repository.dart' show SelfActionFailed;
import '../self/self_feedback.dart';
import 'instance_common.dart';
import 'provider_editor.dart';

/// The login providers ([InstanceRepository.identityProviders]) with the redirect URI an OIDC
/// provider must register.
///
/// Each row shows name, driver, state and whether unknown identities become accounts; "Change"
/// opens [ProviderEditorDialog], "Disable" and "Delete" first say that the sessions that came
/// from this provider end at once (R-ID-12; deleting fails while identities are linked, the
/// server's hint says to disable instead), "Enable" of a proxy header provider can meet the
/// loopback rule, whose `409` is explained with the server's hint. A provider of a driver this
/// build does not know is listed without "Change". Every change has step-up (05
/// `identity_provider.manage`). "Test configuration" is not offered: the API has no check
/// (Ruling D-M6-09-3).
///
/// Keys: `providers.list`, `providers.add`, `providers.add.<kind>`, `providers.empty`,
/// `providers.redirect.uri`, `providers.redirect.copy`, `provider.<id>`, `provider.<id>.state`,
/// `provider.<id>.edit`, `provider.<id>.toggle`, `provider.<id>.delete`.
class ProvidersSection extends StatefulWidget {
  /// Creates the section.
  const ProvidersSection({super.key, required this.repository, this.onChanged});

  /// The administration's calls.
  final AdminRepository repository;

  /// Runs after a saved change.
  final VoidCallback? onChanged;

  @override
  State<ProvidersSection> createState() => _ProvidersSectionState();
}

class _ProvidersSectionState extends State<ProvidersSection> {
  List<IdentityProviderEntry>? _providers;
  Object? _error;
  SelfNotice? _notice;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final providers = await widget.repository.identityProviders();
      if (!mounted) return;
      setState(() {
        _providers = providers;
        _error = null;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _error = e);
    }
  }

  void _saved(SelfNotice notice) {
    setState(() => _notice = notice);
    widget.onChanged?.call();
  }

  Future<void> _add(ProviderKind kind) async {
    final l = AppLocalizations.of(context);
    final saved = await ProviderEditorDialog.show(
      context,
      repository: widget.repository,
      kind: kind,
    );
    if (saved == null || !mounted) return;
    _saved(SelfNotice(l.instanceProviderAdded(saved.name)));
    await _load();
  }

  Future<void> _edit(IdentityProviderEntry p) async {
    final l = AppLocalizations.of(context);
    final saved = await ProviderEditorDialog.show(
      context,
      repository: widget.repository,
      kind: p.kind,
      provider: p,
    );
    if (saved == null || !mounted) return;
    _saved(SelfNotice(l.instanceProviderSaved(saved.name)));
    await _load();
  }

  Future<void> _toggle(IdentityProviderEntry p) async {
    final l = AppLocalizations.of(context);
    final disable = p.enabled;
    final confirmed = await ConfirmChange.show(
      context,
      title: disable
          ? l.instanceProviderDisableTitle(p.name)
          : l.instanceProviderEnableTitle(p.name),
      body: disable
          ? l.instanceProviderDisableBody(p.name)
          : [
              l.instanceProviderEnableBody(p.name),
              if (p.kind == ProviderKind.trustedHeader)
                l.instanceProviderLoopbackRule,
            ].join('\n'),
      confirmLabel: disable
          ? l.instanceProviderDisable
          : l.instanceProviderEnable,
      destructive: disable,
    );
    if (confirmed == null || !mounted) return;
    await _run(() async {
      await widget.repository.updateIdentityProvider(
        p.id,
        ProviderChange(enabled: !disable),
        purpose: disable
            ? l.stepUpPurposeProviderDisable(p.name)
            : l.stepUpPurposeProviderEnable(p.name),
      );
      return SelfNotice(
        disable
            ? l.instanceProviderDisabled(p.name)
            : l.instanceProviderEnabled(p.name),
      );
    });
  }

  Future<void> _delete(IdentityProviderEntry p) async {
    final l = AppLocalizations.of(context);
    final confirmed = await ConfirmChange.show(
      context,
      title: l.instanceProviderDeleteTitle(p.name),
      body: l.instanceProviderDeleteBody(p.name),
      confirmLabel: l.instanceProviderDelete,
      destructive: true,
    );
    if (confirmed == null || !mounted) return;
    await _run(() async {
      await widget.repository.deleteIdentityProvider(
        p.id,
        purpose: l.stepUpPurposeProviderDelete(p.name),
      );
      return SelfNotice(l.instanceProviderDeleted(p.name));
    });
  }

  /// Runs [change]; a refusal shows the server's reason, with the explanation of the loopback
  /// rule (08 2.4 rule 1) or the server's hint of a conflict (`provider_in_use`).
  Future<void> _run(Future<SelfNotice> Function() change) async {
    final l = AppLocalizations.of(context);
    setState(() {
      _busy = true;
      _notice = null;
    });
    try {
      final notice = await change();
      if (mounted) _saved(notice);
    } on Object catch (e) {
      if (!mounted) return;
      final loopback = loopbackRefusal(e);
      final failure = selfFailure(l, e);
      final hint = e is SelfActionFailed ? e.problem?.hint : null;
      final text = loopback != null
          ? [l.instanceProviderLoopbackRefused, ?loopback.hint].join(' ')
          : failure == null
          ? null
          : [failure.text, ?hint].join(' ');
      if (text != null) {
        setState(() => _notice = SelfNotice(text, tone: DsTone.danger));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    if (mounted) await _load();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final providers = _providers;
    final error = _error;
    return Column(
      key: const ValueKey('providers.list'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          l.instanceProvidersIntro,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textSecondary),
        ),
        const SizedBox(height: DsSpace.s12),
        RedirectUriView(
          uri: widget.repository.oidcRedirectUri,
          keyPrefix: 'providers',
        ),
        const SizedBox(height: DsSpace.s12),
        if (_notice case final n?) selfNoticeBanner(n),
        Align(
          alignment: Alignment.centerLeft,
          child: MenuAnchor(
            builder: (context, menu, _) => OutlinedButton(
              key: const ValueKey('providers.add'),
              onPressed: _busy || providers == null
                  ? null
                  : () => menu.isOpen ? menu.close() : menu.open(),
              child: Text(l.instanceProviderAdd),
            ),
            menuChildren: [
              for (final kind in [
                ProviderKind.oidc,
                ProviderKind.trustedHeader,
                ProviderKind.local,
              ])
                MenuItemButton(
                  key: ValueKey('providers.add.${kind.wire}'),
                  onPressed: () => _add(kind),
                  child: Text(providerKindText(l, kind)),
                ),
            ],
          ),
        ),
        const SizedBox(height: DsSpace.s12),
        if (error != null)
          ProblemBanner(problem: loadProblem(error), onRetry: _load)
        else if (providers == null)
          const Center(child: CircularProgressIndicator())
        else if (providers.isEmpty)
          Text(
            l.instanceProvidersEmpty,
            key: const ValueKey('providers.empty'),
            style: dsTextStyle(DsType.body)
                .copyWith(color: colors.textSecondary),
          )
        else
          for (final p in providers) _row(context, p),
      ],
    );
  }

  Widget _row(BuildContext context, IdentityProviderEntry p) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final issuer = p.config['issuer'];
    final header = p.config['login_header'];
    return Padding(
      key: ValueKey('provider.${p.id}'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Wrap(
        spacing: DsSpace.s12,
        runSpacing: DsSpace.s8,
        alignment: WrapAlignment.spaceBetween,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          ConstrainedBox(
            constraints: const BoxConstraints(minWidth: 280, maxWidth: 520),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  p.name,
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
                Text(
                  [
                    providerKindText(l, p.kind),
                    p.enabled
                        ? l.instanceProviderStateEnabled
                        : l.instanceProviderStateDisabled,
                    if (p.allowProvisioning)
                      l.instanceProviderProvisioningShort,
                  ].join(' · '),
                  key: ValueKey('provider.${p.id}.state'),
                  style: secondary,
                ),
                if (issuer is String) Text(issuer, style: secondary),
                if (header is String)
                  Text(l.instanceProviderHeaderLine(header), style: secondary),
              ],
            ),
          ),
          Wrap(
            spacing: DsSpace.s8,
            runSpacing: DsSpace.s8,
            children: [
              if (p.kind != ProviderKind.unknown)
                OutlinedButton(
                  key: ValueKey('provider.${p.id}.edit'),
                  onPressed: _busy ? null : () => _edit(p),
                  child: Text(l.instanceProviderChange),
                ),
              OutlinedButton(
                key: ValueKey('provider.${p.id}.toggle'),
                onPressed: _busy ? null : () => _toggle(p),
                child: Text(
                  p.enabled
                      ? l.instanceProviderDisable
                      : l.instanceProviderEnable,
                ),
              ),
              OutlinedButton(
                key: ValueKey('provider.${p.id}.delete'),
                onPressed: _busy ? null : () => _delete(p),
                child: Text(l.instanceProviderDelete),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
