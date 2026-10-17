// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'flag_rollout.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$FlagRollout extends FlagRollout {
  @override
  final BuiltList<String>? userIds;
  @override
  final BuiltList<String>? groupIds;

  factory _$FlagRollout([void Function(FlagRolloutBuilder)? updates]) =>
      (FlagRolloutBuilder()..update(updates))._build();

  _$FlagRollout._({this.userIds, this.groupIds}) : super._();
  @override
  FlagRollout rebuild(void Function(FlagRolloutBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  FlagRolloutBuilder toBuilder() => FlagRolloutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is FlagRollout &&
        userIds == other.userIds &&
        groupIds == other.groupIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, userIds.hashCode);
    _$hash = $jc(_$hash, groupIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'FlagRollout')
          ..add('userIds', userIds)
          ..add('groupIds', groupIds))
        .toString();
  }
}

class FlagRolloutBuilder implements Builder<FlagRollout, FlagRolloutBuilder> {
  _$FlagRollout? _$v;

  ListBuilder<String>? _userIds;
  ListBuilder<String> get userIds => _$this._userIds ??= ListBuilder<String>();
  set userIds(ListBuilder<String>? userIds) => _$this._userIds = userIds;

  ListBuilder<String>? _groupIds;
  ListBuilder<String> get groupIds =>
      _$this._groupIds ??= ListBuilder<String>();
  set groupIds(ListBuilder<String>? groupIds) => _$this._groupIds = groupIds;

  FlagRolloutBuilder() {
    FlagRollout._defaults(this);
  }

  FlagRolloutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _userIds = $v.userIds?.toBuilder();
      _groupIds = $v.groupIds?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(FlagRollout other) {
    _$v = other as _$FlagRollout;
  }

  @override
  void update(void Function(FlagRolloutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  FlagRollout build() => _build();

  _$FlagRollout _build() {
    _$FlagRollout _$result;
    try {
      _$result =
          _$v ??
          _$FlagRollout._(
            userIds: _userIds?.build(),
            groupIds: _groupIds?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'userIds';
        _userIds?.build();
        _$failedField = 'groupIds';
        _groupIds?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'FlagRollout',
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
