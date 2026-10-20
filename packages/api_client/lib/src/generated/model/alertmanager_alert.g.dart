// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'alertmanager_alert.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AlertmanagerAlertStatusEnum _$alertmanagerAlertStatusEnum_firing =
    const AlertmanagerAlertStatusEnum._('firing');
const AlertmanagerAlertStatusEnum _$alertmanagerAlertStatusEnum_resolved =
    const AlertmanagerAlertStatusEnum._('resolved');
const AlertmanagerAlertStatusEnum
_$alertmanagerAlertStatusEnum_unknownDefaultOpenApi =
    const AlertmanagerAlertStatusEnum._('unknownDefaultOpenApi');

AlertmanagerAlertStatusEnum _$alertmanagerAlertStatusEnumValueOf(String name) {
  switch (name) {
    case 'firing':
      return _$alertmanagerAlertStatusEnum_firing;
    case 'resolved':
      return _$alertmanagerAlertStatusEnum_resolved;
    case 'unknownDefaultOpenApi':
      return _$alertmanagerAlertStatusEnum_unknownDefaultOpenApi;
    default:
      return _$alertmanagerAlertStatusEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AlertmanagerAlertStatusEnum>
_$alertmanagerAlertStatusEnumValues = BuiltSet<AlertmanagerAlertStatusEnum>(
  const <AlertmanagerAlertStatusEnum>[
    _$alertmanagerAlertStatusEnum_firing,
    _$alertmanagerAlertStatusEnum_resolved,
    _$alertmanagerAlertStatusEnum_unknownDefaultOpenApi,
  ],
);

Serializer<AlertmanagerAlertStatusEnum>
_$alertmanagerAlertStatusEnumSerializer =
    _$AlertmanagerAlertStatusEnumSerializer();

class _$AlertmanagerAlertStatusEnumSerializer
    implements PrimitiveSerializer<AlertmanagerAlertStatusEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'firing': 'firing',
    'resolved': 'resolved',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'firing': 'firing',
    'resolved': 'resolved',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AlertmanagerAlertStatusEnum];
  @override
  final String wireName = 'AlertmanagerAlertStatusEnum';

  @override
  Object serialize(
    Serializers serializers,
    AlertmanagerAlertStatusEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AlertmanagerAlertStatusEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AlertmanagerAlertStatusEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AlertmanagerAlert extends AlertmanagerAlert {
  @override
  final AlertmanagerAlertStatusEnum? status;
  @override
  final BuiltMap<String, JsonObject?>? labels;
  @override
  final BuiltMap<String, JsonObject?>? annotations;
  @override
  final DateTime? startsAt;
  @override
  final DateTime? endsAt;
  @override
  final String? generatorURL;
  @override
  final String? fingerprint;

  factory _$AlertmanagerAlert([
    void Function(AlertmanagerAlertBuilder)? updates,
  ]) => (AlertmanagerAlertBuilder()..update(updates))._build();

  _$AlertmanagerAlert._({
    this.status,
    this.labels,
    this.annotations,
    this.startsAt,
    this.endsAt,
    this.generatorURL,
    this.fingerprint,
  }) : super._();
  @override
  AlertmanagerAlert rebuild(void Function(AlertmanagerAlertBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AlertmanagerAlertBuilder toBuilder() =>
      AlertmanagerAlertBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AlertmanagerAlert &&
        status == other.status &&
        labels == other.labels &&
        annotations == other.annotations &&
        startsAt == other.startsAt &&
        endsAt == other.endsAt &&
        generatorURL == other.generatorURL &&
        fingerprint == other.fingerprint;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, status.hashCode);
    _$hash = $jc(_$hash, labels.hashCode);
    _$hash = $jc(_$hash, annotations.hashCode);
    _$hash = $jc(_$hash, startsAt.hashCode);
    _$hash = $jc(_$hash, endsAt.hashCode);
    _$hash = $jc(_$hash, generatorURL.hashCode);
    _$hash = $jc(_$hash, fingerprint.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AlertmanagerAlert')
          ..add('status', status)
          ..add('labels', labels)
          ..add('annotations', annotations)
          ..add('startsAt', startsAt)
          ..add('endsAt', endsAt)
          ..add('generatorURL', generatorURL)
          ..add('fingerprint', fingerprint))
        .toString();
  }
}

class AlertmanagerAlertBuilder
    implements Builder<AlertmanagerAlert, AlertmanagerAlertBuilder> {
  _$AlertmanagerAlert? _$v;

  AlertmanagerAlertStatusEnum? _status;
  AlertmanagerAlertStatusEnum? get status => _$this._status;
  set status(AlertmanagerAlertStatusEnum? status) => _$this._status = status;

  MapBuilder<String, JsonObject?>? _labels;
  MapBuilder<String, JsonObject?> get labels =>
      _$this._labels ??= MapBuilder<String, JsonObject?>();
  set labels(MapBuilder<String, JsonObject?>? labels) =>
      _$this._labels = labels;

  MapBuilder<String, JsonObject?>? _annotations;
  MapBuilder<String, JsonObject?> get annotations =>
      _$this._annotations ??= MapBuilder<String, JsonObject?>();
  set annotations(MapBuilder<String, JsonObject?>? annotations) =>
      _$this._annotations = annotations;

  DateTime? _startsAt;
  DateTime? get startsAt => _$this._startsAt;
  set startsAt(DateTime? startsAt) => _$this._startsAt = startsAt;

  DateTime? _endsAt;
  DateTime? get endsAt => _$this._endsAt;
  set endsAt(DateTime? endsAt) => _$this._endsAt = endsAt;

  String? _generatorURL;
  String? get generatorURL => _$this._generatorURL;
  set generatorURL(String? generatorURL) => _$this._generatorURL = generatorURL;

  String? _fingerprint;
  String? get fingerprint => _$this._fingerprint;
  set fingerprint(String? fingerprint) => _$this._fingerprint = fingerprint;

  AlertmanagerAlertBuilder() {
    AlertmanagerAlert._defaults(this);
  }

  AlertmanagerAlertBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _status = $v.status;
      _labels = $v.labels?.toBuilder();
      _annotations = $v.annotations?.toBuilder();
      _startsAt = $v.startsAt;
      _endsAt = $v.endsAt;
      _generatorURL = $v.generatorURL;
      _fingerprint = $v.fingerprint;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AlertmanagerAlert other) {
    _$v = other as _$AlertmanagerAlert;
  }

  @override
  void update(void Function(AlertmanagerAlertBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AlertmanagerAlert build() => _build();

  _$AlertmanagerAlert _build() {
    _$AlertmanagerAlert _$result;
    try {
      _$result =
          _$v ??
          _$AlertmanagerAlert._(
            status: status,
            labels: _labels?.build(),
            annotations: _annotations?.build(),
            startsAt: startsAt,
            endsAt: endsAt,
            generatorURL: generatorURL,
            fingerprint: fingerprint,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'labels';
        _labels?.build();
        _$failedField = 'annotations';
        _annotations?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AlertmanagerAlert',
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
