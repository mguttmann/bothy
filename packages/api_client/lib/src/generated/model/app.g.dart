// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$App extends App {
  @override
  final String id;
  @override
  final String slug;
  @override
  final String displayName;
  @override
  final AppManifest manifest;
  @override
  final String manifestVersion;
  @override
  final AppInstallation installation;

  factory _$App([void Function(AppBuilder)? updates]) =>
      (AppBuilder()..update(updates))._build();

  _$App._({
    required this.id,
    required this.slug,
    required this.displayName,
    required this.manifest,
    required this.manifestVersion,
    required this.installation,
  }) : super._();
  @override
  App rebuild(void Function(AppBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AppBuilder toBuilder() => AppBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is App &&
        id == other.id &&
        slug == other.slug &&
        displayName == other.displayName &&
        manifest == other.manifest &&
        manifestVersion == other.manifestVersion &&
        installation == other.installation;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, slug.hashCode);
    _$hash = $jc(_$hash, displayName.hashCode);
    _$hash = $jc(_$hash, manifest.hashCode);
    _$hash = $jc(_$hash, manifestVersion.hashCode);
    _$hash = $jc(_$hash, installation.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'App')
          ..add('id', id)
          ..add('slug', slug)
          ..add('displayName', displayName)
          ..add('manifest', manifest)
          ..add('manifestVersion', manifestVersion)
          ..add('installation', installation))
        .toString();
  }
}

class AppBuilder implements Builder<App, AppBuilder> {
  _$App? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _slug;
  String? get slug => _$this._slug;
  set slug(String? slug) => _$this._slug = slug;

  String? _displayName;
  String? get displayName => _$this._displayName;
  set displayName(String? displayName) => _$this._displayName = displayName;

  AppManifestBuilder? _manifest;
  AppManifestBuilder get manifest => _$this._manifest ??= AppManifestBuilder();
  set manifest(AppManifestBuilder? manifest) => _$this._manifest = manifest;

  String? _manifestVersion;
  String? get manifestVersion => _$this._manifestVersion;
  set manifestVersion(String? manifestVersion) =>
      _$this._manifestVersion = manifestVersion;

  AppInstallationBuilder? _installation;
  AppInstallationBuilder get installation =>
      _$this._installation ??= AppInstallationBuilder();
  set installation(AppInstallationBuilder? installation) =>
      _$this._installation = installation;

  AppBuilder() {
    App._defaults(this);
  }

  AppBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _slug = $v.slug;
      _displayName = $v.displayName;
      _manifest = $v.manifest.toBuilder();
      _manifestVersion = $v.manifestVersion;
      _installation = $v.installation.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(App other) {
    _$v = other as _$App;
  }

  @override
  void update(void Function(AppBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  App build() => _build();

  _$App _build() {
    _$App _$result;
    try {
      _$result =
          _$v ??
          _$App._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'App', 'id'),
            slug: BuiltValueNullFieldError.checkNotNull(slug, r'App', 'slug'),
            displayName: BuiltValueNullFieldError.checkNotNull(
              displayName,
              r'App',
              'displayName',
            ),
            manifest: manifest.build(),
            manifestVersion: BuiltValueNullFieldError.checkNotNull(
              manifestVersion,
              r'App',
              'manifestVersion',
            ),
            installation: installation.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'manifest';
        manifest.build();

        _$failedField = 'installation';
        installation.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(r'App', _$failedField, e.toString());
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
