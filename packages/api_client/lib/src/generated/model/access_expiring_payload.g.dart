// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'access_expiring_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AccessExpiringPayloadKindEnum _$accessExpiringPayloadKindEnum_grant =
    const AccessExpiringPayloadKindEnum._('grant');
const AccessExpiringPayloadKindEnum
_$accessExpiringPayloadKindEnum_groupMembership =
    const AccessExpiringPayloadKindEnum._('groupMembership');
const AccessExpiringPayloadKindEnum
_$accessExpiringPayloadKindEnum_unknownDefaultOpenApi =
    const AccessExpiringPayloadKindEnum._('unknownDefaultOpenApi');

AccessExpiringPayloadKindEnum _$accessExpiringPayloadKindEnumValueOf(
  String name,
) {
  switch (name) {
    case 'grant':
      return _$accessExpiringPayloadKindEnum_grant;
    case 'groupMembership':
      return _$accessExpiringPayloadKindEnum_groupMembership;
    case 'unknownDefaultOpenApi':
      return _$accessExpiringPayloadKindEnum_unknownDefaultOpenApi;
    default:
      return _$accessExpiringPayloadKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AccessExpiringPayloadKindEnum>
_$accessExpiringPayloadKindEnumValues = BuiltSet<AccessExpiringPayloadKindEnum>(
  const <AccessExpiringPayloadKindEnum>[
    _$accessExpiringPayloadKindEnum_grant,
    _$accessExpiringPayloadKindEnum_groupMembership,
    _$accessExpiringPayloadKindEnum_unknownDefaultOpenApi,
  ],
);

Serializer<AccessExpiringPayloadKindEnum>
_$accessExpiringPayloadKindEnumSerializer =
    _$AccessExpiringPayloadKindEnumSerializer();

class _$AccessExpiringPayloadKindEnumSerializer
    implements PrimitiveSerializer<AccessExpiringPayloadKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'grant': 'grant',
    'groupMembership': 'group_membership',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'grant': 'grant',
    'group_membership': 'groupMembership',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AccessExpiringPayloadKindEnum];
  @override
  final String wireName = 'AccessExpiringPayloadKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    AccessExpiringPayloadKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AccessExpiringPayloadKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AccessExpiringPayloadKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AccessExpiringPayload extends AccessExpiringPayload {
  @override
  final AccessExpiringPayloadKindEnum kind;
  @override
  final String? grantId;
  @override
  final String? groupId;
  @override
  final String subjectId;
  @override
  final AccessTarget? target;
  @override
  final DateTime expiresAt;

  factory _$AccessExpiringPayload([
    void Function(AccessExpiringPayloadBuilder)? updates,
  ]) => (AccessExpiringPayloadBuilder()..update(updates))._build();

  _$AccessExpiringPayload._({
    required this.kind,
    this.grantId,
    this.groupId,
    required this.subjectId,
    this.target,
    required this.expiresAt,
  }) : super._();
  @override
  AccessExpiringPayload rebuild(
    void Function(AccessExpiringPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AccessExpiringPayloadBuilder toBuilder() =>
      AccessExpiringPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AccessExpiringPayload &&
        kind == other.kind &&
        grantId == other.grantId &&
        groupId == other.groupId &&
        subjectId == other.subjectId &&
        target == other.target &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, grantId.hashCode);
    _$hash = $jc(_$hash, groupId.hashCode);
    _$hash = $jc(_$hash, subjectId.hashCode);
    _$hash = $jc(_$hash, target.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AccessExpiringPayload')
          ..add('kind', kind)
          ..add('grantId', grantId)
          ..add('groupId', groupId)
          ..add('subjectId', subjectId)
          ..add('target', target)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class AccessExpiringPayloadBuilder
    implements Builder<AccessExpiringPayload, AccessExpiringPayloadBuilder> {
  _$AccessExpiringPayload? _$v;

  AccessExpiringPayloadKindEnum? _kind;
  AccessExpiringPayloadKindEnum? get kind => _$this._kind;
  set kind(AccessExpiringPayloadKindEnum? kind) => _$this._kind = kind;

  String? _grantId;
  String? get grantId => _$this._grantId;
  set grantId(String? grantId) => _$this._grantId = grantId;

  String? _groupId;
  String? get groupId => _$this._groupId;
  set groupId(String? groupId) => _$this._groupId = groupId;

  String? _subjectId;
  String? get subjectId => _$this._subjectId;
  set subjectId(String? subjectId) => _$this._subjectId = subjectId;

  AccessTargetBuilder? _target;
  AccessTargetBuilder get target => _$this._target ??= AccessTargetBuilder();
  set target(AccessTargetBuilder? target) => _$this._target = target;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  AccessExpiringPayloadBuilder() {
    AccessExpiringPayload._defaults(this);
  }

  AccessExpiringPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _grantId = $v.grantId;
      _groupId = $v.groupId;
      _subjectId = $v.subjectId;
      _target = $v.target?.toBuilder();
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AccessExpiringPayload other) {
    _$v = other as _$AccessExpiringPayload;
  }

  @override
  void update(void Function(AccessExpiringPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AccessExpiringPayload build() => _build();

  _$AccessExpiringPayload _build() {
    _$AccessExpiringPayload _$result;
    try {
      _$result =
          _$v ??
          _$AccessExpiringPayload._(
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'AccessExpiringPayload',
              'kind',
            ),
            grantId: grantId,
            groupId: groupId,
            subjectId: BuiltValueNullFieldError.checkNotNull(
              subjectId,
              r'AccessExpiringPayload',
              'subjectId',
            ),
            target: _target?.build(),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'AccessExpiringPayload',
              'expiresAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'target';
        _target?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AccessExpiringPayload',
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
