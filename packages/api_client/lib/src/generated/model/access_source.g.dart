// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'access_source.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AccessSourceKindEnum _$accessSourceKindEnum_owner =
    const AccessSourceKindEnum._('owner');
const AccessSourceKindEnum _$accessSourceKindEnum_direct =
    const AccessSourceKindEnum._('direct');
const AccessSourceKindEnum _$accessSourceKindEnum_group =
    const AccessSourceKindEnum._('group');
const AccessSourceKindEnum _$accessSourceKindEnum_inherited =
    const AccessSourceKindEnum._('inherited');
const AccessSourceKindEnum _$accessSourceKindEnum_dmParticipant =
    const AccessSourceKindEnum._('dmParticipant');
const AccessSourceKindEnum _$accessSourceKindEnum_operator_ =
    const AccessSourceKindEnum._('operator_');
const AccessSourceKindEnum _$accessSourceKindEnum_viaAgent =
    const AccessSourceKindEnum._('viaAgent');
const AccessSourceKindEnum _$accessSourceKindEnum_guardBlocked =
    const AccessSourceKindEnum._('guardBlocked');
const AccessSourceKindEnum _$accessSourceKindEnum_unknownDefaultOpenApi =
    const AccessSourceKindEnum._('unknownDefaultOpenApi');

AccessSourceKindEnum _$accessSourceKindEnumValueOf(String name) {
  switch (name) {
    case 'owner':
      return _$accessSourceKindEnum_owner;
    case 'direct':
      return _$accessSourceKindEnum_direct;
    case 'group':
      return _$accessSourceKindEnum_group;
    case 'inherited':
      return _$accessSourceKindEnum_inherited;
    case 'dmParticipant':
      return _$accessSourceKindEnum_dmParticipant;
    case 'operator_':
      return _$accessSourceKindEnum_operator_;
    case 'viaAgent':
      return _$accessSourceKindEnum_viaAgent;
    case 'guardBlocked':
      return _$accessSourceKindEnum_guardBlocked;
    case 'unknownDefaultOpenApi':
      return _$accessSourceKindEnum_unknownDefaultOpenApi;
    default:
      return _$accessSourceKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AccessSourceKindEnum> _$accessSourceKindEnumValues =
    BuiltSet<AccessSourceKindEnum>(const <AccessSourceKindEnum>[
      _$accessSourceKindEnum_owner,
      _$accessSourceKindEnum_direct,
      _$accessSourceKindEnum_group,
      _$accessSourceKindEnum_inherited,
      _$accessSourceKindEnum_dmParticipant,
      _$accessSourceKindEnum_operator_,
      _$accessSourceKindEnum_viaAgent,
      _$accessSourceKindEnum_guardBlocked,
      _$accessSourceKindEnum_unknownDefaultOpenApi,
    ]);

const AccessSourceRuleEnum _$accessSourceRuleEnum_sr2 =
    const AccessSourceRuleEnum._('sr2');
const AccessSourceRuleEnum _$accessSourceRuleEnum_sr3 =
    const AccessSourceRuleEnum._('sr3');
const AccessSourceRuleEnum _$accessSourceRuleEnum_unknownDefaultOpenApi =
    const AccessSourceRuleEnum._('unknownDefaultOpenApi');

AccessSourceRuleEnum _$accessSourceRuleEnumValueOf(String name) {
  switch (name) {
    case 'sr2':
      return _$accessSourceRuleEnum_sr2;
    case 'sr3':
      return _$accessSourceRuleEnum_sr3;
    case 'unknownDefaultOpenApi':
      return _$accessSourceRuleEnum_unknownDefaultOpenApi;
    default:
      return _$accessSourceRuleEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AccessSourceRuleEnum> _$accessSourceRuleEnumValues =
    BuiltSet<AccessSourceRuleEnum>(const <AccessSourceRuleEnum>[
      _$accessSourceRuleEnum_sr2,
      _$accessSourceRuleEnum_sr3,
      _$accessSourceRuleEnum_unknownDefaultOpenApi,
    ]);

Serializer<AccessSourceKindEnum> _$accessSourceKindEnumSerializer =
    _$AccessSourceKindEnumSerializer();
Serializer<AccessSourceRuleEnum> _$accessSourceRuleEnumSerializer =
    _$AccessSourceRuleEnumSerializer();

class _$AccessSourceKindEnumSerializer
    implements PrimitiveSerializer<AccessSourceKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'owner': 'owner',
    'direct': 'direct',
    'group': 'group',
    'inherited': 'inherited',
    'dmParticipant': 'dm_participant',
    'operator_': 'operator',
    'viaAgent': 'via_agent',
    'guardBlocked': 'guard_blocked',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'owner': 'owner',
    'direct': 'direct',
    'group': 'group',
    'inherited': 'inherited',
    'dm_participant': 'dmParticipant',
    'operator': 'operator_',
    'via_agent': 'viaAgent',
    'guard_blocked': 'guardBlocked',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AccessSourceKindEnum];
  @override
  final String wireName = 'AccessSourceKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    AccessSourceKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AccessSourceKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AccessSourceKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AccessSourceRuleEnumSerializer
    implements PrimitiveSerializer<AccessSourceRuleEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'sr2': 'sr2',
    'sr3': 'sr3',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'sr2': 'sr2',
    'sr3': 'sr3',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AccessSourceRuleEnum];
  @override
  final String wireName = 'AccessSourceRuleEnum';

  @override
  Object serialize(
    Serializers serializers,
    AccessSourceRuleEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AccessSourceRuleEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AccessSourceRuleEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AccessSource extends AccessSource {
  @override
  final AccessSourceKindEnum kind;
  @override
  final String? grantId;
  @override
  final String? groupId;
  @override
  final TargetKind? fromTargetKind;
  @override
  final String? fromTargetId;
  @override
  final String? agentId;
  @override
  final AccessSourceRuleEnum? rule;
  @override
  final DateTime? expiresAt;
  @override
  final DateTime? membershipExpiresAt;
  @override
  final String? reason;

  factory _$AccessSource([void Function(AccessSourceBuilder)? updates]) =>
      (AccessSourceBuilder()..update(updates))._build();

  _$AccessSource._({
    required this.kind,
    this.grantId,
    this.groupId,
    this.fromTargetKind,
    this.fromTargetId,
    this.agentId,
    this.rule,
    this.expiresAt,
    this.membershipExpiresAt,
    this.reason,
  }) : super._();
  @override
  AccessSource rebuild(void Function(AccessSourceBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AccessSourceBuilder toBuilder() => AccessSourceBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AccessSource &&
        kind == other.kind &&
        grantId == other.grantId &&
        groupId == other.groupId &&
        fromTargetKind == other.fromTargetKind &&
        fromTargetId == other.fromTargetId &&
        agentId == other.agentId &&
        rule == other.rule &&
        expiresAt == other.expiresAt &&
        membershipExpiresAt == other.membershipExpiresAt &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, grantId.hashCode);
    _$hash = $jc(_$hash, groupId.hashCode);
    _$hash = $jc(_$hash, fromTargetKind.hashCode);
    _$hash = $jc(_$hash, fromTargetId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, rule.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jc(_$hash, membershipExpiresAt.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AccessSource')
          ..add('kind', kind)
          ..add('grantId', grantId)
          ..add('groupId', groupId)
          ..add('fromTargetKind', fromTargetKind)
          ..add('fromTargetId', fromTargetId)
          ..add('agentId', agentId)
          ..add('rule', rule)
          ..add('expiresAt', expiresAt)
          ..add('membershipExpiresAt', membershipExpiresAt)
          ..add('reason', reason))
        .toString();
  }
}

class AccessSourceBuilder
    implements Builder<AccessSource, AccessSourceBuilder> {
  _$AccessSource? _$v;

  AccessSourceKindEnum? _kind;
  AccessSourceKindEnum? get kind => _$this._kind;
  set kind(AccessSourceKindEnum? kind) => _$this._kind = kind;

  String? _grantId;
  String? get grantId => _$this._grantId;
  set grantId(String? grantId) => _$this._grantId = grantId;

  String? _groupId;
  String? get groupId => _$this._groupId;
  set groupId(String? groupId) => _$this._groupId = groupId;

  TargetKind? _fromTargetKind;
  TargetKind? get fromTargetKind => _$this._fromTargetKind;
  set fromTargetKind(TargetKind? fromTargetKind) =>
      _$this._fromTargetKind = fromTargetKind;

  String? _fromTargetId;
  String? get fromTargetId => _$this._fromTargetId;
  set fromTargetId(String? fromTargetId) => _$this._fromTargetId = fromTargetId;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  AccessSourceRuleEnum? _rule;
  AccessSourceRuleEnum? get rule => _$this._rule;
  set rule(AccessSourceRuleEnum? rule) => _$this._rule = rule;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  DateTime? _membershipExpiresAt;
  DateTime? get membershipExpiresAt => _$this._membershipExpiresAt;
  set membershipExpiresAt(DateTime? membershipExpiresAt) =>
      _$this._membershipExpiresAt = membershipExpiresAt;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  AccessSourceBuilder() {
    AccessSource._defaults(this);
  }

  AccessSourceBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _grantId = $v.grantId;
      _groupId = $v.groupId;
      _fromTargetKind = $v.fromTargetKind;
      _fromTargetId = $v.fromTargetId;
      _agentId = $v.agentId;
      _rule = $v.rule;
      _expiresAt = $v.expiresAt;
      _membershipExpiresAt = $v.membershipExpiresAt;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AccessSource other) {
    _$v = other as _$AccessSource;
  }

  @override
  void update(void Function(AccessSourceBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AccessSource build() => _build();

  _$AccessSource _build() {
    final _$result =
        _$v ??
        _$AccessSource._(
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'AccessSource',
            'kind',
          ),
          grantId: grantId,
          groupId: groupId,
          fromTargetKind: fromTargetKind,
          fromTargetId: fromTargetId,
          agentId: agentId,
          rule: rule,
          expiresAt: expiresAt,
          membershipExpiresAt: membershipExpiresAt,
          reason: reason,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
