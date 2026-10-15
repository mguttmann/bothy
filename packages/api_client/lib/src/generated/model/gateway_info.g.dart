// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_info.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayInfo extends GatewayInfo {
  @override
  final String url;
  @override
  final BuiltList<int> protocolVersions;

  factory _$GatewayInfo([void Function(GatewayInfoBuilder)? updates]) =>
      (GatewayInfoBuilder()..update(updates))._build();

  _$GatewayInfo._({required this.url, required this.protocolVersions})
    : super._();
  @override
  GatewayInfo rebuild(void Function(GatewayInfoBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayInfoBuilder toBuilder() => GatewayInfoBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayInfo &&
        url == other.url &&
        protocolVersions == other.protocolVersions;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, url.hashCode);
    _$hash = $jc(_$hash, protocolVersions.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayInfo')
          ..add('url', url)
          ..add('protocolVersions', protocolVersions))
        .toString();
  }
}

class GatewayInfoBuilder implements Builder<GatewayInfo, GatewayInfoBuilder> {
  _$GatewayInfo? _$v;

  String? _url;
  String? get url => _$this._url;
  set url(String? url) => _$this._url = url;

  ListBuilder<int>? _protocolVersions;
  ListBuilder<int> get protocolVersions =>
      _$this._protocolVersions ??= ListBuilder<int>();
  set protocolVersions(ListBuilder<int>? protocolVersions) =>
      _$this._protocolVersions = protocolVersions;

  GatewayInfoBuilder() {
    GatewayInfo._defaults(this);
  }

  GatewayInfoBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _url = $v.url;
      _protocolVersions = $v.protocolVersions.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayInfo other) {
    _$v = other as _$GatewayInfo;
  }

  @override
  void update(void Function(GatewayInfoBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayInfo build() => _build();

  _$GatewayInfo _build() {
    _$GatewayInfo _$result;
    try {
      _$result =
          _$v ??
          _$GatewayInfo._(
            url: BuiltValueNullFieldError.checkNotNull(
              url,
              r'GatewayInfo',
              'url',
            ),
            protocolVersions: protocolVersions.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'protocolVersions';
        protocolVersions.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GatewayInfo',
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
