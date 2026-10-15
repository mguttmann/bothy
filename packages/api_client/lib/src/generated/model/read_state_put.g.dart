// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'read_state_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ReadStatePut extends ReadStatePut {
  @override
  final int lastReadSeq;

  factory _$ReadStatePut([void Function(ReadStatePutBuilder)? updates]) =>
      (ReadStatePutBuilder()..update(updates))._build();

  _$ReadStatePut._({required this.lastReadSeq}) : super._();
  @override
  ReadStatePut rebuild(void Function(ReadStatePutBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ReadStatePutBuilder toBuilder() => ReadStatePutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ReadStatePut && lastReadSeq == other.lastReadSeq;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, lastReadSeq.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ReadStatePut',
    )..add('lastReadSeq', lastReadSeq)).toString();
  }
}

class ReadStatePutBuilder
    implements Builder<ReadStatePut, ReadStatePutBuilder> {
  _$ReadStatePut? _$v;

  int? _lastReadSeq;
  int? get lastReadSeq => _$this._lastReadSeq;
  set lastReadSeq(int? lastReadSeq) => _$this._lastReadSeq = lastReadSeq;

  ReadStatePutBuilder() {
    ReadStatePut._defaults(this);
  }

  ReadStatePutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _lastReadSeq = $v.lastReadSeq;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ReadStatePut other) {
    _$v = other as _$ReadStatePut;
  }

  @override
  void update(void Function(ReadStatePutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ReadStatePut build() => _build();

  _$ReadStatePut _build() {
    final _$result =
        _$v ??
        _$ReadStatePut._(
          lastReadSeq: BuiltValueNullFieldError.checkNotNull(
            lastReadSeq,
            r'ReadStatePut',
            'lastReadSeq',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
