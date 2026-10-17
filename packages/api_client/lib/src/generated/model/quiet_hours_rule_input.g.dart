// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'quiet_hours_rule_input.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$QuietHoursRuleInput extends QuietHoursRuleInput {
  @override
  final int weekdays;
  @override
  final String startLocal;
  @override
  final String endLocal;
  @override
  final String? timeZone;
  @override
  final bool enabled;

  factory _$QuietHoursRuleInput([
    void Function(QuietHoursRuleInputBuilder)? updates,
  ]) => (QuietHoursRuleInputBuilder()..update(updates))._build();

  _$QuietHoursRuleInput._({
    required this.weekdays,
    required this.startLocal,
    required this.endLocal,
    this.timeZone,
    required this.enabled,
  }) : super._();
  @override
  QuietHoursRuleInput rebuild(
    void Function(QuietHoursRuleInputBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  QuietHoursRuleInputBuilder toBuilder() =>
      QuietHoursRuleInputBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is QuietHoursRuleInput &&
        weekdays == other.weekdays &&
        startLocal == other.startLocal &&
        endLocal == other.endLocal &&
        timeZone == other.timeZone &&
        enabled == other.enabled;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, weekdays.hashCode);
    _$hash = $jc(_$hash, startLocal.hashCode);
    _$hash = $jc(_$hash, endLocal.hashCode);
    _$hash = $jc(_$hash, timeZone.hashCode);
    _$hash = $jc(_$hash, enabled.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'QuietHoursRuleInput')
          ..add('weekdays', weekdays)
          ..add('startLocal', startLocal)
          ..add('endLocal', endLocal)
          ..add('timeZone', timeZone)
          ..add('enabled', enabled))
        .toString();
  }
}

class QuietHoursRuleInputBuilder
    implements Builder<QuietHoursRuleInput, QuietHoursRuleInputBuilder> {
  _$QuietHoursRuleInput? _$v;

  int? _weekdays;
  int? get weekdays => _$this._weekdays;
  set weekdays(int? weekdays) => _$this._weekdays = weekdays;

  String? _startLocal;
  String? get startLocal => _$this._startLocal;
  set startLocal(String? startLocal) => _$this._startLocal = startLocal;

  String? _endLocal;
  String? get endLocal => _$this._endLocal;
  set endLocal(String? endLocal) => _$this._endLocal = endLocal;

  String? _timeZone;
  String? get timeZone => _$this._timeZone;
  set timeZone(String? timeZone) => _$this._timeZone = timeZone;

  bool? _enabled;
  bool? get enabled => _$this._enabled;
  set enabled(bool? enabled) => _$this._enabled = enabled;

  QuietHoursRuleInputBuilder() {
    QuietHoursRuleInput._defaults(this);
  }

  QuietHoursRuleInputBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _weekdays = $v.weekdays;
      _startLocal = $v.startLocal;
      _endLocal = $v.endLocal;
      _timeZone = $v.timeZone;
      _enabled = $v.enabled;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(QuietHoursRuleInput other) {
    _$v = other as _$QuietHoursRuleInput;
  }

  @override
  void update(void Function(QuietHoursRuleInputBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  QuietHoursRuleInput build() => _build();

  _$QuietHoursRuleInput _build() {
    final _$result =
        _$v ??
        _$QuietHoursRuleInput._(
          weekdays: BuiltValueNullFieldError.checkNotNull(
            weekdays,
            r'QuietHoursRuleInput',
            'weekdays',
          ),
          startLocal: BuiltValueNullFieldError.checkNotNull(
            startLocal,
            r'QuietHoursRuleInput',
            'startLocal',
          ),
          endLocal: BuiltValueNullFieldError.checkNotNull(
            endLocal,
            r'QuietHoursRuleInput',
            'endLocal',
          ),
          timeZone: timeZone,
          enabled: BuiltValueNullFieldError.checkNotNull(
            enabled,
            r'QuietHoursRuleInput',
            'enabled',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
