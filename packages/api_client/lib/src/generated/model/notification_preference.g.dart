// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_preference.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const NotificationPreferenceTargetKindEnum
_$notificationPreferenceTargetKindEnum_space =
    const NotificationPreferenceTargetKindEnum._('space');
const NotificationPreferenceTargetKindEnum
_$notificationPreferenceTargetKindEnum_channel =
    const NotificationPreferenceTargetKindEnum._('channel');
const NotificationPreferenceTargetKindEnum
_$notificationPreferenceTargetKindEnum_thread =
    const NotificationPreferenceTargetKindEnum._('thread');
const NotificationPreferenceTargetKindEnum
_$notificationPreferenceTargetKindEnum_unknownDefaultOpenApi =
    const NotificationPreferenceTargetKindEnum._('unknownDefaultOpenApi');

NotificationPreferenceTargetKindEnum
_$notificationPreferenceTargetKindEnumValueOf(String name) {
  switch (name) {
    case 'space':
      return _$notificationPreferenceTargetKindEnum_space;
    case 'channel':
      return _$notificationPreferenceTargetKindEnum_channel;
    case 'thread':
      return _$notificationPreferenceTargetKindEnum_thread;
    case 'unknownDefaultOpenApi':
      return _$notificationPreferenceTargetKindEnum_unknownDefaultOpenApi;
    default:
      return _$notificationPreferenceTargetKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<NotificationPreferenceTargetKindEnum>
_$notificationPreferenceTargetKindEnumValues =
    BuiltSet<NotificationPreferenceTargetKindEnum>(
      const <NotificationPreferenceTargetKindEnum>[
        _$notificationPreferenceTargetKindEnum_space,
        _$notificationPreferenceTargetKindEnum_channel,
        _$notificationPreferenceTargetKindEnum_thread,
        _$notificationPreferenceTargetKindEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<NotificationPreferenceTargetKindEnum>
_$notificationPreferenceTargetKindEnumSerializer =
    _$NotificationPreferenceTargetKindEnumSerializer();

class _$NotificationPreferenceTargetKindEnumSerializer
    implements PrimitiveSerializer<NotificationPreferenceTargetKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'space': 'space',
    'channel': 'channel',
    'thread': 'thread',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'space': 'space',
    'channel': 'channel',
    'thread': 'thread',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    NotificationPreferenceTargetKindEnum,
  ];
  @override
  final String wireName = 'NotificationPreferenceTargetKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    NotificationPreferenceTargetKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  NotificationPreferenceTargetKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => NotificationPreferenceTargetKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$NotificationPreference extends NotificationPreference {
  @override
  final NotificationPreferenceTargetKindEnum targetKind;
  @override
  final String targetId;
  @override
  final NotificationLevel level;
  @override
  final DateTime? mutedUntil;
  @override
  final int version;

  factory _$NotificationPreference([
    void Function(NotificationPreferenceBuilder)? updates,
  ]) => (NotificationPreferenceBuilder()..update(updates))._build();

  _$NotificationPreference._({
    required this.targetKind,
    required this.targetId,
    required this.level,
    this.mutedUntil,
    required this.version,
  }) : super._();
  @override
  NotificationPreference rebuild(
    void Function(NotificationPreferenceBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  NotificationPreferenceBuilder toBuilder() =>
      NotificationPreferenceBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is NotificationPreference &&
        targetKind == other.targetKind &&
        targetId == other.targetId &&
        level == other.level &&
        mutedUntil == other.mutedUntil &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, targetKind.hashCode);
    _$hash = $jc(_$hash, targetId.hashCode);
    _$hash = $jc(_$hash, level.hashCode);
    _$hash = $jc(_$hash, mutedUntil.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'NotificationPreference')
          ..add('targetKind', targetKind)
          ..add('targetId', targetId)
          ..add('level', level)
          ..add('mutedUntil', mutedUntil)
          ..add('version', version))
        .toString();
  }
}

class NotificationPreferenceBuilder
    implements Builder<NotificationPreference, NotificationPreferenceBuilder> {
  _$NotificationPreference? _$v;

  NotificationPreferenceTargetKindEnum? _targetKind;
  NotificationPreferenceTargetKindEnum? get targetKind => _$this._targetKind;
  set targetKind(NotificationPreferenceTargetKindEnum? targetKind) =>
      _$this._targetKind = targetKind;

  String? _targetId;
  String? get targetId => _$this._targetId;
  set targetId(String? targetId) => _$this._targetId = targetId;

  NotificationLevel? _level;
  NotificationLevel? get level => _$this._level;
  set level(NotificationLevel? level) => _$this._level = level;

  DateTime? _mutedUntil;
  DateTime? get mutedUntil => _$this._mutedUntil;
  set mutedUntil(DateTime? mutedUntil) => _$this._mutedUntil = mutedUntil;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  NotificationPreferenceBuilder() {
    NotificationPreference._defaults(this);
  }

  NotificationPreferenceBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _targetKind = $v.targetKind;
      _targetId = $v.targetId;
      _level = $v.level;
      _mutedUntil = $v.mutedUntil;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(NotificationPreference other) {
    _$v = other as _$NotificationPreference;
  }

  @override
  void update(void Function(NotificationPreferenceBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  NotificationPreference build() => _build();

  _$NotificationPreference _build() {
    final _$result =
        _$v ??
        _$NotificationPreference._(
          targetKind: BuiltValueNullFieldError.checkNotNull(
            targetKind,
            r'NotificationPreference',
            'targetKind',
          ),
          targetId: BuiltValueNullFieldError.checkNotNull(
            targetId,
            r'NotificationPreference',
            'targetId',
          ),
          level: BuiltValueNullFieldError.checkNotNull(
            level,
            r'NotificationPreference',
            'level',
          ),
          mutedUntil: mutedUntil,
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'NotificationPreference',
            'version',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
