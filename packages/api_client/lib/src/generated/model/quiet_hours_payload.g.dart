// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'quiet_hours_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$QuietHoursPayload extends QuietHoursPayload {
  @override
  final BuiltList<QuietHoursRule> rules;

  factory _$QuietHoursPayload([
    void Function(QuietHoursPayloadBuilder)? updates,
  ]) => (QuietHoursPayloadBuilder()..update(updates))._build();

  _$QuietHoursPayload._({required this.rules}) : super._();
  @override
  QuietHoursPayload rebuild(void Function(QuietHoursPayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  QuietHoursPayloadBuilder toBuilder() =>
      QuietHoursPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is QuietHoursPayload && rules == other.rules;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, rules.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'QuietHoursPayload',
    )..add('rules', rules)).toString();
  }
}

class QuietHoursPayloadBuilder
    implements Builder<QuietHoursPayload, QuietHoursPayloadBuilder> {
  _$QuietHoursPayload? _$v;

  ListBuilder<QuietHoursRule>? _rules;
  ListBuilder<QuietHoursRule> get rules =>
      _$this._rules ??= ListBuilder<QuietHoursRule>();
  set rules(ListBuilder<QuietHoursRule>? rules) => _$this._rules = rules;

  QuietHoursPayloadBuilder() {
    QuietHoursPayload._defaults(this);
  }

  QuietHoursPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _rules = $v.rules.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(QuietHoursPayload other) {
    _$v = other as _$QuietHoursPayload;
  }

  @override
  void update(void Function(QuietHoursPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  QuietHoursPayload build() => _build();

  _$QuietHoursPayload _build() {
    _$QuietHoursPayload _$result;
    try {
      _$result = _$v ?? _$QuietHoursPayload._(rules: rules.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'rules';
        rules.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'QuietHoursPayload',
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
