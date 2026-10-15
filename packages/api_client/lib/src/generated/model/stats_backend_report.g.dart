// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_backend_report.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsBackendReport extends StatsBackendReport {
  @override
  final BuiltList<StatsBackendAgent> items;

  factory _$StatsBackendReport([
    void Function(StatsBackendReportBuilder)? updates,
  ]) => (StatsBackendReportBuilder()..update(updates))._build();

  _$StatsBackendReport._({required this.items}) : super._();
  @override
  StatsBackendReport rebuild(
    void Function(StatsBackendReportBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StatsBackendReportBuilder toBuilder() =>
      StatsBackendReportBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsBackendReport && items == other.items;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'StatsBackendReport',
    )..add('items', items)).toString();
  }
}

class StatsBackendReportBuilder
    implements Builder<StatsBackendReport, StatsBackendReportBuilder> {
  _$StatsBackendReport? _$v;

  ListBuilder<StatsBackendAgent>? _items;
  ListBuilder<StatsBackendAgent> get items =>
      _$this._items ??= ListBuilder<StatsBackendAgent>();
  set items(ListBuilder<StatsBackendAgent>? items) => _$this._items = items;

  StatsBackendReportBuilder() {
    StatsBackendReport._defaults(this);
  }

  StatsBackendReportBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsBackendReport other) {
    _$v = other as _$StatsBackendReport;
  }

  @override
  void update(void Function(StatsBackendReportBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsBackendReport build() => _build();

  _$StatsBackendReport _build() {
    _$StatsBackendReport _$result;
    try {
      _$result = _$v ?? _$StatsBackendReport._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatsBackendReport',
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
