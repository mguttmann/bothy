// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'consumer_ack.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ConsumerAck extends ConsumerAck {
  @override
  final int seq;

  factory _$ConsumerAck([void Function(ConsumerAckBuilder)? updates]) =>
      (ConsumerAckBuilder()..update(updates))._build();

  _$ConsumerAck._({required this.seq}) : super._();
  @override
  ConsumerAck rebuild(void Function(ConsumerAckBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ConsumerAckBuilder toBuilder() => ConsumerAckBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ConsumerAck && seq == other.seq;
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
      r'ConsumerAck',
    )..add('seq', seq)).toString();
  }
}

class ConsumerAckBuilder implements Builder<ConsumerAck, ConsumerAckBuilder> {
  _$ConsumerAck? _$v;

  int? _seq;
  int? get seq => _$this._seq;
  set seq(int? seq) => _$this._seq = seq;

  ConsumerAckBuilder() {
    ConsumerAck._defaults(this);
  }

  ConsumerAckBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _seq = $v.seq;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ConsumerAck other) {
    _$v = other as _$ConsumerAck;
  }

  @override
  void update(void Function(ConsumerAckBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ConsumerAck build() => _build();

  _$ConsumerAck _build() {
    final _$result =
        _$v ??
        _$ConsumerAck._(
          seq: BuiltValueNullFieldError.checkNotNull(
            seq,
            r'ConsumerAck',
            'seq',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
