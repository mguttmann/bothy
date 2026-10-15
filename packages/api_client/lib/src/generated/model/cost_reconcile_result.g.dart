// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cost_reconcile_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CostReconcileResult extends CostReconcileResult {
  @override
  final int reconciled;
  @override
  final BuiltList<String> unmatchedRunIds;

  factory _$CostReconcileResult([
    void Function(CostReconcileResultBuilder)? updates,
  ]) => (CostReconcileResultBuilder()..update(updates))._build();

  _$CostReconcileResult._({
    required this.reconciled,
    required this.unmatchedRunIds,
  }) : super._();
  @override
  CostReconcileResult rebuild(
    void Function(CostReconcileResultBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  CostReconcileResultBuilder toBuilder() =>
      CostReconcileResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CostReconcileResult &&
        reconciled == other.reconciled &&
        unmatchedRunIds == other.unmatchedRunIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, reconciled.hashCode);
    _$hash = $jc(_$hash, unmatchedRunIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CostReconcileResult')
          ..add('reconciled', reconciled)
          ..add('unmatchedRunIds', unmatchedRunIds))
        .toString();
  }
}

class CostReconcileResultBuilder
    implements Builder<CostReconcileResult, CostReconcileResultBuilder> {
  _$CostReconcileResult? _$v;

  int? _reconciled;
  int? get reconciled => _$this._reconciled;
  set reconciled(int? reconciled) => _$this._reconciled = reconciled;

  ListBuilder<String>? _unmatchedRunIds;
  ListBuilder<String> get unmatchedRunIds =>
      _$this._unmatchedRunIds ??= ListBuilder<String>();
  set unmatchedRunIds(ListBuilder<String>? unmatchedRunIds) =>
      _$this._unmatchedRunIds = unmatchedRunIds;

  CostReconcileResultBuilder() {
    CostReconcileResult._defaults(this);
  }

  CostReconcileResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _reconciled = $v.reconciled;
      _unmatchedRunIds = $v.unmatchedRunIds.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CostReconcileResult other) {
    _$v = other as _$CostReconcileResult;
  }

  @override
  void update(void Function(CostReconcileResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CostReconcileResult build() => _build();

  _$CostReconcileResult _build() {
    _$CostReconcileResult _$result;
    try {
      _$result =
          _$v ??
          _$CostReconcileResult._(
            reconciled: BuiltValueNullFieldError.checkNotNull(
              reconciled,
              r'CostReconcileResult',
              'reconciled',
            ),
            unmatchedRunIds: unmatchedRunIds.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'unmatchedRunIds';
        unmatchedRunIds.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CostReconcileResult',
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
