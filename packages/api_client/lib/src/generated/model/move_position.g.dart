// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'move_position.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$MovePosition extends MovePosition {
  @override
  final String? afterId;
  @override
  final String? beforeId;

  factory _$MovePosition([void Function(MovePositionBuilder)? updates]) =>
      (MovePositionBuilder()..update(updates))._build();

  _$MovePosition._({this.afterId, this.beforeId}) : super._();
  @override
  MovePosition rebuild(void Function(MovePositionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  MovePositionBuilder toBuilder() => MovePositionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is MovePosition &&
        afterId == other.afterId &&
        beforeId == other.beforeId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, afterId.hashCode);
    _$hash = $jc(_$hash, beforeId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'MovePosition')
          ..add('afterId', afterId)
          ..add('beforeId', beforeId))
        .toString();
  }
}

class MovePositionBuilder
    implements Builder<MovePosition, MovePositionBuilder> {
  _$MovePosition? _$v;

  String? _afterId;
  String? get afterId => _$this._afterId;
  set afterId(String? afterId) => _$this._afterId = afterId;

  String? _beforeId;
  String? get beforeId => _$this._beforeId;
  set beforeId(String? beforeId) => _$this._beforeId = beforeId;

  MovePositionBuilder() {
    MovePosition._defaults(this);
  }

  MovePositionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _afterId = $v.afterId;
      _beforeId = $v.beforeId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(MovePosition other) {
    _$v = other as _$MovePosition;
  }

  @override
  void update(void Function(MovePositionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  MovePosition build() => _build();

  _$MovePosition _build() {
    final _$result =
        _$v ?? _$MovePosition._(afterId: afterId, beforeId: beforeId);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
