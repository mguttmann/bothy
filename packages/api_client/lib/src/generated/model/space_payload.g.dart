// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'space_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SpacePayload extends SpacePayload {
  @override
  final Space space;

  factory _$SpacePayload([void Function(SpacePayloadBuilder)? updates]) =>
      (SpacePayloadBuilder()..update(updates))._build();

  _$SpacePayload._({required this.space}) : super._();
  @override
  SpacePayload rebuild(void Function(SpacePayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SpacePayloadBuilder toBuilder() => SpacePayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SpacePayload && space == other.space;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, space.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'SpacePayload',
    )..add('space', space)).toString();
  }
}

class SpacePayloadBuilder
    implements Builder<SpacePayload, SpacePayloadBuilder> {
  _$SpacePayload? _$v;

  SpaceBuilder? _space;
  SpaceBuilder get space => _$this._space ??= SpaceBuilder();
  set space(SpaceBuilder? space) => _$this._space = space;

  SpacePayloadBuilder() {
    SpacePayload._defaults(this);
  }

  SpacePayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _space = $v.space.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SpacePayload other) {
    _$v = other as _$SpacePayload;
  }

  @override
  void update(void Function(SpacePayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SpacePayload build() => _build();

  _$SpacePayload _build() {
    _$SpacePayload _$result;
    try {
      _$result = _$v ?? _$SpacePayload._(space: space.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'space';
        space.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SpacePayload',
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
