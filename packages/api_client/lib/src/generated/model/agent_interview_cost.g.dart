// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_interview_cost.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentInterviewCost extends AgentInterviewCost {
  @override
  final int amountMicros;
  @override
  final String currency;

  factory _$AgentInterviewCost([
    void Function(AgentInterviewCostBuilder)? updates,
  ]) => (AgentInterviewCostBuilder()..update(updates))._build();

  _$AgentInterviewCost._({required this.amountMicros, required this.currency})
    : super._();
  @override
  AgentInterviewCost rebuild(
    void Function(AgentInterviewCostBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentInterviewCostBuilder toBuilder() =>
      AgentInterviewCostBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentInterviewCost &&
        amountMicros == other.amountMicros &&
        currency == other.currency;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, amountMicros.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentInterviewCost')
          ..add('amountMicros', amountMicros)
          ..add('currency', currency))
        .toString();
  }
}

class AgentInterviewCostBuilder
    implements Builder<AgentInterviewCost, AgentInterviewCostBuilder> {
  _$AgentInterviewCost? _$v;

  int? _amountMicros;
  int? get amountMicros => _$this._amountMicros;
  set amountMicros(int? amountMicros) => _$this._amountMicros = amountMicros;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  AgentInterviewCostBuilder() {
    AgentInterviewCost._defaults(this);
  }

  AgentInterviewCostBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _amountMicros = $v.amountMicros;
      _currency = $v.currency;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentInterviewCost other) {
    _$v = other as _$AgentInterviewCost;
  }

  @override
  void update(void Function(AgentInterviewCostBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentInterviewCost build() => _build();

  _$AgentInterviewCost _build() {
    final _$result =
        _$v ??
        _$AgentInterviewCost._(
          amountMicros: BuiltValueNullFieldError.checkNotNull(
            amountMicros,
            r'AgentInterviewCost',
            'amountMicros',
          ),
          currency: BuiltValueNullFieldError.checkNotNull(
            currency,
            r'AgentInterviewCost',
            'currency',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
