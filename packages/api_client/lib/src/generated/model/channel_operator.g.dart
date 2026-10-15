// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_operator.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelOperator extends ChannelOperator {
  @override
  final OperatorMode mode;
  @override
  final String? agentId;
  @override
  final String? effectiveAgentId;

  factory _$ChannelOperator([void Function(ChannelOperatorBuilder)? updates]) =>
      (ChannelOperatorBuilder()..update(updates))._build();

  _$ChannelOperator._({required this.mode, this.agentId, this.effectiveAgentId})
    : super._();
  @override
  ChannelOperator rebuild(void Function(ChannelOperatorBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ChannelOperatorBuilder toBuilder() => ChannelOperatorBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelOperator &&
        mode == other.mode &&
        agentId == other.agentId &&
        effectiveAgentId == other.effectiveAgentId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, mode.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, effectiveAgentId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ChannelOperator')
          ..add('mode', mode)
          ..add('agentId', agentId)
          ..add('effectiveAgentId', effectiveAgentId))
        .toString();
  }
}

class ChannelOperatorBuilder
    implements Builder<ChannelOperator, ChannelOperatorBuilder> {
  _$ChannelOperator? _$v;

  OperatorMode? _mode;
  OperatorMode? get mode => _$this._mode;
  set mode(OperatorMode? mode) => _$this._mode = mode;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _effectiveAgentId;
  String? get effectiveAgentId => _$this._effectiveAgentId;
  set effectiveAgentId(String? effectiveAgentId) =>
      _$this._effectiveAgentId = effectiveAgentId;

  ChannelOperatorBuilder() {
    ChannelOperator._defaults(this);
  }

  ChannelOperatorBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _mode = $v.mode;
      _agentId = $v.agentId;
      _effectiveAgentId = $v.effectiveAgentId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelOperator other) {
    _$v = other as _$ChannelOperator;
  }

  @override
  void update(void Function(ChannelOperatorBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelOperator build() => _build();

  _$ChannelOperator _build() {
    final _$result =
        _$v ??
        _$ChannelOperator._(
          mode: BuiltValueNullFieldError.checkNotNull(
            mode,
            r'ChannelOperator',
            'mode',
          ),
          agentId: agentId,
          effectiveAgentId: effectiveAgentId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
