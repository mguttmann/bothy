// SPDX-License-Identifier: Apache-2.0

// The instance calls of the fake repository (brief M6-09 Tests), behaving like the server of
// M1-36, M1-46, M1-47 and M3-10: a write names the version it read and loses with the newer
// state against a concurrent change (412), the same value writes nothing, branding keys change
// only through the branding calls, the product name and the accent are checked as the server
// checks them (both field errors of one request together, DECISIONS M1-46/9), a logo is
// deliverable only after the scan, a flag keeps its rollout, a proxy header provider cannot be
// enabled while the listener is not loopback (409), and a provider with linked identities
// cannot be deleted (409 provider_in_use).

import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';

/// The instance calls and whether 05 gives their action step-up
/// (`instance.settings.update`, `instance.branding.update`, `feature_flag.update`,
/// `identity_provider.manage`).
const Map<String, bool> instanceStepUpOf = {
  'putInstanceSetting': true,
  'updateBranding': true,
  'uploadLogo': true,
  'removeLogo': true,
  'putFeatureFlag': true,
  'createIdentityProvider': true,
  'updateIdentityProvider': true,
  'deleteIdentityProvider': true,
};

/// The action of 05 behind each call of [instanceStepUpOf].
const Map<String, String> instanceActionOf = {
  'putInstanceSetting': 'instance.settings.update',
  'updateBranding': 'instance.branding.update',
  'uploadLogo': 'instance.branding.update',
  'removeLogo': 'instance.branding.update',
  'putFeatureFlag': 'feature_flag.update',
  'createIdentityProvider': 'identity_provider.manage',
  'updateIdentityProvider': 'identity_provider.manage',
  'deleteIdentityProvider': 'identity_provider.manage',
};

/// The limit of `branding.product_name_max_chars` in the fake.
const int fakeProductNameMax = 40;

/// The redirect URI of the fake server.
const String fakeRedirectUri =
    'https://chat.example.com/api/v1/auth/oidc/callback';

/// The bytes of the logo preview (a 1x1 PNG).
final Uint8List fakeLogoBytes = Uint8List.fromList(const [
  0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, //
  0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, //
  0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, //
  0x0D, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0xF8, 0xCF, 0xC0, 0xF0, //
  0x1F, 0x00, 0x05, 0x00, 0x01, 0xFF, 0x89, 0x99, 0x3D, 0x1D, 0x00, 0x00, //
  0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
]);

SelfActionFailed _invalid(Map<String, String> fields) => SelfActionFailed(
  ApiProblem(
    status: 400,
    code: 'validation_failed',
    errors: [
      for (final MapEntry(:key, :value) in fields.entries)
        ProblemFieldError(pointer: key, code: value),
    ],
  ),
);

/// Whether [s] holds a control, format or bidi character (SEC-IN-4), as the server's display
/// text check refuses.
bool _forbidden(String s) => s.runes.any(
  (r) =>
      r < 0x20 ||
      (r >= 0x7F && r < 0xA0) ||
      (r >= 0x200B && r <= 0x200F) ||
      (r >= 0x202A && r <= 0x202E) ||
      (r >= 0x2066 && r <= 0x2069),
);

/// The instance calls of the fake; [change] and [refused] are those of the repository.
mixin FakeInstance {
  /// Records the change [op] and asks for the step-up as the server would.
  Future<void> change(String op, String args, String purpose);

  /// Throws the refusal of [op], if any.
  void refused(String op);

  /// The settings by key, as `listInstanceSettings` answers them.
  final Map<String, InstanceSettingEntry> settingsByKey = {
    'branding.accent_color': const InstanceSettingEntry(
      key: 'branding.accent_color',
      value: 'spruce',
      version: 0,
    ),
    'branding.product_name': const InstanceSettingEntry(
      key: 'branding.product_name',
      value: 'Bothy',
      version: 0,
    ),
    'locale.default': const InstanceSettingEntry(
      key: 'locale.default',
      value: 'de',
      version: 0,
    ),
  };

  /// The product name.
  String productName = 'Bothy';

  /// The accent key.
  String accentColor = 'spruce';

  /// The logo file, or null.
  String? logoFileId;

  /// The branding version.
  int brandingVersion = 0;

  /// Whether the logo file passed the scan (`getBrandingLogo` answers it).
  bool logoReady = true;

  /// The flags by key.
  final Map<String, FeatureFlagEntry> flagsByKey = {
    'dm.enabled': const FeatureFlagEntry(
      key: 'dm.enabled',
      kind: FeatureFlagKind.option,
      description: 'Direct messages between people',
      defaultValue: false,
      module: 'chat',
      orphaned: false,
      enabled: false,
      version: 0,
    ),
    'tokens.personal_enabled': const FeatureFlagEntry(
      key: 'tokens.personal_enabled',
      kind: FeatureFlagKind.option,
      description: 'Personal API tokens',
      defaultValue: false,
      module: 'identity',
      orphaned: false,
      enabled: false,
      rolloutUserIds: ['alex'],
      version: 2,
      updatedBy: 'alex',
    ),
    'voice.enabled': const FeatureFlagEntry(
      key: 'voice.enabled',
      kind: FeatureFlagKind.option,
      description: 'Voice messages with transcription',
      defaultValue: false,
      module: 'files',
      orphaned: false,
      enabled: false,
      version: 0,
    ),
  };

  /// The providers by ID.
  final Map<String, IdentityProviderEntry> providersById = {};

  /// Whether the public listener is bound to loopback (08 2.4 rule 1).
  bool loopback = true;

  /// Providers with linked identities (deleting them answers `409 provider_in_use`).
  final Set<String> providersInUse = {};

  /// Runs once before the next write of the call it is keyed by, as another owner's change.
  final Map<String, void Function()> concurrent = {};

  /// The uploads of the logo, by file name.
  final List<String> logoUploads = [];

  /// The bodies of the provider writes, in order.
  final List<Map<String, Object?>> providerBodies = [];

  int _providers = 0;

  void _concurrent(String op) => concurrent.remove(op)?.call();

  /// The flags as evaluated for Alex: the global value, inverted by a rollout naming Alex.
  Map<String, bool> get evaluatedForAlex => {
    for (final f in flagsByKey.values)
      if (!f.orphaned) f.key: f.enabled != f.rolloutUserIds.contains('alex'),
  };

  BrandingState get _branding => BrandingState(
    productName: productName,
    accentColor: accentColor,
    logoFileId: logoFileId,
    version: brandingVersion,
  );

  /// Writes [key] as another owner would (version up, by Robin).
  void otherOwnerSets(String key, Object? value) {
    final s = settingsByKey[key]!;
    settingsByKey[key] = InstanceSettingEntry(
      key: key,
      value: value,
      version: s.version + 1,
      updatedBy: 'robin',
      updatedAt: DateTime.utc(2026, 10, 17, 8, 30),
    );
  }

  /// Sets the branding as another owner would.
  void otherOwnerBrands(String name, String accent) {
    productName = name;
    accentColor = accent;
    brandingVersion += 2;
  }

  /// Sets [key] as another owner would.
  void otherOwnerFlags(String key, bool enabled) {
    final f = flagsByKey[key]!;
    flagsByKey[key] = _flagWith(f, enabled: enabled, by: 'robin');
  }

  FeatureFlagEntry _flagWith(
    FeatureFlagEntry f, {
    required bool enabled,
    required String by,
  }) => FeatureFlagEntry(
    key: f.key,
    kind: f.kind,
    description: f.description,
    defaultValue: f.defaultValue,
    removeBy: f.removeBy,
    module: f.module,
    orphaned: f.orphaned,
    enabled: enabled,
    rolloutUserIds: f.rolloutUserIds,
    rolloutGroupIds: f.rolloutGroupIds,
    version: f.version + 1,
    updatedBy: by,
    updatedAt: DateTime.utc(2026, 10, 17, 9),
  );

  Future<List<InstanceSettingEntry>> instanceSettings() async {
    refused('instanceSettings');
    return [
      for (final k in settingsByKey.keys.toList()..sort()) settingsByKey[k]!,
    ];
  }

  Future<InstanceSettingEntry> putInstanceSetting(
    String key,
    Object? value, {
    required int version,
    required String purpose,
  }) async {
    await change('putInstanceSetting', '$key $value', purpose);
    _concurrent('putInstanceSetting');
    final current = settingsByKey[key];
    if (current == null) {
      throw const SelfActionFailed(ApiProblem(status: 404, code: 'not_found'));
    }
    if (key.startsWith('branding.')) {
      throw _invalid({'#path/key': 'use_branding_endpoint'});
    }
    if (current.version != version) throw InstanceVersionConflict(current);
    if (key == 'locale.default' && value != 'de' && value != 'en') {
      throw _invalid({'/value': 'invalid_enum'});
    }
    if (current.value == value) return current;
    return settingsByKey[key] = InstanceSettingEntry(
      key: key,
      value: value,
      version: current.version + 1,
      updatedBy: 'alex',
      updatedAt: DateTime.utc(2026, 10, 17, 9, 30),
    );
  }

  Future<InstanceOverview> instanceOverview() async {
    refused('instanceOverview');
    return InstanceOverview(branding: _branding, features: evaluatedForAlex);
  }

  Future<BrandingState> updateBranding({
    required String productName,
    required String accentColor,
    required int version,
    required String purpose,
  }) async {
    await change('updateBranding', '$productName $accentColor', purpose);
    _concurrent('updateBranding');
    final name = productName.trim();
    final errors = <String, String>{
      if (name.isEmpty)
        '/product_name': 'required'
      else if (_forbidden(name))
        '/product_name': 'invalid_text'
      else if (name.runes.length > fakeProductNameMax)
        '/product_name': 'too_long',
      if (!accentPaletteKeys.contains(accentColor))
        '/accent_color': 'unknown_color_key',
    };
    if (errors.isNotEmpty) throw _invalid(errors);
    if (version != brandingVersion) throw InstanceVersionConflict(_branding);
    if (name != this.productName) brandingVersion++;
    if (accentColor != this.accentColor) brandingVersion++;
    this.productName = name;
    this.accentColor = accentColor;
    return _branding;
  }

  Future<BrandingState> uploadLogo(
    PickedFile file, {
    required String purpose,
  }) async {
    await change('uploadLogo', file.name, purpose);
    final mime = logoMimeOf(file.name);
    if (mime == 'application/octet-stream') {
      throw const SelfActionFailed(
        ApiProblem(status: 415, code: 'unsupported_media_type'),
      );
    }
    logoUploads.add(file.name);
    logoFileId = 'logo-${logoUploads.length}';
    brandingVersion++;
    return _branding;
  }

  Future<void> removeLogo({required String purpose}) async {
    await change('removeLogo', '', purpose);
    logoFileId = null;
    brandingVersion++;
  }

  Future<Uint8List?> logo() async =>
      logoFileId != null && logoReady ? fakeLogoBytes : null;

  Future<List<FeatureFlagEntry>> featureFlags() async {
    refused('featureFlags');
    return [for (final k in flagsByKey.keys.toList()..sort()) flagsByKey[k]!];
  }

  Future<FeatureFlagEntry> putFeatureFlag(
    FeatureFlagEntry flag, {
    required bool enabled,
    required String reason,
    required String purpose,
  }) async {
    await change('putFeatureFlag', '${flag.key} $enabled $reason', purpose);
    _concurrent('putFeatureFlag');
    final current = flagsByKey[flag.key];
    if (current == null || current.orphaned) {
      throw const SelfActionFailed(ApiProblem(status: 404, code: 'not_found'));
    }
    if (reason.trim().isEmpty) throw _invalid({'/reason': 'required'});
    if (current.version != flag.version) throw InstanceVersionConflict(current);
    if (current.enabled == enabled) return current;
    return flagsByKey[flag.key] = _flagWith(
      current,
      enabled: enabled,
      by: 'alex',
    );
  }

  Future<List<IdentityProviderEntry>> identityProviders() async {
    refused('identityProviders');
    return providersById.values.toList();
  }

  /// The field errors of [config] for [kind] as 08 2.6 checks them (a subset).
  Map<String, String> _configErrors(
    ProviderKind kind,
    Map<String, Object?> config,
  ) {
    String? text(String m) => config[m] is String ? config[m]! as String : null;
    switch (kind) {
      case ProviderKind.oidc:
        final auth = text('client_auth');
        return {
          if (!(text('issuer') ?? '').startsWith('https://'))
            '/config/issuer': text('issuer') == null ? 'required' : 'invalid',
          if (text('client_id') == null) '/config/client_id': 'required',
          if (auth == 'client_secret_basic' &&
              text('client_secret_ref') == null)
            '/config/client_secret_ref': 'required',
          if (auth == 'private_key_jwt' && text('client_key_ref') == null)
            '/config/client_key_ref': 'required',
          if (auth == 'client_secret_basic' &&
              config.containsKey('client_key_ref'))
            '/config/client_key_ref': 'not_allowed',
        };
      case ProviderKind.trustedHeader:
        return {
          if (text('login_header') == null) '/config/login_header': 'required',
        };
      case ProviderKind.local || ProviderKind.unknown:
        return const {};
    }
  }

  void _loopbackRule(ProviderKind kind, bool enabled) {
    if (kind == ProviderKind.trustedHeader && enabled && !loopback) {
      throw const SelfActionFailed(
        ApiProblem(
          status: 409,
          code: 'conflict',
          reason: 'trusted_header_requires_loopback',
          hint: 'Bind server.listen_addr to a loopback address.',
        ),
      );
    }
  }

  Future<IdentityProviderEntry> createIdentityProvider(
    ProviderDraft draft, {
    required String purpose,
  }) async {
    await change('createIdentityProvider', draft.name, purpose);
    providerBodies.add({
      'kind': draft.kind.wire,
      'name': draft.name,
      'config': draft.config,
      'enabled': draft.enabled,
      'allow_provisioning': draft.allowProvisioning,
    });
    final errors = _configErrors(draft.kind, draft.config);
    if (errors.isNotEmpty) throw _invalid(errors);
    _loopbackRule(draft.kind, draft.enabled);
    final id = 'idp-${++_providers}';
    return providersById[id] = IdentityProviderEntry(
      id: id,
      kind: draft.kind,
      name: draft.name,
      config: draft.config,
      enabled: draft.enabled,
      allowProvisioning: draft.allowProvisioning,
    );
  }

  Future<IdentityProviderEntry> updateIdentityProvider(
    String id,
    ProviderChange change, {
    required String purpose,
  }) async {
    await this.change('updateIdentityProvider', id, purpose);
    providerBodies.add({
      'name': ?change.name,
      'config': ?change.config,
      'enabled': ?change.enabled,
      'allow_provisioning': ?change.allowProvisioning,
    });
    final p = providersById[id]!;
    final config = change.config ?? p.config;
    final errors = _configErrors(p.kind, config);
    if (errors.isNotEmpty) throw _invalid(errors);
    _loopbackRule(p.kind, change.enabled ?? false);
    return providersById[id] = IdentityProviderEntry(
      id: id,
      kind: p.kind,
      name: change.name ?? p.name,
      config: config,
      enabled: change.enabled ?? p.enabled,
      allowProvisioning: change.allowProvisioning ?? p.allowProvisioning,
    );
  }

  Future<void> deleteIdentityProvider(
    String id, {
    required String purpose,
  }) async {
    await change('deleteIdentityProvider', id, purpose);
    if (providersInUse.contains(id)) {
      throw const SelfActionFailed(
        ApiProblem(
          status: 409,
          code: 'conflict',
          reason: 'provider_in_use',
          hint: 'disable the provider instead',
        ),
      );
    }
    providersById.remove(id);
  }

  String get oidcRedirectUri => fakeRedirectUri;

  /// Adds a provider as stored by the server.
  IdentityProviderEntry seedProvider(
    ProviderKind kind,
    String name,
    Map<String, Object?> config, {
    bool enabled = true,
  }) {
    final id = 'idp-${++_providers}';
    return providersById[id] = IdentityProviderEntry(
      id: id,
      kind: kind,
      name: name,
      config: config,
      enabled: enabled,
      allowProvisioning: false,
    );
  }
}
