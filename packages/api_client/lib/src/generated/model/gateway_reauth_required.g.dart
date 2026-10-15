// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_reauth_required.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayReauthRequired extends GatewayReauthRequired {
  @override
  final DateTime expiresAt;
  @override
  final String dpopNonce;

  factory _$GatewayReauthRequired([
    void Function(GatewayReauthRequiredBuilder)? updates,
  ]) => (GatewayReauthRequiredBuilder()..update(updates))._build();

  _$GatewayReauthRequired._({required this.expiresAt, required this.dpopNonce})
    : super._();
  @override
  GatewayReauthRequired rebuild(
    void Function(GatewayReauthRequiredBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  GatewayReauthRequiredBuilder toBuilder() =>
      GatewayReauthRequiredBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayReauthRequired &&
        expiresAt == other.expiresAt &&
        dpopNonce == other.dpopNonce;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jc(_$hash, dpopNonce.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayReauthRequired')
          ..add('expiresAt', expiresAt)
          ..add('dpopNonce', dpopNonce))
        .toString();
  }
}

class GatewayReauthRequiredBuilder
    implements Builder<GatewayReauthRequired, GatewayReauthRequiredBuilder> {
  _$GatewayReauthRequired? _$v;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  String? _dpopNonce;
  String? get dpopNonce => _$this._dpopNonce;
  set dpopNonce(String? dpopNonce) => _$this._dpopNonce = dpopNonce;

  GatewayReauthRequiredBuilder() {
    GatewayReauthRequired._defaults(this);
  }

  GatewayReauthRequiredBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _expiresAt = $v.expiresAt;
      _dpopNonce = $v.dpopNonce;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayReauthRequired other) {
    _$v = other as _$GatewayReauthRequired;
  }

  @override
  void update(void Function(GatewayReauthRequiredBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayReauthRequired build() => _build();

  _$GatewayReauthRequired _build() {
    final _$result =
        _$v ??
        _$GatewayReauthRequired._(
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'GatewayReauthRequired',
            'expiresAt',
          ),
          dpopNonce: BuiltValueNullFieldError.checkNotNull(
            dpopNonce,
            r'GatewayReauthRequired',
            'dpopNonce',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
