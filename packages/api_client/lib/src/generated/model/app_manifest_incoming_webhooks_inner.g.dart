// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_manifest_incoming_webhooks_inner.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AppManifestIncomingWebhooksInner
    extends AppManifestIncomingWebhooksInner {
  @override
  final String name;
  @override
  final String purpose;

  factory _$AppManifestIncomingWebhooksInner([
    void Function(AppManifestIncomingWebhooksInnerBuilder)? updates,
  ]) => (AppManifestIncomingWebhooksInnerBuilder()..update(updates))._build();

  _$AppManifestIncomingWebhooksInner._({
    required this.name,
    required this.purpose,
  }) : super._();
  @override
  AppManifestIncomingWebhooksInner rebuild(
    void Function(AppManifestIncomingWebhooksInnerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AppManifestIncomingWebhooksInnerBuilder toBuilder() =>
      AppManifestIncomingWebhooksInnerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AppManifestIncomingWebhooksInner &&
        name == other.name &&
        purpose == other.purpose;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, purpose.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AppManifestIncomingWebhooksInner')
          ..add('name', name)
          ..add('purpose', purpose))
        .toString();
  }
}

class AppManifestIncomingWebhooksInnerBuilder
    implements
        Builder<
          AppManifestIncomingWebhooksInner,
          AppManifestIncomingWebhooksInnerBuilder
        > {
  _$AppManifestIncomingWebhooksInner? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _purpose;
  String? get purpose => _$this._purpose;
  set purpose(String? purpose) => _$this._purpose = purpose;

  AppManifestIncomingWebhooksInnerBuilder() {
    AppManifestIncomingWebhooksInner._defaults(this);
  }

  AppManifestIncomingWebhooksInnerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _purpose = $v.purpose;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AppManifestIncomingWebhooksInner other) {
    _$v = other as _$AppManifestIncomingWebhooksInner;
  }

  @override
  void update(void Function(AppManifestIncomingWebhooksInnerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AppManifestIncomingWebhooksInner build() => _build();

  _$AppManifestIncomingWebhooksInner _build() {
    final _$result =
        _$v ??
        _$AppManifestIncomingWebhooksInner._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'AppManifestIncomingWebhooksInner',
            'name',
          ),
          purpose: BuiltValueNullFieldError.checkNotNull(
            purpose,
            r'AppManifestIncomingWebhooksInner',
            'purpose',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
