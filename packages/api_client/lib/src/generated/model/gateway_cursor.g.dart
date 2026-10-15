// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_cursor.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayCursor extends GatewayCursor {
  @override
  final int seq;

  factory _$GatewayCursor([void Function(GatewayCursorBuilder)? updates]) =>
      (GatewayCursorBuilder()..update(updates))._build();

  _$GatewayCursor._({required this.seq}) : super._();
  @override
  GatewayCursor rebuild(void Function(GatewayCursorBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayCursorBuilder toBuilder() => GatewayCursorBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayCursor && seq == other.seq;
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
      r'GatewayCursor',
    )..add('seq', seq)).toString();
  }
}

class GatewayCursorBuilder
    implements Builder<GatewayCursor, GatewayCursorBuilder> {
  _$GatewayCursor? _$v;

  int? _seq;
  int? get seq => _$this._seq;
  set seq(int? seq) => _$this._seq = seq;

  GatewayCursorBuilder() {
    GatewayCursor._defaults(this);
  }

  GatewayCursorBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _seq = $v.seq;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayCursor other) {
    _$v = other as _$GatewayCursor;
  }

  @override
  void update(void Function(GatewayCursorBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayCursor build() => _build();

  _$GatewayCursor _build() {
    final _$result =
        _$v ??
        _$GatewayCursor._(
          seq: BuiltValueNullFieldError.checkNotNull(
            seq,
            r'GatewayCursor',
            'seq',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
