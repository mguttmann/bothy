// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_install.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AppInstall extends AppInstall {
  @override
  final AppManifest manifest;
  @override
  final BuiltList<AppScope> grantedScopes;
  @override
  final BuiltList<String>? eventSubscriptions;
  @override
  final String reason;

  factory _$AppInstall([void Function(AppInstallBuilder)? updates]) =>
      (AppInstallBuilder()..update(updates))._build();

  _$AppInstall._({
    required this.manifest,
    required this.grantedScopes,
    this.eventSubscriptions,
    required this.reason,
  }) : super._();
  @override
  AppInstall rebuild(void Function(AppInstallBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AppInstallBuilder toBuilder() => AppInstallBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AppInstall &&
        manifest == other.manifest &&
        grantedScopes == other.grantedScopes &&
        eventSubscriptions == other.eventSubscriptions &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, manifest.hashCode);
    _$hash = $jc(_$hash, grantedScopes.hashCode);
    _$hash = $jc(_$hash, eventSubscriptions.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AppInstall')
          ..add('manifest', manifest)
          ..add('grantedScopes', grantedScopes)
          ..add('eventSubscriptions', eventSubscriptions)
          ..add('reason', reason))
        .toString();
  }
}

class AppInstallBuilder implements Builder<AppInstall, AppInstallBuilder> {
  _$AppInstall? _$v;

  AppManifestBuilder? _manifest;
  AppManifestBuilder get manifest => _$this._manifest ??= AppManifestBuilder();
  set manifest(AppManifestBuilder? manifest) => _$this._manifest = manifest;

  ListBuilder<AppScope>? _grantedScopes;
  ListBuilder<AppScope> get grantedScopes =>
      _$this._grantedScopes ??= ListBuilder<AppScope>();
  set grantedScopes(ListBuilder<AppScope>? grantedScopes) =>
      _$this._grantedScopes = grantedScopes;

  ListBuilder<String>? _eventSubscriptions;
  ListBuilder<String> get eventSubscriptions =>
      _$this._eventSubscriptions ??= ListBuilder<String>();
  set eventSubscriptions(ListBuilder<String>? eventSubscriptions) =>
      _$this._eventSubscriptions = eventSubscriptions;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  AppInstallBuilder() {
    AppInstall._defaults(this);
  }

  AppInstallBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _manifest = $v.manifest.toBuilder();
      _grantedScopes = $v.grantedScopes.toBuilder();
      _eventSubscriptions = $v.eventSubscriptions?.toBuilder();
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AppInstall other) {
    _$v = other as _$AppInstall;
  }

  @override
  void update(void Function(AppInstallBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AppInstall build() => _build();

  _$AppInstall _build() {
    _$AppInstall _$result;
    try {
      _$result =
          _$v ??
          _$AppInstall._(
            manifest: manifest.build(),
            grantedScopes: grantedScopes.build(),
            eventSubscriptions: _eventSubscriptions?.build(),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'AppInstall',
              'reason',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'manifest';
        manifest.build();
        _$failedField = 'grantedScopes';
        grantedScopes.build();
        _$failedField = 'eventSubscriptions';
        _eventSubscriptions?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AppInstall',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
