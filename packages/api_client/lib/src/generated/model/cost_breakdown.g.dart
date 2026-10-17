// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cost_breakdown.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CostBreakdown extends CostBreakdown {
  @override
  final String threadId;
  @override
  final Money total;
  @override
  final BuiltList<RunCost> runs;

  factory _$CostBreakdown([void Function(CostBreakdownBuilder)? updates]) =>
      (CostBreakdownBuilder()..update(updates))._build();

  _$CostBreakdown._({
    required this.threadId,
    required this.total,
    required this.runs,
  }) : super._();
  @override
  CostBreakdown rebuild(void Function(CostBreakdownBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CostBreakdownBuilder toBuilder() => CostBreakdownBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CostBreakdown &&
        threadId == other.threadId &&
        total == other.total &&
        runs == other.runs;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, total.hashCode);
    _$hash = $jc(_$hash, runs.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CostBreakdown')
          ..add('threadId', threadId)
          ..add('total', total)
          ..add('runs', runs))
        .toString();
  }
}

class CostBreakdownBuilder
    implements Builder<CostBreakdown, CostBreakdownBuilder> {
  _$CostBreakdown? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  MoneyBuilder? _total;
  MoneyBuilder get total => _$this._total ??= MoneyBuilder();
  set total(MoneyBuilder? total) => _$this._total = total;

  ListBuilder<RunCost>? _runs;
  ListBuilder<RunCost> get runs => _$this._runs ??= ListBuilder<RunCost>();
  set runs(ListBuilder<RunCost>? runs) => _$this._runs = runs;

  CostBreakdownBuilder() {
    CostBreakdown._defaults(this);
  }

  CostBreakdownBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _total = $v.total.toBuilder();
      _runs = $v.runs.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CostBreakdown other) {
    _$v = other as _$CostBreakdown;
  }

  @override
  void update(void Function(CostBreakdownBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CostBreakdown build() => _build();

  _$CostBreakdown _build() {
    _$CostBreakdown _$result;
    try {
      _$result =
          _$v ??
          _$CostBreakdown._(
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'CostBreakdown',
              'threadId',
            ),
            total: total.build(),
            runs: runs.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'total';
        total.build();
        _$failedField = 'runs';
        runs.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CostBreakdown',
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
