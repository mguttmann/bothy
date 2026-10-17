// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_catchup_complete.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayCatchupComplete extends GatewayCatchupComplete {
  @override
  final int headSeq;

  factory _$GatewayCatchupComplete([
    void Function(GatewayCatchupCompleteBuilder)? updates,
  ]) => (GatewayCatchupCompleteBuilder()..update(updates))._build();

  _$GatewayCatchupComplete._({required this.headSeq}) : super._();
  @override
  GatewayCatchupComplete rebuild(
    void Function(GatewayCatchupCompleteBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  GatewayCatchupCompleteBuilder toBuilder() =>
      GatewayCatchupCompleteBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayCatchupComplete && headSeq == other.headSeq;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, headSeq.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'GatewayCatchupComplete',
    )..add('headSeq', headSeq)).toString();
  }
}

class GatewayCatchupCompleteBuilder
    implements Builder<GatewayCatchupComplete, GatewayCatchupCompleteBuilder> {
  _$GatewayCatchupComplete? _$v;

  int? _headSeq;
  int? get headSeq => _$this._headSeq;
  set headSeq(int? headSeq) => _$this._headSeq = headSeq;

  GatewayCatchupCompleteBuilder() {
    GatewayCatchupComplete._defaults(this);
  }

  GatewayCatchupCompleteBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _headSeq = $v.headSeq;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayCatchupComplete other) {
    _$v = other as _$GatewayCatchupComplete;
  }

  @override
  void update(void Function(GatewayCatchupCompleteBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayCatchupComplete build() => _build();

  _$GatewayCatchupComplete _build() {
    final _$result =
        _$v ??
        _$GatewayCatchupComplete._(
          headSeq: BuiltValueNullFieldError.checkNotNull(
            headSeq,
            r'GatewayCatchupComplete',
            'headSeq',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
