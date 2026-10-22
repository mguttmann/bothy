// SPDX-License-Identifier: Apache-2.0

/// The page of one app (docs/spec/01-product.md 5.2; 11 4.2, 4.3, 7; brief M6-08).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/principal_tokens.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/apps_repository.dart';
import '../secrets/once_only_secret.dart';
import '../self/self_feedback.dart';
import 'app_labels.dart';
import 'lifecycle_dialog.dart';
import 'manifest_picker.dart';
import 'subscriptions_section.dart';
import 'update_dialog.dart';

/// The key under which a refused lifecycle change hides suspend, resume and uninstall: all
/// three are owner actions, so a `403` of one means the others are refused too.
const String _lifecycle = 'lifecycle';

/// The key under which a refused update hides "Update".
const String _update = 'update';

/// One app with its consent, its lifecycle, its tokens and its subscriptions.
///
/// The page loads `getApp` and shows the header (name, version, publisher, state badge), the
/// description, the homepage as text (the server never loads it, 11 3.1, and the page links
/// nothing), the granted scopes with their meaning (05 2.6), the event subscriptions, the
/// destinations of outgoing data, the commands and the publisher's `data_egress` text.
///
/// "Update" ([UpdateAppDialog]) needs the host's file dialog [pickManifest]; "Suspend" or
/// "Resume" follows the state, "Uninstall" is offered until the app is uninstalled; each
/// lifecycle action first shows its effects of 11 4.3 ([AppLifecycleDialog]) and then goes to
/// the server with its purpose, the step-up is the server's. A `403` hides the refused
/// controls (15 1 principle 1). After a change the page reloads and runs [onChanged].
///
/// Below follow the API tokens of the app (an app token is created like an agent token, its
/// DPoP thumbprint optional, Ruling 6; the secret shown once in [OnceOnlySecret] with the keys
/// `appToken.*`) and the [AppSubscriptionsSection]. A refused app shows the server's reason
/// and nothing else.
///
/// Example: `AppPage(repository: admin, appId: id, pickManifest: pick, onBack: back)`.
class AppPage extends StatefulWidget {
  /// Creates the page.
  const AppPage({
    super.key,
    required this.repository,
    required this.appId,
    this.pickManifest,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onBack,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The app.
  final String appId;

  /// The host's file dialog for a manifest, or null when the host has none.
  final ManifestPicker? pickManifest;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Goes back to the list, or null.
  final VoidCallback? onBack;

  /// Runs after every change, or null.
  final VoidCallback? onChanged;

  @override
  State<AppPage> createState() => _AppPageState();
}

class _AppPageState extends State<AppPage> with AdminActionState<AppPage> {
  AppRecord? _app;
  Object? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(AppPage old) {
    super.didUpdateWidget(old);
    if (old.appId != widget.appId) _load();
  }

  Future<void> _load() async {
    try {
      final app = await widget.repository.app(widget.appId);
      if (mounted) {
        setState(() {
          _app = app;
          _error = null;
        });
      }
    } on Object catch (e) {
      if (mounted) {
        setState(() {
          _app = null;
          _error = e;
        });
      }
    }
  }

  Future<void> _change(AppRecord a, AppLifecycleAction action) async {
    final l = AppLocalizations.of(context);
    final name = a.displayName;
    final reason = await AppLifecycleDialog.show(
      context,
      appName: name,
      action: action,
    );
    if (reason == null || !mounted) return;
    final r = widget.repository;
    final ok = await runChange(_lifecycle, () async {
      switch (action) {
        case AppLifecycleAction.suspend:
          await r.suspendApp(a.id, purpose: l.adminAppPurposeSuspend(name));
          return SelfNotice(l.adminAppSuspended(name));
        case AppLifecycleAction.resume:
          await r.resumeApp(a.id, purpose: l.adminAppPurposeResume(name));
          return SelfNotice(l.adminAppResumed(name));
        case AppLifecycleAction.uninstall:
          await r.uninstallApp(
            a.id,
            reason: reason,
            purpose: l.adminAppPurposeUninstall(name),
          );
          return SelfNotice(l.adminAppUninstalled(name));
      }
    });
    if (!ok || !mounted) return;
    widget.onChanged?.call();
    await _load();
  }

  Future<void> _updateApp(AppRecord a, ManifestPicker pick) async {
    final l = AppLocalizations.of(context);
    final changed = await UpdateAppDialog.show(
      context,
      repository: widget.repository,
      app: a,
      pickManifest: pick,
    );
    if (changed != true || !mounted) return;
    widget.onChanged?.call();
    await _load();
    final now = _app;
    if (now != null && mounted) {
      setState(
        () => notice = SelfNotice(
          l.adminAppUpdated(now.displayName, now.manifestVersion),
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final app = _app;
    final error = _error;
    final onBack = widget.onBack;
    final Widget content;
    if (error != null) {
      content = ProblemBanner(problem: loadProblem(error), onRetry: _load);
    } else if (app == null) {
      content = const Center(child: CircularProgressIndicator());
    } else {
      content = _page(context, l, colors, app);
    }
    return Column(
      key: const ValueKey('appPage'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (onBack != null)
          Align(
            alignment: AlignmentDirectional.centerStart,
            child: TextButton(
              key: const ValueKey('appPage.back'),
              onPressed: onBack,
              child: Text(l.adminAppBack),
            ),
          ),
        content,
      ],
    );
  }

  Widget _page(
    BuildContext context,
    AppLocalizations l,
    DsColors colors,
    AppRecord a,
  ) {
    final m = a.manifest;
    final heading = dsTextStyle(DsType.heading)
        .copyWith(color: colors.textPrimary);
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final code = dsTextStyle(DsType.code).copyWith(color: colors.textSecondary);
    final pick = widget.pickManifest;
    final live = a.state != AppInstallState.uninstalled;

    Widget section(String title, List<Widget> children) => Padding(
      padding: const EdgeInsets.only(top: DsSpace.s20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Semantics(header: true, child: Text(title, style: heading)),
          const SizedBox(height: DsSpace.s4),
          if (children.isEmpty)
            Text(l.adminAppConsentNone, style: secondary)
          else
            ...children,
        ],
      ),
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            a.displayName,
            style: dsTextStyle(DsType.title)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s4),
        Wrap(
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s4,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            AppStateBadge(key: const ValueKey('appPage.state'), state: a.state),
            Text(l.adminAppVersion(a.manifestVersion), style: secondary),
            Text(l.adminAppPublisher(m.publisher), style: secondary),
            Text(a.slug, style: code),
          ],
        ),
        const SizedBox(height: DsSpace.s8),
        Text(m.description, style: body),
        if (m.homepageUrl case final url? when url.isNotEmpty)
          // Text only: the server never loads the homepage (11 3.1) and the page links nothing.
          Text(
            l.adminAppHomepage(url),
            key: const ValueKey('appPage.homepage'),
            style: secondary,
          ),
        const SizedBox(height: DsSpace.s12),
        Wrap(
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s8,
          children: [
            if (live && !denied.contains(_update))
              OutlinedButton(
                key: const ValueKey('appPage.update'),
                onPressed: pick == null || busy
                    ? null
                    : () => _updateApp(a, pick),
                child: Text(l.adminAppUpdate),
              ),
            if (!denied.contains(_lifecycle)) ...[
              if (a.state == AppInstallState.active)
                OutlinedButton(
                  key: const ValueKey('appPage.suspend'),
                  onPressed: busy
                      ? null
                      : () => _change(a, AppLifecycleAction.suspend),
                  child: Text(l.adminAppSuspend),
                ),
              if (a.state == AppInstallState.suspended)
                OutlinedButton(
                  key: const ValueKey('appPage.resume'),
                  onPressed: busy
                      ? null
                      : () => _change(a, AppLifecycleAction.resume),
                  child: Text(l.adminAppResume),
                ),
              if (live)
                OutlinedButton(
                  key: const ValueKey('appPage.uninstall'),
                  onPressed: busy
                      ? null
                      : () => _change(a, AppLifecycleAction.uninstall),
                  child: Text(l.adminAppUninstall),
                ),
            ],
          ],
        ),
        if (pick == null && live)
          Text(l.adminAppInstallNoPicker, style: secondary),
        const SizedBox(height: DsSpace.s8),
        noticeBanner(),
        section(l.adminAppConsentScopes, [
          for (final s in a.grantedScopes)
            Padding(
              padding: const EdgeInsets.only(bottom: DsSpace.s4),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(appScopeLabel(l, s), style: body),
                  Text(s, style: code),
                ],
              ),
            ),
        ]),
        section(l.adminAppPageEvents, [
          for (final e in a.eventSubscriptions) Text(e, style: body),
        ]),
        section(l.adminAppConsentDestinations, [
          for (final h in m.targetHosts) Text(h, style: body),
        ]),
        section(l.adminAppConsentCommands, [
          for (final c in m.commands)
            Padding(
              padding: const EdgeInsets.only(bottom: DsSpace.s4),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('/${c.name}', style: code),
                  Text(c.description, style: body),
                ],
              ),
            ),
        ]),
        section(l.adminAppConsentEgress, [
          if (m.dataEgress case final egress? when egress.trim().isNotEmpty)
            Text(egress, style: body)
          else
            Text(l.adminAppConsentNoEgress, style: secondary),
        ]),
        const SizedBox(height: DsSpace.s20),
        PrincipalTokens(
          repository: widget.repository,
          principalId: a.id,
          principalName: a.displayName,
          style: _appTokens,
          canCreate: a.state != AppInstallState.uninstalled,
          timeZone: widget.timeZone,
          clock: widget.clock,
        ),
        const SizedBox(height: DsSpace.s20),
        AppSubscriptionsSection(
          repository: widget.repository,
          app: a,
          timeZone: widget.timeZone,
          clock: widget.clock,
          onChanged: widget.onChanged,
        ),
      ],
    );
  }
}

/// The texts and keys of an app's tokens (Ruling 6): the DPoP thumbprint is optional, because
/// `auth.dpop_required_for` asks it only of agents by default; the secret of a new token is shown
/// once with the keys `appToken.*`.
final PrincipalTokensStyle _appTokens = PrincipalTokensStyle(
  sectionKey: 'appTokens',
  createKey: 'appTokens.create',
  rowKey: 'appToken',
  formKey: 'appTokenForm',
  secretKey: 'appToken',
  dpopRequired: false,
  createLabel: (l) => l.adminAppTokenNew,
  formTitle: (l, name) => l.adminAppTokenCreateTitle(name),
  formBody: (l, _) => l.adminAppTokenBody,
  dpopLabel: (l) => l.adminAppTokenDpop,
  purpose: (l, name) => l.adminAppPurposeToken(name),
  secretTitle: (l, name, _) => l.adminAppTokenSecretTitle(name),
  secretNotice: (l) => l.adminAppTokenSecretOnce,
  revokeBody: (l) => l.adminAppTokenRevokeBody,
);
