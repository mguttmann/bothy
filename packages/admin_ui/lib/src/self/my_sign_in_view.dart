// SPDX-License-Identifier: Apache-2.0

/// The own sign-in methods: the password with its one-time code, the passkeys and the linked
/// login services, and their setup (docs/spec/01-product.md 5.1 D8 and 5.5; 08 2.5 and 3.3;
/// design note docs/design/sign-in-methods.md).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/sign_in_repository.dart';
import 'confirm_device_dialog.dart' show CodeDialog;
import 'device_tile.dart' show activityText;
import 'password_dialog.dart';
import 'self_feedback.dart';

/// A login service a further identity can be linked with (`auth_providers` of discovery with
/// kind `oidc`).
final class SignInProvider {
  /// Creates the provider.
  const SignInProvider({required this.id, required this.name});

  /// `auth_providers[].id`.
  final String id;

  /// The name the button shows.
  final String name;
}

/// How linking an identity ended, as the platform reports it.
enum IdentityLinkOutcome {
  /// The identity is linked.
  linked,

  /// The person cancelled at the login service.
  cancelled,

  /// The login service or the server refused, or the flow expired.
  failed,

  /// The page left for the login service and comes back on its own (the console); nothing to
  /// show now.
  redirected,
}

/// Links a further identity at [provider]: the login flow in the mode "link" (08 3.3). Throws
/// what [SignInRepository] throws for an answer of the server (`SelfActionFailed`) or the login
/// layer (a cancelled step-up).
typedef IdentityLinker = Future<IdentityLinkOutcome> Function(
  SignInProvider provider,
);

/// The area "Sign-in" of the settings and of the console.
///
/// It lists what `getMyCredentials` returns and offers what the platform can do: "Set password"
/// (with the one-time code; only for a person with a local identity when [passwordEnabled]),
/// "Add passkey" when [createPasskey] is given (else [noPasskeyHint] when given) and "Link with"
/// each of [providers] that is not linked yet when [linkIdentity] is given. The step-up of each
/// change is asked by the step-up of the client when the server answers `401
/// step_up_required`, with the purpose text this view passes. After every change the list loads
/// again and [onChanged] runs.
///
/// Example:
/// ```dart
/// MySignInView(repository: ApiSignInRepository(api), timeZone: 'Europe/Berlin');
/// ```
class MySignInView extends StatefulWidget {
  /// Creates the view.
  const MySignInView({
    super.key,
    required this.repository,
    required this.timeZone,
    this.clock = DateTime.now,
    this.passwordEnabled = false,
    this.createPasskey,
    this.noPasskeyHint,
    this.providers = const [],
    this.linkIdentity,
    this.initialNotice,
    this.onChanged,
  });

  /// The own credentials and their setup.
  final SignInRepository repository;

  /// The time zone of the person (`users.time_zone`).
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Whether the instance has local passwords (`local` driver and `auth.local_password_enabled`).
  final bool passwordEnabled;

  /// Creates a passkey on this platform, or null where it cannot.
  final PasskeyCreator? createPasskey;

  /// What the passkey part says when [createPasskey] is null, or null for nothing.
  final String? noPasskeyHint;

  /// The login services a further identity can be linked with.
  final List<SignInProvider> providers;

  /// Links an identity on this platform, or null where it cannot.
  final IdentityLinker? linkIdentity;

  /// A notice to show first (the console's result of a link that came back by redirect).
  final SelfNotice? initialNotice;

  /// Runs after a change.
  final VoidCallback? onChanged;

  @override
  State<MySignInView> createState() => _MySignInViewState();
}

class _MySignInViewState extends State<MySignInView> {
  SelfCredentials? _credentials;
  bool _loadFailed = false;
  bool _busy = false;
  SelfNotice? _notice;
  int _loads = 0;

  @override
  void initState() {
    super.initState();
    _notice = widget.initialNotice;
    _load();
  }

  /// Loads the overview; only the answer of the latest load is shown, so an older answer that
  /// arrives late does not replace a newer one.
  Future<void> _load() async {
    final load = ++_loads;
    try {
      final c = await widget.repository.credentials();
      if (!mounted || load != _loads) return;
      setState(() {
        _credentials = c;
        _loadFailed = false;
      });
    } on Object {
      if (!mounted || load != _loads) return;
      setState(() => _loadFailed = true);
    }
  }

  /// Runs [action] (which returns the notice to show, null for none), then loads again.
  Future<void> _run(Future<SelfNotice?> Function() action) async {
    final l = AppLocalizations.of(context);
    setState(() => _busy = true);
    try {
      final notice = await action();
      if (!mounted) return;
      setState(() => _notice = notice);
      widget.onChanged?.call();
    } on Object catch (e) {
      final failure = selfFailure(l, e);
      if (!mounted) return;
      if (failure != null) {
        setState(() => _notice = SelfNotice(failure.text, tone: DsTone.danger));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    if (mounted) await _load();
  }

  Future<void> _password(PasswordDialogMode mode) async {
    final notice = await PasswordDialog.show(
      context,
      repository: widget.repository,
      mode: mode,
    );
    if (!mounted) return;
    // A cancel after the first step leaves a saved password with an unconfirmed code, so the
    // overview loads again after every close.
    if (notice != null) {
      setState(() => _notice = notice);
      widget.onChanged?.call();
    }
    await _load();
  }

  Future<void> _addPasskey(PasskeyCreator create) async {
    final l = AppLocalizations.of(context);
    final name = await _PasskeyNameDialog.show(context);
    if (name == null || !mounted) return;
    await _run(() async {
      final added = await widget.repository.addPasskey(
        name,
        create: create,
        purpose: l.stepUpPurposeAddPasskey,
      );
      return added == null
          ? SelfNotice(l.signInPasskeyCancelled, tone: DsTone.warning)
          : SelfNotice(l.signInPasskeyAdded(added.name));
    });
  }

  Future<void> _link(IdentityLinker link, SignInProvider provider) async {
    final l = AppLocalizations.of(context);
    await _run(() async {
      final outcome = await link(provider);
      return switch (outcome) {
        IdentityLinkOutcome.linked => SelfNotice(l.signInLinked(provider.name)),
        IdentityLinkOutcome.cancelled => SelfNotice(
          l.signInLinkCancelled,
          tone: DsTone.warning,
        ),
        IdentityLinkOutcome.failed => SelfNotice(
          l.signInLinkFailed,
          tone: DsTone.danger,
        ),
        IdentityLinkOutcome.redirected => null,
      };
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final c = _credentials;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final strong = dsTextStyle(DsType.bodyStrong)
        .copyWith(color: colors.textPrimary);
    final now = widget.clock();
    Widget heading(String text) => Padding(
      padding: const EdgeInsets.only(top: DsSpace.s16, bottom: DsSpace.s8),
      child: Semantics(
        header: true,
        child: Text(
          text,
          style: dsTextStyle(DsType.heading)
              .copyWith(color: colors.textPrimary),
        ),
      ),
    );
    Widget row(Key key, String title, String detail) => Padding(
      key: key,
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
      child: MergeSemantics(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title, style: strong),
            Text(detail, style: secondary),
          ],
        ),
      ),
    );
    String when(DateTime at) =>
        activityText(context, at, now: now, timeZone: widget.timeZone);
    Widget buttons(List<Widget> children) => Padding(
      padding: const EdgeInsets.only(top: DsSpace.s8),
      child: Wrap(
        spacing: DsSpace.s8,
        runSpacing: DsSpace.s8,
        children: children,
      ),
    );
    final create = widget.createPasskey;
    final link = widget.linkIdentity;
    return Column(
      key: const ValueKey('signIn'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (_notice case final notice?) selfNoticeBanner(notice),
        if (_loadFailed && c == null)
          DsBanner(
            key: const ValueKey('signIn.loadFailed'),
            tone: DsTone.warning,
            text: l.selfLoadFailed,
            actionLabel: l.selfRetry,
            onAction: _load,
          )
        else if (c != null) ...[
          if (widget.passwordEnabled && c.hasLocalIdentity) ...[
            heading(l.signInPasswordTitle),
            Text(
              c.passwordSet
                  ? l.adminCredentialsPasswordSet
                  : l.adminCredentialsPasswordNone,
              key: const ValueKey('signIn.password.state'),
              style: secondary,
            ),
            Text(
              c.totpEnrolled
                  ? l.adminCredentialsTotpSet
                  : l.adminCredentialsTotpNone,
              key: const ValueKey('signIn.totp.state'),
              style: secondary,
            ),
            buttons([
              if (!c.passwordSet)
                FilledButton(
                  key: const ValueKey('signIn.password.set'),
                  onPressed: _busy
                      ? null
                      : () => _password(PasswordDialogMode.setUp),
                  child: Text(l.signInSetPassword),
                )
              else ...[
                if (!c.totpEnrolled)
                  FilledButton(
                    key: const ValueKey('signIn.totp.finish'),
                    onPressed: _busy
                        ? null
                        : () => _password(PasswordDialogMode.newCode),
                    child: Text(l.signInFinishTotp),
                  ),
                OutlinedButton(
                  key: const ValueKey('signIn.password.change'),
                  onPressed: _busy
                      ? null
                      : () => _password(PasswordDialogMode.changePassword),
                  child: Text(l.signInChangePassword),
                ),
                if (c.totpEnrolled)
                  OutlinedButton(
                    key: const ValueKey('signIn.totp.new'),
                    onPressed: _busy
                        ? null
                        : () => _password(PasswordDialogMode.newCode),
                    child: Text(l.signInNewTotp),
                  ),
              ],
            ]),
          ],
          heading(l.adminCredentialsPasskeys),
          if (c.passkeys.isEmpty)
            Text(
              l.adminCredentialsNone,
              key: const ValueKey('signIn.passkeys.none'),
              style: secondary,
            ),
          for (final p in c.passkeys)
            row(
              ValueKey('signIn.passkey.${p.id}'),
              p.name,
              l.adminPasskeyAdded(when(p.createdAt)),
            ),
          if (create != null)
            buttons([
              FilledButton(
                key: const ValueKey('signIn.passkey.add'),
                onPressed: _busy ? null : () => _addPasskey(create),
                child: Text(l.signInAddPasskey),
              ),
            ])
          else if (widget.noPasskeyHint case final hint?)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s8),
              child: Text(
                hint,
                key: const ValueKey('signIn.passkey.unavailable'),
                style: secondary,
              ),
            ),
          heading(l.adminCredentialsIdentities),
          for (final i in c.identities)
            row(
              ValueKey('signIn.identity.${i.id}'),
              _providerName(i),
              '${i.subject} · ${l.signInLinkedAt(when(i.linkedAt))}',
            ),
          if (link != null)
            buttons([
              for (final p in widget.providers)
                if (!c.identities.any((i) => i.providerId == p.id))
                  OutlinedButton(
                    key: ValueKey('signIn.link.${p.id}'),
                    onPressed: _busy ? null : () => _link(link, p),
                    child: Text(l.signInLinkWith(p.name)),
                  ),
            ]),
        ],
      ],
    );
  }

  /// The name of the login service of [identity]: the provider's name when known, else its
  /// issuer or its kind.
  String _providerName(SelfIdentity identity) {
    for (final p in widget.providers) {
      if (p.id == identity.providerId) return p.name;
    }
    return identity.issuer ?? identity.providerKind;
  }
}

/// Asks for the name of a new passkey; pops the trimmed name or null.
class _PasskeyNameDialog extends StatefulWidget {
  const _PasskeyNameDialog();

  static Future<String?> show(BuildContext context) => showDialog<String>(
    context: context,
    builder: (_) => const _PasskeyNameDialog(),
  );

  @override
  State<_PasskeyNameDialog> createState() => _PasskeyNameDialogState();
}

class _PasskeyNameDialogState extends State<_PasskeyNameDialog> {
  final _name = TextEditingController();

  @override
  void initState() {
    super.initState();
    _name.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  void _submit() {
    final name = _name.text.trim();
    if (name.isEmpty) return;
    Navigator.of(context).pop(name);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return CodeDialog(
      dialogKey: const ValueKey('signIn.passkey.dialog'),
      title: l.signInPasskeyNameTitle,
      hint: l.signInPasskeyNameHint,
      field: TextField(
        key: const ValueKey('signIn.passkey.name'),
        controller: _name,
        autofocus: true,
        decoration: InputDecoration(labelText: l.signInPasskeyName),
        onSubmitted: (_) => _submit(),
      ),
      submitKey: const ValueKey('signIn.passkey.submit'),
      submitLabel: l.signInPasskeyContinue,
      onSubmit: _name.text.trim().isEmpty ? null : _submit,
    );
  }
}
