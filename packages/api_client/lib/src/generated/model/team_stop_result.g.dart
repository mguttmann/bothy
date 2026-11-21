// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'team_stop_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TeamStopResult extends TeamStopResult {
  @override
  final BuiltList<String> stoppedRunIds;

  factory _$TeamStopResult([void Function(TeamStopResultBuilder)? updates]) =>
      (TeamStopResultBuilder()..update(updates))._build();

  _$TeamStopResult._({required this.stoppedRunIds}) : super._();
  @override
  TeamStopResult rebuild(void Function(TeamStopResultBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TeamStopResultBuilder toBuilder() => TeamStopResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TeamStopResult && stoppedRunIds == other.stoppedRunIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, stoppedRunIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'TeamStopResult',
    )..add('stoppedRunIds', stoppedRunIds)).toString();
  }
}

class TeamStopResultBuilder
    implements Builder<TeamStopResult, TeamStopResultBuilder> {
  _$TeamStopResult? _$v;

  ListBuilder<String>? _stoppedRunIds;
  ListBuilder<String> get stoppedRunIds =>
      _$this._stoppedRunIds ??= ListBuilder<String>();
  set stoppedRunIds(ListBuilder<String>? stoppedRunIds) =>
      _$this._stoppedRunIds = stoppedRunIds;

  TeamStopResultBuilder() {
    TeamStopResult._defaults(this);
  }

  TeamStopResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _stoppedRunIds = $v.stoppedRunIds.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TeamStopResult other) {
    _$v = other as _$TeamStopResult;
  }

  @override
  void update(void Function(TeamStopResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TeamStopResult build() => _build();

  _$TeamStopResult _build() {
    _$TeamStopResult _$result;
    try {
      _$result =
          _$v ?? _$TeamStopResult._(stoppedRunIds: stoppedRunIds.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'stoppedRunIds';
        stoppedRunIds.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'TeamStopResult',
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
