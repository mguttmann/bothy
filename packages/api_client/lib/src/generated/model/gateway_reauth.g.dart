// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_reauth.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayReauth extends GatewayReauth {
  @override
  final String accessToken;
  @override
  final String dpop;

  factory _$GatewayReauth([void Function(GatewayReauthBuilder)? updates]) =>
      (GatewayReauthBuilder()..update(updates))._build();

  _$GatewayReauth._({required this.accessToken, required this.dpop})
    : super._();
  @override
  GatewayReauth rebuild(void Function(GatewayReauthBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayReauthBuilder toBuilder() => GatewayReauthBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayReauth &&
        accessToken == other.accessToken &&
        dpop == other.dpop;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, accessToken.hashCode);
    _$hash = $jc(_$hash, dpop.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayReauth')
          ..add('accessToken', accessToken)
          ..add('dpop', dpop))
        .toString();
  }
}

class GatewayReauthBuilder
    implements Builder<GatewayReauth, GatewayReauthBuilder> {
  _$GatewayReauth? _$v;

  String? _accessToken;
  String? get accessToken => _$this._accessToken;
  set accessToken(String? accessToken) => _$this._accessToken = accessToken;

  String? _dpop;
  String? get dpop => _$this._dpop;
  set dpop(String? dpop) => _$this._dpop = dpop;

  GatewayReauthBuilder() {
    GatewayReauth._defaults(this);
  }

  GatewayReauthBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _accessToken = $v.accessToken;
      _dpop = $v.dpop;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayReauth other) {
    _$v = other as _$GatewayReauth;
  }

  @override
  void update(void Function(GatewayReauthBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayReauth build() => _build();

  _$GatewayReauth _build() {
    final _$result =
        _$v ??
        _$GatewayReauth._(
          accessToken: BuiltValueNullFieldError.checkNotNull(
            accessToken,
            r'GatewayReauth',
            'accessToken',
          ),
          dpop: BuiltValueNullFieldError.checkNotNull(
            dpop,
            r'GatewayReauth',
            'dpop',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
