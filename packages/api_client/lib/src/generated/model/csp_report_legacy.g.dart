// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'csp_report_legacy.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CspReportLegacy extends CspReportLegacy {
  @override
  final BuiltMap<String, JsonObject?> cspReport;

  factory _$CspReportLegacy([void Function(CspReportLegacyBuilder)? updates]) =>
      (CspReportLegacyBuilder()..update(updates))._build();

  _$CspReportLegacy._({required this.cspReport}) : super._();
  @override
  CspReportLegacy rebuild(void Function(CspReportLegacyBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CspReportLegacyBuilder toBuilder() => CspReportLegacyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CspReportLegacy && cspReport == other.cspReport;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, cspReport.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'CspReportLegacy',
    )..add('cspReport', cspReport)).toString();
  }
}

class CspReportLegacyBuilder
    implements Builder<CspReportLegacy, CspReportLegacyBuilder> {
  _$CspReportLegacy? _$v;

  MapBuilder<String, JsonObject?>? _cspReport;
  MapBuilder<String, JsonObject?> get cspReport =>
      _$this._cspReport ??= MapBuilder<String, JsonObject?>();
  set cspReport(MapBuilder<String, JsonObject?>? cspReport) =>
      _$this._cspReport = cspReport;

  CspReportLegacyBuilder() {
    CspReportLegacy._defaults(this);
  }

  CspReportLegacyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _cspReport = $v.cspReport.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CspReportLegacy other) {
    _$v = other as _$CspReportLegacy;
  }

  @override
  void update(void Function(CspReportLegacyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CspReportLegacy build() => _build();

  _$CspReportLegacy _build() {
    _$CspReportLegacy _$result;
    try {
      _$result = _$v ?? _$CspReportLegacy._(cspReport: cspReport.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'cspReport';
        cspReport.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CspReportLegacy',
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
