// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_resumed.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayResumed extends GatewayResumed {
  @override
  final String gatewaySessionId;
  @override
  final int headSeq;
  @override
  final String syncEpoch;

  factory _$GatewayResumed([void Function(GatewayResumedBuilder)? updates]) =>
      (GatewayResumedBuilder()..update(updates))._build();

  _$GatewayResumed._({
    required this.gatewaySessionId,
    required this.headSeq,
    required this.syncEpoch,
  }) : super._();
  @override
  GatewayResumed rebuild(void Function(GatewayResumedBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayResumedBuilder toBuilder() => GatewayResumedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayResumed &&
        gatewaySessionId == other.gatewaySessionId &&
        headSeq == other.headSeq &&
        syncEpoch == other.syncEpoch;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, gatewaySessionId.hashCode);
    _$hash = $jc(_$hash, headSeq.hashCode);
    _$hash = $jc(_$hash, syncEpoch.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayResumed')
          ..add('gatewaySessionId', gatewaySessionId)
          ..add('headSeq', headSeq)
          ..add('syncEpoch', syncEpoch))
        .toString();
  }
}

class GatewayResumedBuilder
    implements Builder<GatewayResumed, GatewayResumedBuilder> {
  _$GatewayResumed? _$v;

  String? _gatewaySessionId;
  String? get gatewaySessionId => _$this._gatewaySessionId;
  set gatewaySessionId(String? gatewaySessionId) =>
      _$this._gatewaySessionId = gatewaySessionId;

  int? _headSeq;
  int? get headSeq => _$this._headSeq;
  set headSeq(int? headSeq) => _$this._headSeq = headSeq;

  String? _syncEpoch;
  String? get syncEpoch => _$this._syncEpoch;
  set syncEpoch(String? syncEpoch) => _$this._syncEpoch = syncEpoch;

  GatewayResumedBuilder() {
    GatewayResumed._defaults(this);
  }

  GatewayResumedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _gatewaySessionId = $v.gatewaySessionId;
      _headSeq = $v.headSeq;
      _syncEpoch = $v.syncEpoch;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayResumed other) {
    _$v = other as _$GatewayResumed;
  }

  @override
  void update(void Function(GatewayResumedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayResumed build() => _build();

  _$GatewayResumed _build() {
    final _$result =
        _$v ??
        _$GatewayResumed._(
          gatewaySessionId: BuiltValueNullFieldError.checkNotNull(
            gatewaySessionId,
            r'GatewayResumed',
            'gatewaySessionId',
          ),
          headSeq: BuiltValueNullFieldError.checkNotNull(
            headSeq,
            r'GatewayResumed',
            'headSeq',
          ),
          syncEpoch: BuiltValueNullFieldError.checkNotNull(
            syncEpoch,
            r'GatewayResumed',
            'syncEpoch',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
