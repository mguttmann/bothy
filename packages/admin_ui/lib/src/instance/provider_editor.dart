// SPDX-License-Identifier: Apache-2.0

/// The form of a login provider: per driver the fields of 08 2.6, secrets as write-only
/// references, the redirect URI to register at the provider with a copy action, and the
/// loopback rule of the proxy header driver (brief M6-09 Design note 3; Ruling D-M6-09-3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import '../data/instance_repository.dart';
import '../self/self_feedback.dart';
import 'instance_common.dart';

/// The OIDC client authentication methods of 08 2.6, the preferred first.
const List<String> oidcClientAuthMethods = [
  'private_key_jwt',
  'client_secret_basic',
];

/// The algorithms of the client assertion of 08 2.6, the default first.
const List<String> oidcKeyAlgorithms = ['PS256', 'RS256', 'ES256'];

/// The configuration members of `identity_provider.oidc.v1` that hold a secret reference; the
/// form never shows their stored value (Ruling D-M6-09-3).
const Set<String> oidcSecretMembers = {'client_secret_ref', 'client_key_ref'};

/// The name of the driver [kind].
String providerKindText(AppLocalizations l, ProviderKind kind) =>
    switch (kind) {
      ProviderKind.oidc => l.instanceProviderKindOidc,
      ProviderKind.trustedHeader => l.instanceProviderKindHeader,
      ProviderKind.local => l.instanceProviderKindLocal,
      ProviderKind.unknown => l.instanceProviderKindUnknown,
    };

/// The redirect URI of this server with a copy action (08 2.2 step 4: the one `https`
/// redirect registered at the provider).
///
/// Keys: `<keyPrefix>.redirect.uri`, `<keyPrefix>.redirect.copy`.
class RedirectUriView extends StatefulWidget {
  /// Creates the view of [uri].
  const RedirectUriView({
    super.key,
    required this.uri,
    required this.keyPrefix,
  });

  /// The redirect URI.
  final String uri;

  /// The prefix of the keys.
  final String keyPrefix;

  @override
  State<RedirectUriView> createState() => _RedirectUriViewState();
}

class _RedirectUriViewState extends State<RedirectUriView> {
  bool _copied = false;

  Future<void> _copy() async {
    await Clipboard.setData(ClipboardData(text: widget.uri));
    if (mounted) setState(() => _copied = true);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          l.instanceProviderRedirectLabel,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textSecondary),
        ),
        SelectableText(
          widget.uri,
          key: ValueKey('${widget.keyPrefix}.redirect.uri'),
          style: dsTextStyle(DsType.code).copyWith(color: colors.textPrimary),
        ),
        TextButton(
          key: ValueKey('${widget.keyPrefix}.redirect.copy'),
          onPressed: _copy,
          child: Text(
            _copied
                ? l.instanceProviderRedirectCopied
                : l.instanceProviderRedirectCopy,
          ),
        ),
      ],
    );
  }
}

/// The form of a new provider of [kind] or of the existing [provider].
///
/// OIDC: issuer, client ID, client authentication, the secret reference of the chosen method
/// (`client_secret_ref` or `client_key_ref`, written only: empty keeps the stored reference
/// when editing, the stored reference is never shown), key algorithm, key ID and certificate
/// thumbprint for `private_key_jwt`, allowed tenant IDs, app redirect URIs and the group claim,
/// plus the redirect URI to register at the provider. Proxy header: login header and name
/// header, with the loopback rule (08 2.4 rule 1). Local accounts: no fields. Members of the
/// stored configuration that this form does not know stay as they are.
///
/// "Save" states the change (never a secret) and confirms; the server's `400` stands at the
/// field it names (`/name`, `/config/<member>`), its `409` for the loopback rule is explained
/// with the server's hint. Creating and changing have step-up (05
/// `identity_provider.manage`). Returns the saved provider.
///
/// Keys: `provider.editor`, `provider.name`, `provider.enabled`, `provider.provisioning`,
/// `provider.oidc.<member>` (`issuer`, `clientId`, `clientAuth`, `secretRef`, `keyRef`,
/// `keyAlg`, `keyId`, `x5t`, `tenants`, `appRedirects`, `groupsClaim`),
/// `provider.header.login`, `provider.header.name`, `provider.header.loopback`,
/// `provider.loopbackRefused`, `provider.error`, `provider.save`, `provider.cancel`.
class ProviderEditorDialog extends StatefulWidget {
  /// Creates the form for a new provider of [kind], or for [provider] when it is given.
  const ProviderEditorDialog({
    super.key,
    required this.repository,
    required this.kind,
    this.provider,
  });

  /// The administration's calls.
  final InstanceRepository repository;

  /// The driver of the form.
  final ProviderKind kind;

  /// The provider to change, or null for a new one.
  final IdentityProviderEntry? provider;

  /// Shows the form; returns the saved provider, or null when nothing was saved.
  static Future<IdentityProviderEntry?> show(
    BuildContext context, {
    required InstanceRepository repository,
    required ProviderKind kind,
    IdentityProviderEntry? provider,
  }) => showDialog<IdentityProviderEntry>(
    context: context,
    builder: (_) => ProviderEditorDialog(
      repository: repository,
      kind: kind,
      provider: provider,
    ),
  );

  @override
  State<ProviderEditorDialog> createState() => _ProviderEditorDialogState();
}

class _ProviderEditorDialogState extends State<ProviderEditorDialog> {
  late final Map<String, Object?> _stored = {...?widget.provider?.config};
  late final TextEditingController _name = TextEditingController(
    text: widget.provider?.name ?? '',
  );
  late bool _enabled = widget.provider?.enabled ?? true;
  late bool _provisioning = widget.provider?.allowProvisioning ?? false;

  final Map<String, TextEditingController> _fields = {};
  late String _clientAuth =
      _storedText('client_auth') ?? oidcClientAuthMethods.first;
  late String _keyAlg =
      _storedText('client_key_alg') ?? oidcKeyAlgorithms.first;

  Map<String, String> _fieldErrors = const {};
  String? _failure;
  String? _loopback;
  bool _busy = false;

  bool get _editing => widget.provider != null;

  String? _storedText(String member) {
    final v = _stored[member];
    return v is String ? v : null;
  }

  String _storedList(String member, String separator) {
    final v = _stored[member];
    return v is List ? v.whereType<String>().join(separator) : '';
  }

  /// The controller of the text member [member]; a secret reference starts empty.
  TextEditingController _field(String member) =>
      _fields.putIfAbsent(member, () {
        final text = oidcSecretMembers.contains(member)
            ? ''
            : switch (member) {
                'allowed_tenant_ids' => _storedList(member, ', '),
                'app_redirect_uris' => _storedList(member, '\n'),
                _ => _storedText(member) ?? '',
              };
        return TextEditingController(text: text);
      });

  @override
  void dispose() {
    _name.dispose();
    for (final c in _fields.values) {
      c.dispose();
    }
    super.dispose();
  }

  static List<String> _items(String text, Pattern separator) => [
    for (final part in text.split(separator))
      if (part.trim().isNotEmpty) part.trim(),
  ];

  /// The configuration to send: the stored one (members this form does not know stay) with the
  /// form's members set or removed. An empty secret reference keeps the stored one only while
  /// the client authentication that uses it stays.
  Map<String, Object?> _config() {
    final out = <String, Object?>{..._stored};
    void text(String member) {
      final v = _field(member).text.trim();
      if (v.isEmpty) {
        out.remove(member);
      } else {
        out[member] = v;
      }
    }

    switch (widget.kind) {
      case ProviderKind.oidc:
        text('issuer');
        text('client_id');
        out['client_auth'] = _clientAuth;
        final pkjwt = _clientAuth == 'private_key_jwt';
        final ref = pkjwt ? 'client_key_ref' : 'client_secret_ref';
        final other = pkjwt ? 'client_secret_ref' : 'client_key_ref';
        out.remove(other);
        final typed = _field(ref).text.trim();
        if (typed.isNotEmpty) {
          out[ref] = typed;
        } else if (_storedText('client_auth') != _clientAuth) {
          // A new method needs its own reference; the server names the missing field.
          out.remove(ref);
        }
        if (pkjwt) {
          out['client_key_alg'] = _keyAlg;
          text('client_key_id');
          text('client_cert_x5t_s256');
        } else {
          out
            ..remove('client_key_alg')
            ..remove('client_key_id')
            ..remove('client_cert_x5t_s256');
        }
        final tenants = _items(_field('allowed_tenant_ids').text, ',');
        if (tenants.isEmpty) {
          out.remove('allowed_tenant_ids');
        } else {
          out['allowed_tenant_ids'] = tenants;
        }
        final apps = _items(_field('app_redirect_uris').text, RegExp(r'[\n,]'));
        if (apps.isEmpty) {
          out.remove('app_redirect_uris');
        } else {
          out['app_redirect_uris'] = apps;
        }
        text('groups_claim');
      case ProviderKind.trustedHeader:
        text('login_header');
        text('name_header');
      case ProviderKind.local || ProviderKind.unknown:
        break;
    }
    return out;
  }

  /// The members whose value differs from the stored configuration; a secret is named, never
  /// shown.
  List<String> _changedMembers(Map<String, Object?> config) => [
    for (final key in {..._stored.keys, ...config.keys}.toList()..sort())
      if ('${_stored[key]}' != '${config[key]}') key,
  ];

  String _summary(AppLocalizations l, Map<String, Object?> config) {
    final name = _name.text.trim();
    if (!_editing) {
      return [
        l.instanceProviderCreateSummary(providerKindText(l, widget.kind), name),
        if (config['issuer'] case final String issuer)
          l.instanceProviderIssuerLine(issuer),
        if (config['login_header'] case final String header)
          l.instanceProviderHeaderLine(header),
        _enabled
            ? l.instanceProviderStartsEnabled
            : l.instanceProviderStartsDisabled,
        if (_provisioning) l.instanceProviderProvisioningOn,
      ].join('\n');
    }
    final p = widget.provider!;
    final members = _changedMembers(config);
    return [
      if (name != p.name) l.instanceProviderNameChange(p.name, name),
      if (members.isNotEmpty)
        l.instanceProviderMembersChange(members.join(', ')),
      if (_provisioning != p.allowProvisioning)
        _provisioning
            ? l.instanceProviderProvisioningOn
            : l.instanceProviderProvisioningOff,
    ].join('\n');
  }

  bool _hasChange(Map<String, Object?> config) {
    final p = widget.provider;
    if (p == null) return true;
    return _name.text.trim() != p.name ||
        _provisioning != p.allowProvisioning ||
        _changedMembers(config).isNotEmpty;
  }

  Future<void> _save() async {
    final l = AppLocalizations.of(context);
    final config = _config();
    final confirmed = await ConfirmChange.show(
      context,
      title: _editing
          ? l.instanceProviderEditTitle(widget.provider!.name)
          : l.instanceProviderAddTitle(providerKindText(l, widget.kind)),
      body: _summary(l, config),
      confirmLabel: l.instanceSave,
    );
    if (confirmed == null || !mounted) return;
    setState(() {
      _busy = true;
      _fieldErrors = const {};
      _failure = null;
      _loopback = null;
    });
    try {
      final p = widget.provider;
      final name = _name.text.trim();
      final IdentityProviderEntry saved;
      if (p == null) {
        saved = await widget.repository.createIdentityProvider(
          ProviderDraft(
            kind: widget.kind,
            name: name,
            config: config,
            enabled: _enabled,
            allowProvisioning: _provisioning,
          ),
          purpose: l.stepUpPurposeProviderAdd(name),
        );
      } else {
        saved = await widget.repository.updateIdentityProvider(
          p.id,
          ProviderChange(
            name: name == p.name ? null : name,
            config: _changedMembers(config).isEmpty ? null : config,
            allowProvisioning: _provisioning == p.allowProvisioning
                ? null
                : _provisioning,
          ),
          purpose: l.stepUpPurposeProviderEdit(p.name),
        );
      }
      if (mounted) Navigator.of(context).pop(saved);
    } on Object catch (e) {
      if (!mounted) return;
      final fields = fieldErrorsOf(e);
      final loopback = loopbackRefusal(e);
      setState(() {
        _fieldErrors = {
          for (final MapEntry(:key, :value) in fields.entries)
            key: fieldErrorText(l, value),
        };
        if (loopback != null) {
          _loopback = [
            l.instanceProviderLoopbackRefused,
            ?loopback.hint,
          ].join(' ');
        } else if (fields.isEmpty) {
          _failure = selfFailure(l, e)?.text;
        }
      });
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// The field error at `/config/<member>` or inside it.
  String? _errorAt(String pointer) {
    for (final MapEntry(:key, :value) in _fieldErrors.entries) {
      if (key == pointer || key.startsWith('$pointer/')) return value;
    }
    return null;
  }

  Widget _text(
    String key,
    String member,
    String label, {
    String? helper,
    int maxLines = 1,
  }) => Padding(
    padding: const EdgeInsets.only(bottom: DsSpace.s12),
    child: TextField(
      key: ValueKey(key),
      controller: _field(member),
      enabled: !_busy,
      minLines: 1,
      maxLines: maxLines,
      decoration: InputDecoration(
        labelText: label,
        helperText: helper,
        helperMaxLines: 3,
        errorText: _errorAt('/config/$member'),
        errorMaxLines: 3,
      ),
      onChanged: (_) => setState(() {}),
    ),
  );

  List<Widget> _oidcFields(AppLocalizations l) {
    final pkjwt = _clientAuth == 'private_key_jwt';
    final ref = pkjwt ? 'client_key_ref' : 'client_secret_ref';
    final keepHint = _editing && _storedText('client_auth') == _clientAuth
        ? l.instanceProviderSecretKeep
        : null;
    return [
      _text(
        'provider.oidc.issuer',
        'issuer',
        l.instanceProviderIssuer,
        helper: l.instanceProviderIssuerHelp,
      ),
      _text('provider.oidc.clientId', 'client_id', l.instanceProviderClientId),
      Padding(
        padding: const EdgeInsets.only(bottom: DsSpace.s12),
        child: DropdownButtonFormField<String>(
          key: const ValueKey('provider.oidc.clientAuth'),
          initialValue: _clientAuth,
          decoration: InputDecoration(
            labelText: l.instanceProviderClientAuth,
            errorText: _errorAt('/config/client_auth'),
          ),
          items: [
            for (final m in oidcClientAuthMethods)
              DropdownMenuItem(
                key: ValueKey('provider.oidc.clientAuth.$m'),
                value: m,
                child: Text(m),
              ),
          ],
          onChanged: _busy
              ? null
              : (v) => setState(() => _clientAuth = v ?? _clientAuth),
        ),
      ),
      Padding(
        padding: const EdgeInsets.only(bottom: DsSpace.s12),
        child: TextField(
          key: ValueKey(
            pkjwt ? 'provider.oidc.keyRef' : 'provider.oidc.secretRef',
          ),
          controller: _field(ref),
          enabled: !_busy,
          // A reference names where the secret lives; it is still never shown back.
          obscureText: true,
          enableSuggestions: false,
          autocorrect: false,
          decoration: InputDecoration(
            labelText: pkjwt
                ? l.instanceProviderKeyRef
                : l.instanceProviderSecretRef,
            helperText: [l.instanceProviderSecretRefHelp, ?keepHint].join(' '),
            helperMaxLines: 4,
            errorText: _errorAt('/config/$ref'),
            errorMaxLines: 3,
          ),
          onChanged: (_) => setState(() {}),
        ),
      ),
      if (pkjwt) ...[
        Padding(
          padding: const EdgeInsets.only(bottom: DsSpace.s12),
          child: DropdownButtonFormField<String>(
            key: const ValueKey('provider.oidc.keyAlg'),
            initialValue: _keyAlg,
            decoration: InputDecoration(
              labelText: l.instanceProviderKeyAlg,
              errorText: _errorAt('/config/client_key_alg'),
            ),
            items: [
              for (final a in oidcKeyAlgorithms)
                DropdownMenuItem(value: a, child: Text(a)),
            ],
            onChanged: _busy
                ? null
                : (v) => setState(() => _keyAlg = v ?? _keyAlg),
          ),
        ),
        _text('provider.oidc.keyId', 'client_key_id', l.instanceProviderKeyId),
        _text(
          'provider.oidc.x5t',
          'client_cert_x5t_s256',
          l.instanceProviderX5t,
        ),
      ],
      _text(
        'provider.oidc.tenants',
        'allowed_tenant_ids',
        l.instanceProviderTenants,
        helper: l.instanceProviderTenantsHelp('{tenantid}'),
      ),
      _text(
        'provider.oidc.appRedirects',
        'app_redirect_uris',
        l.instanceProviderAppRedirects,
        helper: l.instanceProviderAppRedirectsHelp,
        maxLines: 4,
      ),
      _text(
        'provider.oidc.groupsClaim',
        'groups_claim',
        l.instanceProviderGroupsClaim,
        helper: l.instanceProviderGroupsClaimHelp,
      ),
      RedirectUriView(
        uri: widget.repository.oidcRedirectUri,
        keyPrefix: 'provider',
      ),
    ];
  }

  List<Widget> _headerFields(AppLocalizations l) => [
    Padding(
      padding: const EdgeInsets.only(bottom: DsSpace.s12),
      child: DsBanner(
        key: const ValueKey('provider.header.loopback'),
        tone: DsTone.info,
        text: l.instanceProviderLoopbackRule,
      ),
    ),
    _text(
      'provider.header.login',
      'login_header',
      l.instanceProviderLoginHeader,
      helper: l.instanceProviderLoginHeaderHelp,
    ),
    _text('provider.header.name', 'name_header', l.instanceProviderNameHeader),
  ];

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final config = _config();
    final canSave =
        !_busy && _name.text.trim().isNotEmpty && _hasChange(config);
    // While a write runs the dialog stays, so its result always reaches the list.
    return PopScope(
      canPop: !_busy,
      child: AlertDialog(
        key: const ValueKey('provider.editor'),
        backgroundColor: colors.surfaceRaised,
        title: Text(
          _editing
              ? l.instanceProviderEditTitle(widget.provider!.name)
              : l.instanceProviderAddTitle(providerKindText(l, widget.kind)),
          style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
        ),
        content: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 560),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (_loopback case final text?)
                  Padding(
                    padding: const EdgeInsets.only(bottom: DsSpace.s12),
                    child: DsBanner(
                      key: const ValueKey('provider.loopbackRefused'),
                      tone: DsTone.danger,
                      text: text,
                      liveRegion: true,
                    ),
                  ),
                if (_failure case final text?)
                  Padding(
                    padding: const EdgeInsets.only(bottom: DsSpace.s12),
                    child: DsBanner(
                      key: const ValueKey('provider.error'),
                      tone: DsTone.danger,
                      text: text,
                      liveRegion: true,
                    ),
                  ),
                Padding(
                  padding: const EdgeInsets.only(bottom: DsSpace.s12),
                  child: TextField(
                    key: const ValueKey('provider.name'),
                    controller: _name,
                    enabled: !_busy,
                    decoration: InputDecoration(
                      labelText: l.instanceProviderName,
                      helperText: l.instanceProviderNameHelp,
                      errorText: _errorAt('/name'),
                    ),
                    onChanged: (_) => setState(() {}),
                  ),
                ),
                ...switch (widget.kind) {
                  ProviderKind.oidc => _oidcFields(l),
                  ProviderKind.trustedHeader => _headerFields(l),
                  ProviderKind.local || ProviderKind.unknown => [
                    Padding(
                      padding: const EdgeInsets.only(bottom: DsSpace.s12),
                      child: Text(
                        l.instanceProviderLocalNote,
                        style: dsTextStyle(DsType.bodySm)
                            .copyWith(color: colors.textSecondary),
                      ),
                    ),
                  ],
                },
                if (!_editing)
                  SwitchListTile(
                    key: const ValueKey('provider.enabled'),
                    contentPadding: EdgeInsets.zero,
                    value: _enabled,
                    onChanged: _busy
                        ? null
                        : (v) => setState(() => _enabled = v),
                    title: Text(l.instanceProviderEnableNow),
                  ),
                SwitchListTile(
                  key: const ValueKey('provider.provisioning'),
                  contentPadding: EdgeInsets.zero,
                  value: _provisioning,
                  onChanged: _busy
                      ? null
                      : (v) => setState(() => _provisioning = v),
                  title: Text(l.instanceProviderProvisioning),
                  subtitle: Text(l.instanceProviderProvisioningHelp),
                ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            key: const ValueKey('provider.cancel'),
            onPressed: _busy ? null : () => Navigator.of(context).pop(),
            child: Text(l.selfCancel),
          ),
          FilledButton(
            key: const ValueKey('provider.save'),
            onPressed: canSave ? _save : null,
            child: Text(l.instanceSave),
          ),
        ],
      ),
    );
  }
}
