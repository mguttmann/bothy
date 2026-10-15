// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'dm_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$DmCreate extends DmCreate {
  @override
  final BuiltList<String> participantIds;

  factory _$DmCreate([void Function(DmCreateBuilder)? updates]) =>
      (DmCreateBuilder()..update(updates))._build();

  _$DmCreate._({required this.participantIds}) : super._();
  @override
  DmCreate rebuild(void Function(DmCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DmCreateBuilder toBuilder() => DmCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DmCreate && participantIds == other.participantIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, participantIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'DmCreate',
    )..add('participantIds', participantIds)).toString();
  }
}

class DmCreateBuilder implements Builder<DmCreate, DmCreateBuilder> {
  _$DmCreate? _$v;

  ListBuilder<String>? _participantIds;
  ListBuilder<String> get participantIds =>
      _$this._participantIds ??= ListBuilder<String>();
  set participantIds(ListBuilder<String>? participantIds) =>
      _$this._participantIds = participantIds;

  DmCreateBuilder() {
    DmCreate._defaults(this);
  }

  DmCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _participantIds = $v.participantIds.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DmCreate other) {
    _$v = other as _$DmCreate;
  }

  @override
  void update(void Function(DmCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DmCreate build() => _build();

  _$DmCreate _build() {
    _$DmCreate _$result;
    try {
      _$result = _$v ?? _$DmCreate._(participantIds: participantIds.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'participantIds';
        participantIds.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'DmCreate',
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
