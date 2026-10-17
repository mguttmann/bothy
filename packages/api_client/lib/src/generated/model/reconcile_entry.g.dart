// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'reconcile_entry.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ReconcileEntry extends ReconcileEntry {
  @override
  final String runId;
  @override
  final int amountMicros;
  @override
  final String currency;

  factory _$ReconcileEntry([void Function(ReconcileEntryBuilder)? updates]) =>
      (ReconcileEntryBuilder()..update(updates))._build();

  _$ReconcileEntry._({
    required this.runId,
    required this.amountMicros,
    required this.currency,
  }) : super._();
  @override
  ReconcileEntry rebuild(void Function(ReconcileEntryBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ReconcileEntryBuilder toBuilder() => ReconcileEntryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ReconcileEntry &&
        runId == other.runId &&
        amountMicros == other.amountMicros &&
        currency == other.currency;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, amountMicros.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ReconcileEntry')
          ..add('runId', runId)
          ..add('amountMicros', amountMicros)
          ..add('currency', currency))
        .toString();
  }
}

class ReconcileEntryBuilder
    implements Builder<ReconcileEntry, ReconcileEntryBuilder> {
  _$ReconcileEntry? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  int? _amountMicros;
  int? get amountMicros => _$this._amountMicros;
  set amountMicros(int? amountMicros) => _$this._amountMicros = amountMicros;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  ReconcileEntryBuilder() {
    ReconcileEntry._defaults(this);
  }

  ReconcileEntryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _amountMicros = $v.amountMicros;
      _currency = $v.currency;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ReconcileEntry other) {
    _$v = other as _$ReconcileEntry;
  }

  @override
  void update(void Function(ReconcileEntryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ReconcileEntry build() => _build();

  _$ReconcileEntry _build() {
    final _$result =
        _$v ??
        _$ReconcileEntry._(
          runId: BuiltValueNullFieldError.checkNotNull(
            runId,
            r'ReconcileEntry',
            'runId',
          ),
          amountMicros: BuiltValueNullFieldError.checkNotNull(
            amountMicros,
            r'ReconcileEntry',
            'amountMicros',
          ),
          currency: BuiltValueNullFieldError.checkNotNull(
            currency,
            r'ReconcileEntry',
            'currency',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
