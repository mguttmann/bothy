// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_session_bind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentSessionBind extends AgentSessionBind {
  @override
  final String externalSessionRef;

  factory _$AgentSessionBind([
    void Function(AgentSessionBindBuilder)? updates,
  ]) => (AgentSessionBindBuilder()..update(updates))._build();

  _$AgentSessionBind._({required this.externalSessionRef}) : super._();
  @override
  AgentSessionBind rebuild(void Function(AgentSessionBindBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentSessionBindBuilder toBuilder() =>
      AgentSessionBindBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentSessionBind &&
        externalSessionRef == other.externalSessionRef;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, externalSessionRef.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'AgentSessionBind',
    )..add('externalSessionRef', externalSessionRef)).toString();
  }
}

class AgentSessionBindBuilder
    implements Builder<AgentSessionBind, AgentSessionBindBuilder> {
  _$AgentSessionBind? _$v;

  String? _externalSessionRef;
  String? get externalSessionRef => _$this._externalSessionRef;
  set externalSessionRef(String? externalSessionRef) =>
      _$this._externalSessionRef = externalSessionRef;

  AgentSessionBindBuilder() {
    AgentSessionBind._defaults(this);
  }

  AgentSessionBindBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _externalSessionRef = $v.externalSessionRef;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentSessionBind other) {
    _$v = other as _$AgentSessionBind;
  }

  @override
  void update(void Function(AgentSessionBindBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentSessionBind build() => _build();

  _$AgentSessionBind _build() {
    final _$result =
        _$v ??
        _$AgentSessionBind._(
          externalSessionRef: BuiltValueNullFieldError.checkNotNull(
            externalSessionRef,
            r'AgentSessionBind',
            'externalSessionRef',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
