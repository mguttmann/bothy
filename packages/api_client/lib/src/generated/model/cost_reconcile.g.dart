// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cost_reconcile.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CostReconcile extends CostReconcile {
  @override
  final String provider;
  @override
  final DateTime periodStart;
  @override
  final DateTime periodEnd;
  @override
  final BuiltList<ReconcileEntry> entries;

  factory _$CostReconcile([void Function(CostReconcileBuilder)? updates]) =>
      (CostReconcileBuilder()..update(updates))._build();

  _$CostReconcile._({
    required this.provider,
    required this.periodStart,
    required this.periodEnd,
    required this.entries,
  }) : super._();
  @override
  CostReconcile rebuild(void Function(CostReconcileBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CostReconcileBuilder toBuilder() => CostReconcileBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CostReconcile &&
        provider == other.provider &&
        periodStart == other.periodStart &&
        periodEnd == other.periodEnd &&
        entries == other.entries;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, provider.hashCode);
    _$hash = $jc(_$hash, periodStart.hashCode);
    _$hash = $jc(_$hash, periodEnd.hashCode);
    _$hash = $jc(_$hash, entries.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CostReconcile')
          ..add('provider', provider)
          ..add('periodStart', periodStart)
          ..add('periodEnd', periodEnd)
          ..add('entries', entries))
        .toString();
  }
}

class CostReconcileBuilder
    implements Builder<CostReconcile, CostReconcileBuilder> {
  _$CostReconcile? _$v;

  String? _provider;
  String? get provider => _$this._provider;
  set provider(String? provider) => _$this._provider = provider;

  DateTime? _periodStart;
  DateTime? get periodStart => _$this._periodStart;
  set periodStart(DateTime? periodStart) => _$this._periodStart = periodStart;

  DateTime? _periodEnd;
  DateTime? get periodEnd => _$this._periodEnd;
  set periodEnd(DateTime? periodEnd) => _$this._periodEnd = periodEnd;

  ListBuilder<ReconcileEntry>? _entries;
  ListBuilder<ReconcileEntry> get entries =>
      _$this._entries ??= ListBuilder<ReconcileEntry>();
  set entries(ListBuilder<ReconcileEntry>? entries) =>
      _$this._entries = entries;

  CostReconcileBuilder() {
    CostReconcile._defaults(this);
  }

  CostReconcileBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _provider = $v.provider;
      _periodStart = $v.periodStart;
      _periodEnd = $v.periodEnd;
      _entries = $v.entries.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CostReconcile other) {
    _$v = other as _$CostReconcile;
  }

  @override
  void update(void Function(CostReconcileBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CostReconcile build() => _build();

  _$CostReconcile _build() {
    _$CostReconcile _$result;
    try {
      _$result =
          _$v ??
          _$CostReconcile._(
            provider: BuiltValueNullFieldError.checkNotNull(
              provider,
              r'CostReconcile',
              'provider',
            ),
            periodStart: BuiltValueNullFieldError.checkNotNull(
              periodStart,
              r'CostReconcile',
              'periodStart',
            ),
            periodEnd: BuiltValueNullFieldError.checkNotNull(
              periodEnd,
              r'CostReconcile',
              'periodEnd',
            ),
            entries: entries.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'entries';
        entries.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CostReconcile',
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
