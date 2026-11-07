// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_wake_facts.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentWakeFacts extends AgentWakeFacts {
  @override
  final RunWakeKind kind;
  @override
  final RunWakeReason reason;
  @override
  final String label;
  @override
  final String resultText;

  factory _$AgentWakeFacts([void Function(AgentWakeFactsBuilder)? updates]) =>
      (AgentWakeFactsBuilder()..update(updates))._build();

  _$AgentWakeFacts._({
    required this.kind,
    required this.reason,
    required this.label,
    required this.resultText,
  }) : super._();
  @override
  AgentWakeFacts rebuild(void Function(AgentWakeFactsBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentWakeFactsBuilder toBuilder() => AgentWakeFactsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentWakeFacts &&
        kind == other.kind &&
        reason == other.reason &&
        label == other.label &&
        resultText == other.resultText;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, label.hashCode);
    _$hash = $jc(_$hash, resultText.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentWakeFacts')
          ..add('kind', kind)
          ..add('reason', reason)
          ..add('label', label)
          ..add('resultText', resultText))
        .toString();
  }
}

class AgentWakeFactsBuilder
    implements Builder<AgentWakeFacts, AgentWakeFactsBuilder> {
  _$AgentWakeFacts? _$v;

  RunWakeKind? _kind;
  RunWakeKind? get kind => _$this._kind;
  set kind(RunWakeKind? kind) => _$this._kind = kind;

  RunWakeReason? _reason;
  RunWakeReason? get reason => _$this._reason;
  set reason(RunWakeReason? reason) => _$this._reason = reason;

  String? _label;
  String? get label => _$this._label;
  set label(String? label) => _$this._label = label;

  String? _resultText;
  String? get resultText => _$this._resultText;
  set resultText(String? resultText) => _$this._resultText = resultText;

  AgentWakeFactsBuilder() {
    AgentWakeFacts._defaults(this);
  }

  AgentWakeFactsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _reason = $v.reason;
      _label = $v.label;
      _resultText = $v.resultText;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentWakeFacts other) {
    _$v = other as _$AgentWakeFacts;
  }

  @override
  void update(void Function(AgentWakeFactsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentWakeFacts build() => _build();

  _$AgentWakeFacts _build() {
    final _$result =
        _$v ??
        _$AgentWakeFacts._(
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'AgentWakeFacts',
            'kind',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'AgentWakeFacts',
            'reason',
          ),
          label: BuiltValueNullFieldError.checkNotNull(
            label,
            r'AgentWakeFacts',
            'label',
          ),
          resultText: BuiltValueNullFieldError.checkNotNull(
            resultText,
            r'AgentWakeFacts',
            'resultText',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
