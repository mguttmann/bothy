// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'quiet_hours_rule.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$QuietHoursRule extends QuietHoursRule {
  @override
  final String id;
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
  @override
  final int version;

  factory _$QuietHoursRule([void Function(QuietHoursRuleBuilder)? updates]) =>
      (QuietHoursRuleBuilder()..update(updates))._build();

  _$QuietHoursRule._({
    required this.id,
    required this.weekdays,
    required this.startLocal,
    required this.endLocal,
    this.timeZone,
    required this.enabled,
    required this.version,
  }) : super._();
  @override
  QuietHoursRule rebuild(void Function(QuietHoursRuleBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  QuietHoursRuleBuilder toBuilder() => QuietHoursRuleBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is QuietHoursRule &&
        id == other.id &&
        weekdays == other.weekdays &&
        startLocal == other.startLocal &&
        endLocal == other.endLocal &&
        timeZone == other.timeZone &&
        enabled == other.enabled &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, weekdays.hashCode);
    _$hash = $jc(_$hash, startLocal.hashCode);
    _$hash = $jc(_$hash, endLocal.hashCode);
    _$hash = $jc(_$hash, timeZone.hashCode);
    _$hash = $jc(_$hash, enabled.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'QuietHoursRule')
          ..add('id', id)
          ..add('weekdays', weekdays)
          ..add('startLocal', startLocal)
          ..add('endLocal', endLocal)
          ..add('timeZone', timeZone)
          ..add('enabled', enabled)
          ..add('version', version))
        .toString();
  }
}

class QuietHoursRuleBuilder
    implements Builder<QuietHoursRule, QuietHoursRuleBuilder> {
  _$QuietHoursRule? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

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

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  QuietHoursRuleBuilder() {
    QuietHoursRule._defaults(this);
  }

  QuietHoursRuleBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _weekdays = $v.weekdays;
      _startLocal = $v.startLocal;
      _endLocal = $v.endLocal;
      _timeZone = $v.timeZone;
      _enabled = $v.enabled;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(QuietHoursRule other) {
    _$v = other as _$QuietHoursRule;
  }

  @override
  void update(void Function(QuietHoursRuleBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  QuietHoursRule build() => _build();

  _$QuietHoursRule _build() {
    final _$result =
        _$v ??
        _$QuietHoursRule._(
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'QuietHoursRule',
            'id',
          ),
          weekdays: BuiltValueNullFieldError.checkNotNull(
            weekdays,
            r'QuietHoursRule',
            'weekdays',
          ),
          startLocal: BuiltValueNullFieldError.checkNotNull(
            startLocal,
            r'QuietHoursRule',
            'startLocal',
          ),
          endLocal: BuiltValueNullFieldError.checkNotNull(
            endLocal,
            r'QuietHoursRule',
            'endLocal',
          ),
          timeZone: timeZone,
          enabled: BuiltValueNullFieldError.checkNotNull(
            enabled,
            r'QuietHoursRule',
            'enabled',
          ),
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'QuietHoursRule',
            'version',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
