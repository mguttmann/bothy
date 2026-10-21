// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_session_bind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentSessionBind extends AgentSessionBind {
  @override
  final String externalSessionRef;
  @override
  final String? expectedCurrentSessionId;
  @override
  final bool? takeOver;

  factory _$AgentSessionBind([
    void Function(AgentSessionBindBuilder)? updates,
  ]) => (AgentSessionBindBuilder()..update(updates))._build();

  _$AgentSessionBind._({
    required this.externalSessionRef,
    this.expectedCurrentSessionId,
    this.takeOver,
  }) : super._();
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
        externalSessionRef == other.externalSessionRef &&
        expectedCurrentSessionId == other.expectedCurrentSessionId &&
        takeOver == other.takeOver;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, externalSessionRef.hashCode);
    _$hash = $jc(_$hash, expectedCurrentSessionId.hashCode);
    _$hash = $jc(_$hash, takeOver.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentSessionBind')
          ..add('externalSessionRef', externalSessionRef)
          ..add('expectedCurrentSessionId', expectedCurrentSessionId)
          ..add('takeOver', takeOver))
        .toString();
  }
}

class AgentSessionBindBuilder
    implements Builder<AgentSessionBind, AgentSessionBindBuilder> {
  _$AgentSessionBind? _$v;

  String? _externalSessionRef;
  String? get externalSessionRef => _$this._externalSessionRef;
  set externalSessionRef(String? externalSessionRef) =>
      _$this._externalSessionRef = externalSessionRef;

  String? _expectedCurrentSessionId;
  String? get expectedCurrentSessionId => _$this._expectedCurrentSessionId;
  set expectedCurrentSessionId(String? expectedCurrentSessionId) =>
      _$this._expectedCurrentSessionId = expectedCurrentSessionId;

  bool? _takeOver;
  bool? get takeOver => _$this._takeOver;
  set takeOver(bool? takeOver) => _$this._takeOver = takeOver;

  AgentSessionBindBuilder() {
    AgentSessionBind._defaults(this);
  }

  AgentSessionBindBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _externalSessionRef = $v.externalSessionRef;
      _expectedCurrentSessionId = $v.expectedCurrentSessionId;
      _takeOver = $v.takeOver;
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
          expectedCurrentSessionId: expectedCurrentSessionId,
          takeOver: takeOver,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
