// SPDX-License-Identifier: Apache-2.0

/// The instance settings, the branding, the feature flags and the login providers over REST
/// (docs/spec/01-product.md 5.5 "Instance, branding" and 12; docs/operations/configuration.md
/// 6 and 7; 08 2 and 2.6; brief M6-09 Design notes 1 to 4).
///
/// The types carry what the server answers. Whether a value is allowed is the server's answer
/// (`400 validation_failed` with its field errors, `409 conflict` for the loopback rule of the
/// proxy header driver), never a check of the client (15 1 principle 1). A write that carries a
/// version and loses against a concurrent change of another owner ends in
/// [InstanceVersionConflict] with the newer state, so the screen can show it next to the
/// person's input and ask again.
library;

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';

import '../apps/manifest_picker.dart' show PickedFile;
import 'access_repository.dart' show adminCall;
import 'self_repository.dart';

/// One instance setting (`InstanceSetting`, configuration reference 6): its [key], its JSON
/// [value], the [version] that a write must name, and who changed it when.
final class InstanceSettingEntry {
  /// Creates the setting.
  const InstanceSettingEntry({
    required this.key,
    required this.value,
    required this.version,
    this.updatedBy,
    this.updatedAt,
  });

  /// The key, for example `locale.default`.
  final String key;

  /// The value as decoded JSON (a string for every key of this version).
  final Object? value;

  /// The version; `0` for a key that was never written (DECISIONS M1-46/3).
  final int version;

  /// The person who wrote the value last, or null for the default.
  final String? updatedBy;

  /// When the value was written last, or null for the default.
  final DateTime? updatedAt;
}

/// The branding of the instance (`Branding`, 01 12; ADR 0044).
final class BrandingState {
  /// Creates the branding.
  const BrandingState({
    required this.productName,
    required this.accentColor,
    required this.logoFileId,
    required this.version,
  });

  /// The displayed product name.
  final String productName;

  /// The accent key, for example `spruce`, or null when the server sends none.
  final String? accentColor;

  /// The file of the logo, or null without a logo.
  final String? logoFileId;

  /// The branding version that `updateBranding` must name in `If-Match`.
  final int version;
}

/// What `getInstance` tells the signed-in person: the branding and the evaluated feature flags.
final class InstanceOverview {
  /// Creates the overview.
  const InstanceOverview({required this.branding, required this.features});

  /// The branding.
  final BrandingState branding;

  /// The flags as evaluated for the caller, by key.
  final Map<String, bool> features;
}

/// The kind of a feature flag (16 6.2).
enum FeatureFlagKind {
  /// A switch of a release in progress, removed by a later version.
  release,

  /// An option the owner keeps.
  option,

  /// Absent (an orphaned key) or a kind of a newer server.
  unknown,
}

/// One feature flag (`FeatureFlag`, configuration reference 7; 16 6.2).
final class FeatureFlagEntry {
  /// Creates the flag.
  const FeatureFlagEntry({
    required this.key,
    required this.kind,
    this.description,
    this.defaultValue,
    this.removeBy,
    this.module,
    required this.orphaned,
    required this.enabled,
    this.rolloutUserIds = const [],
    this.rolloutGroupIds = const [],
    required this.version,
    this.updatedBy,
    this.updatedAt,
  });

  /// The key, for example `dm.enabled`.
  final String key;

  /// The kind.
  final FeatureFlagKind kind;

  /// The description of the registration (English), or null for an orphaned key.
  final String? description;

  /// The registered default, or null for an orphaned key.
  final bool? defaultValue;

  /// The server version that removes a `release` flag, or null.
  final String? removeBy;

  /// The module that owns the flag, or null.
  final String? module;

  /// Whether the key is stored but no longer registered; such a flag is off and cannot be set.
  final bool orphaned;

  /// The stored global value, or the default without a row.
  final bool enabled;

  /// The people for whom the global value is inverted.
  final List<String> rolloutUserIds;

  /// The groups whose members get the global value inverted.
  final List<String> rolloutGroupIds;

  /// The version that a write names in `If-Match`.
  final int version;

  /// The person who set the flag last, or null.
  final String? updatedBy;

  /// When the flag was set last, or null.
  final DateTime? updatedAt;
}

/// The driver of a login provider (08 2.1).
enum ProviderKind {
  /// Any OpenID Connect provider.
  oidc('oidc'),

  /// A reverse proxy that sets the identity in a header (08 2.4).
  trustedHeader('trusted_header'),

  /// Local accounts with password and TOTP or passkey (08 2.5).
  local('local'),

  /// A driver of a newer server; shown, never edited.
  unknown('');

  const ProviderKind(this.wire);

  /// The value of `IdentityProvider.kind`.
  final String wire;

  /// The kind of [wire]; [unknown] for any other value.
  static ProviderKind of(String wire) => values.firstWhere(
    (k) => k.wire == wire && k != unknown,
    orElse: () => unknown,
  );
}

/// One login provider (`IdentityProvider`, 08 2.6). The [config] holds the normalized
/// configuration the server stored; secrets appear in it only as references to the secret
/// provider (08 2.6), never as values.
final class IdentityProviderEntry {
  /// Creates the provider.
  const IdentityProviderEntry({
    required this.id,
    required this.kind,
    required this.name,
    required this.config,
    required this.enabled,
    required this.allowProvisioning,
    this.updatedAt,
  });

  /// The ID.
  final String id;

  /// The driver.
  final ProviderKind kind;

  /// The name on the login button.
  final String name;

  /// The configuration of the driver (`identity_provider.<kind>.v1`).
  final Map<String, Object?> config;

  /// Whether the provider offers its login.
  final bool enabled;

  /// Whether an unknown identity becomes an account without an enrollment code (08 3.1).
  final bool allowProvisioning;

  /// When the provider was changed last, or null.
  final DateTime? updatedAt;
}

/// The members of a new login provider (`IdentityProviderCreate`).
final class ProviderDraft {
  /// Creates the draft.
  const ProviderDraft({
    required this.kind,
    required this.name,
    required this.config,
    required this.enabled,
    required this.allowProvisioning,
  });

  /// The driver; never [ProviderKind.unknown].
  final ProviderKind kind;

  /// The name.
  final String name;

  /// The configuration of the driver.
  final Map<String, Object?> config;

  /// Whether the provider starts enabled.
  final bool enabled;

  /// Whether unknown identities become accounts.
  final bool allowProvisioning;
}

/// A change of a login provider (`IdentityProviderUpdate`): a null member stays as it is; a
/// non-null [config] replaces the whole configuration.
final class ProviderChange {
  /// Creates the change.
  const ProviderChange({
    this.name,
    this.config,
    this.enabled,
    this.allowProvisioning,
  });

  /// The new name, or null.
  final String? name;

  /// The new configuration, or null.
  final Map<String, Object?>? config;

  /// The new state, or null.
  final bool? enabled;

  /// The new provisioning rule, or null.
  final bool? allowProvisioning;

  /// Whether the change sets nothing.
  bool get isEmpty =>
      name == null &&
      config == null &&
      enabled == null &&
      allowProvisioning == null;
}

/// A write that named an outdated version (`412 version_conflict`, 06 8): another owner changed
/// the same thing in the meantime. [current] is the newer state, from the problem's `current` or
/// read again when the problem has none.
final class InstanceVersionConflict<T> implements Exception {
  /// Creates the conflict.
  const InstanceVersionConflict(this.current);

  /// The newer state.
  final T current;

  @override
  String toString() => 'InstanceVersionConflict($current)';
}

/// The instance administration: settings, branding, feature flags and login providers.
///
/// Every changing call carries the purpose text the step-up dialog shows when the server asks
/// for a step-up (`401 step_up_required`); 05 gives `instance.settings.update`,
/// `instance.branding.update`, `feature_flag.update` and `identity_provider.manage` step-up, the
/// reading actions none.
///
/// Errors: [InstanceVersionConflict] for a write that lost against a concurrent change,
/// [SelfActionFailed] for any other answer of the server or no answer, [StepUpCancelled] when
/// the person cancels the step-up.
abstract interface class InstanceRepository {
  /// Every instance setting the server lists (`listInstanceSettings`).
  Future<List<InstanceSettingEntry>> instanceSettings();

  /// Sets [key] to the JSON [value] if the setting still has [version] (`putInstanceSetting`
  /// with `If-Match`).
  Future<InstanceSettingEntry> putInstanceSetting(
    String key,
    Object? value, {
    required int version,
    required String purpose,
  });

  /// The branding and the evaluated flags of the caller (`getInstance`).
  Future<InstanceOverview> instanceOverview();

  /// Sets product name and accent if the branding still has [version] (`updateBranding`).
  Future<BrandingState> updateBranding({
    required String productName,
    required String accentColor,
    required int version,
    required String purpose,
  });

  /// Uploads [file] as the logo (`uploadBrandingLogo`); last write wins (06 8).
  Future<BrandingState> uploadLogo(PickedFile file, {required String purpose});

  /// Removes the logo (`deleteBrandingLogo`).
  Future<void> removeLogo({required String purpose});

  /// The server's preview of the logo (`getBrandingLogo`, the variant `medium`), or null while
  /// there is none or it is not deliverable yet (`404`).
  Future<Uint8List?> logo();

  /// Every feature flag (`listFeatureFlags`): the registered ones, then the orphaned ones.
  Future<List<FeatureFlagEntry>> featureFlags();

  /// Sets the global value of [flag] with [reason] if it still has its version
  /// (`putFeatureFlag` with `If-Match`); the rollout stays as it is.
  Future<FeatureFlagEntry> putFeatureFlag(
    FeatureFlagEntry flag, {
    required bool enabled,
    required String reason,
    required String purpose,
  });

  /// Every login provider (`listIdentityProviders`).
  Future<List<IdentityProviderEntry>> identityProviders();

  /// Adds a login provider (`createIdentityProvider`).
  Future<IdentityProviderEntry> createIdentityProvider(
    ProviderDraft draft, {
    required String purpose,
  });

  /// Changes the provider [id] (`updateIdentityProvider`); disabling it ends the sessions of
  /// its identities (08 3.3, R-ID-12).
  Future<IdentityProviderEntry> updateIdentityProvider(
    String id,
    ProviderChange change, {
    required String purpose,
  });

  /// Removes the provider [id] (`deleteIdentityProvider`); `409 conflict` with reason
  /// `provider_in_use` while identities are linked.
  Future<void> deleteIdentityProvider(String id, {required String purpose});

  /// The redirect URI of this server that an OIDC provider must register
  /// (`<server>/api/v1/auth/oidc/callback`, 08 2.2 step 4).
  String get oidcRedirectUri;
}

/// The path of the OIDC callback below the API base (08 2.2 step 4).
const String oidcCallbackPath = '/auth/oidc/callback';

/// [entry] of an `InstanceSetting` JSON object.
InstanceSettingEntry instanceSettingOfJson(Map<String, Object?> json) =>
    InstanceSettingEntry(
      key: json['key']! as String,
      value: json['value'],
      version: (json['version']! as num).toInt(),
      updatedBy: json['updated_by'] as String?,
      updatedAt: _time(json['updated_at']),
    );

/// The branding of a `Branding` JSON object.
BrandingState brandingOfJson(Map<String, Object?> json) => BrandingState(
  productName: json['product_name']! as String,
  accentColor: json['accent_color'] as String?,
  logoFileId: _logoId(json['logo_file_id'] as String?),
  version: (json['version']! as num).toInt(),
);

/// The flag of a `FeatureFlag` JSON object.
FeatureFlagEntry featureFlagOfJson(Map<String, Object?> json) {
  final rollout = json['rollout'] is Map
      ? (json['rollout']! as Map).cast<String, Object?>()
      : const <String, Object?>{};
  List<String> ids(Object? v) => [
    if (v is List)
      for (final e in v)
        if (e is String) e,
  ];
  return FeatureFlagEntry(
    key: json['key']! as String,
    kind: _flagKind(json['kind'] as String?),
    description: json['description'] as String?,
    defaultValue: json['default'] as bool?,
    removeBy: json['remove_by'] as String?,
    module: json['module'] as String?,
    orphaned: json['orphaned'] as bool? ?? false,
    enabled: json['enabled']! as bool,
    rolloutUserIds: ids(rollout['user_ids']),
    rolloutGroupIds: ids(rollout['group_ids']),
    version: (json['version']! as num).toInt(),
    updatedBy: json['updated_by'] as String?,
    updatedAt: _time(json['updated_at']),
  );
}

FeatureFlagKind _flagKind(String? wire) => switch (wire) {
  'release' => FeatureFlagKind.release,
  'option' => FeatureFlagKind.option,
  _ => FeatureFlagKind.unknown,
};

/// The logo file of a branding; the empty value the server stores after a removal is no logo
/// (DECISIONS M3-10, `NoLogo`).
String? _logoId(String? id) => id == null || id.isEmpty ? null : id;

DateTime? _time(Object? v) => v is String ? DateTime.tryParse(v) : null;

/// The MIME type of an image file name the logo upload accepts (JPEG, PNG, WebP; 12 3.7), or
/// `application/octet-stream` for any other name, which the server refuses with `415`.
String logoMimeOf(String name) {
  final lower = name.toLowerCase();
  if (lower.endsWith('.png')) return 'image/png';
  if (lower.endsWith('.jpg') || lower.endsWith('.jpeg')) return 'image/jpeg';
  if (lower.endsWith('.webp')) return 'image/webp';
  return 'application/octet-stream';
}

/// [InstanceRepository] over the generated client.
///
/// Example:
/// ```dart
/// final instance = ApiInstanceRepository(api, newIdempotencyKey: ids.next);
/// final settings = await instance.instanceSettings();
/// ```
final class ApiInstanceRepository implements InstanceRepository {
  /// Creates the repository over `api` (the client of the signed-in session, so DPoP, refresh
  /// and the step-up interceptor apply); [newIdempotencyKey] returns a fresh UUIDv7.
  ApiInstanceRepository(
    this._api, {
    required String Function() newIdempotencyKey,
  }) : _newKey = newIdempotencyKey;

  final ApiClient _api;
  final String Function() _newKey;

  AdminApi get _admin => _api.getAdminApi();

  Map<String, Object?> _purpose(String text) => {stepUpPurposeExtra: text};

  static String _ifMatch(int version) => '"$version"';

  /// The security requirement of the admin operations as the generated client sends it, so the
  /// auth interceptors sign the requests sent with a JSON text too.
  static const Map<String, Object?> _secure = {
    'secure': <Map<String, String>>[
      {
        'type': 'apiKey',
        'name': 'consoleSession',
        'keyName': '__Host-console_session',
        'where': '',
      },
      {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
      {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
    ],
  };

  /// Sends [body] as JSON with [method] to [path] and returns the decoded answer. The free-form
  /// members (`value` of a setting, `config` of a provider) travel as the plain JSON the screen
  /// built, so the server's closed schema sees exactly that (08 2.6: unknown fields, duplicate
  /// keys and `null` are refused).
  Future<Map<String, Object?>> _sendJson(
    String method,
    String path,
    Map<String, Object?> body, {
    required String purpose,
    String? ifMatch,
    bool idempotent = false,
  }) async {
    final response = await _api.dio.request<Object>(
      path,
      data: jsonEncode(body),
      options: Options(
        method: method,
        contentType: 'application/json',
        headers: {
          'If-Match': ?ifMatch,
          if (idempotent) 'Idempotency-Key': _newKey(),
        },
        extra: {..._secure, ..._purpose(purpose)},
      ),
    );
    final data = response.data;
    final json = data is String ? jsonDecode(data) : data;
    return (json! as Map).cast<String, Object?>();
  }

  static InstanceSettingEntry _setting(InstanceSetting s) =>
      InstanceSettingEntry(
        key: s.key,
        value: s.value?.value,
        version: s.version,
        updatedBy: s.updatedBy,
        updatedAt: s.updatedAt,
      );

  static BrandingState _branding(Branding b) => BrandingState(
    productName: b.productName,
    accentColor: b.accentColor,
    logoFileId: _logoId(b.logoFileId),
    version: b.version,
  );

  static FeatureFlagEntry _flag(FeatureFlag f) => FeatureFlagEntry(
    key: f.key,
    kind: _flagKind(f.kind?.name),
    description: f.description,
    defaultValue: f.default_,
    removeBy: f.removeBy,
    module: f.module,
    orphaned: f.orphaned,
    enabled: f.enabled,
    rolloutUserIds: f.rollout.userIds?.toList() ?? const [],
    rolloutGroupIds: f.rollout.groupIds?.toList() ?? const [],
    version: f.version,
    updatedBy: f.updatedBy,
    updatedAt: f.updatedAt,
  );

  static IdentityProviderEntry _provider(IdentityProvider p) =>
      IdentityProviderEntry(
        id: p.id,
        kind: ProviderKind.of(_kindWire(p.kind)),
        name: p.name,
        config: {
          for (final MapEntry(:key, :value) in p.config.entries)
            key: value?.value,
        },
        enabled: p.enabled,
        allowProvisioning: p.allowProvisioning,
        updatedAt: p.updatedAt,
      );

  static String _kindWire(IdentityProviderKind k) => switch (k) {
    IdentityProviderKind.oidc => 'oidc',
    IdentityProviderKind.trustedHeader => 'trusted_header',
    IdentityProviderKind.local => 'local',
    _ => '',
  };

  IdentityProviderEntry _providerOfJson(Map<String, Object?> json) => _provider(
    _api.serializers.deserializeWith(IdentityProvider.serializer, json)!,
  );

  /// Runs [write]; a `412` becomes an [InstanceVersionConflict] with the problem's `current`
  /// read by [ofCurrent], or with the state [reread] returns when the problem has none.
  static Future<R> _versioned<R>(
    Future<R> Function() write, {
    required R Function(Map<String, Object?> current) ofCurrent,
    required Future<R> Function() reread,
  }) async {
    try {
      return await write();
    } on SelfActionFailed catch (e) {
      final problem = e.problem;
      if (problem == null || problem.status != 412) rethrow;
      final current = problem.current;
      throw InstanceVersionConflict<R>(
        current != null ? ofCurrent(current) : await reread(),
      );
    }
  }

  @override
  Future<List<InstanceSettingEntry>> instanceSettings() => adminCall(
    () async => [
      for (final s in (await _admin.listInstanceSettings()).data!.items)
        _setting(s),
    ],
  );

  Future<InstanceSettingEntry> _settingNow(String key) async =>
      (await instanceSettings()).firstWhere(
        (s) => s.key == key,
        orElse: () => throw const SelfActionFailed(
          ApiProblem(status: 404, code: 'not_found'),
        ),
      );

  @override
  Future<InstanceSettingEntry> putInstanceSetting(
    String key,
    Object? value, {
    required int version,
    required String purpose,
  }) => _versioned(
    () => adminCall(
      () async => instanceSettingOfJson(
        await _sendJson(
          'PUT',
          '/instance/settings/${Uri.encodeComponent(key)}',
          {'value': value},
          ifMatch: _ifMatch(version),
          purpose: purpose,
        ),
      ),
    ),
    ofCurrent: instanceSettingOfJson,
    reread: () => _settingNow(key),
  );

  @override
  Future<InstanceOverview> instanceOverview() => adminCall(() async {
    final info = (await _admin.getInstance()).data!;
    return InstanceOverview(
      branding: _branding(info.branding),
      features: {for (final f in info.features) f.key: f.enabled},
    );
  });

  @override
  Future<BrandingState> updateBranding({
    required String productName,
    required String accentColor,
    required int version,
    required String purpose,
  }) => _versioned(
    () => adminCall(
      () async => _branding(
        (await _admin.updateBranding(
          ifMatch: _ifMatch(version),
          brandingUpdate: BrandingUpdate(
            (b) => b
              ..productName = productName
              ..accentColor = accentColor,
          ),
          extra: _purpose(purpose),
        )).data!,
      ),
    ),
    ofCurrent: brandingOfJson,
    reread: () async => (await instanceOverview()).branding,
  );

  @override
  Future<BrandingState> uploadLogo(
    PickedFile file, {
    required String purpose,
  }) => adminCall(
    () async => _branding(
      (await _admin.uploadBrandingLogo(
        file: MultipartFile.fromBytes(
          file.bytes,
          filename: file.name,
          contentType: DioMediaType.parse(logoMimeOf(file.name)),
        ),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<void> removeLogo({required String purpose}) => adminCall(() async {
    await _admin.deleteBrandingLogo(extra: _purpose(purpose));
  });

  @override
  Future<Uint8List?> logo() async {
    try {
      return await adminCall(
        () async => (await _api.getDiscoveryApi().getBrandingLogo()).data,
      );
    } on SelfActionFailed catch (e) {
      if (e.problem?.status == 404) return null;
      rethrow;
    }
  }

  @override
  Future<List<FeatureFlagEntry>> featureFlags() => adminCall(
    () async => [
      for (final f in (await _admin.listFeatureFlags()).data!.items) _flag(f),
    ],
  );

  Future<FeatureFlagEntry> _flagNow(String key) async =>
      (await featureFlags()).firstWhere(
        (f) => f.key == key,
        orElse: () => throw const SelfActionFailed(
          ApiProblem(status: 404, code: 'not_found'),
        ),
      );

  @override
  Future<FeatureFlagEntry> putFeatureFlag(
    FeatureFlagEntry flag, {
    required bool enabled,
    required String reason,
    required String purpose,
  }) => _versioned(
    () => adminCall(
      () async => _flag(
        (await _admin.putFeatureFlag(
          key: flag.key,
          ifMatch: _ifMatch(flag.version),
          featureFlagPut: FeatureFlagPut(
            (b) => b
              ..enabled = enabled
              ..reason = reason
              // The switch changes only the global value; the rollout is sent as it is, so
              // the write keeps it (configuration reference 7; Ruling D-M6-09-4).
              ..rollout.userIds.replace(flag.rolloutUserIds)
              ..rollout.groupIds.replace(flag.rolloutGroupIds),
          ),
          extra: _purpose(purpose),
        )).data!,
      ),
    ),
    ofCurrent: featureFlagOfJson,
    reread: () => _flagNow(flag.key),
  );

  @override
  Future<List<IdentityProviderEntry>> identityProviders() => adminCall(
    () async => [
      for (final p in (await _admin.listIdentityProviders()).data!.items)
        _provider(p),
    ],
  );

  @override
  Future<IdentityProviderEntry> createIdentityProvider(
    ProviderDraft draft, {
    required String purpose,
  }) {
    if (draft.kind == ProviderKind.unknown) {
      throw ArgumentError.value(draft.kind, 'kind', 'not a driver to create');
    }
    return adminCall(
      () async => _providerOfJson(
        await _sendJson(
          'POST',
          '/identity-providers',
          {
            'kind': draft.kind.wire,
            'name': draft.name,
            'config': draft.config,
            'enabled': draft.enabled,
            'allow_provisioning': draft.allowProvisioning,
          },
          idempotent: true,
          purpose: purpose,
        ),
      ),
    );
  }

  @override
  Future<IdentityProviderEntry> updateIdentityProvider(
    String id,
    ProviderChange change, {
    required String purpose,
  }) => adminCall(
    () async => _providerOfJson(
      await _sendJson(
        'PATCH',
        '/identity-providers/${Uri.encodeComponent(id)}',
        {
          'name': ?change.name,
          'config': ?change.config,
          'enabled': ?change.enabled,
          'allow_provisioning': ?change.allowProvisioning,
        },
        idempotent: true,
        purpose: purpose,
      ),
    ),
  );

  @override
  Future<void> deleteIdentityProvider(String id, {required String purpose}) =>
      adminCall(() async {
        await _admin.deleteIdentityProvider(
          providerId: id,
          extra: _purpose(purpose),
        );
      });

  @override
  String get oidcRedirectUri =>
      '${_api.dio.options.baseUrl.replaceFirst(RegExp(r'/+$'), '')}'
      '$oidcCallbackPath';
}
