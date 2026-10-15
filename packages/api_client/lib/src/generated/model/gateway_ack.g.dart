// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_ack.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayAck extends GatewayAck {
  @override
  final int seq;

  factory _$GatewayAck([void Function(GatewayAckBuilder)? updates]) =>
      (GatewayAckBuilder()..update(updates))._build();

  _$GatewayAck._({required this.seq}) : super._();
  @override
  GatewayAck rebuild(void Function(GatewayAckBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayAckBuilder toBuilder() => GatewayAckBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayAck && seq == other.seq;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, seq.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'GatewayAck',
    )..add('seq', seq)).toString();
  }
}

class GatewayAckBuilder implements Builder<GatewayAck, GatewayAckBuilder> {
  _$GatewayAck? _$v;

  int? _seq;
  int? get seq => _$this._seq;
  set seq(int? seq) => _$this._seq = seq;

  GatewayAckBuilder() {
    GatewayAck._defaults(this);
  }

  GatewayAckBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _seq = $v.seq;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayAck other) {
    _$v = other as _$GatewayAck;
  }

  @override
  void update(void Function(GatewayAckBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayAck build() => _build();

  _$GatewayAck _build() {
    final _$result =
        _$v ??
        _$GatewayAck._(
          seq: BuiltValueNullFieldError.checkNotNull(seq, r'GatewayAck', 'seq'),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
