// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'permission_hint.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const PermissionHintRulesEnum _$permissionHintRulesEnum_sr2 =
    const PermissionHintRulesEnum._('sr2');
const PermissionHintRulesEnum _$permissionHintRulesEnum_sr3 =
    const PermissionHintRulesEnum._('sr3');
const PermissionHintRulesEnum _$permissionHintRulesEnum_unknownDefaultOpenApi =
    const PermissionHintRulesEnum._('unknownDefaultOpenApi');

PermissionHintRulesEnum _$permissionHintRulesEnumValueOf(String name) {
  switch (name) {
    case 'sr2':
      return _$permissionHintRulesEnum_sr2;
    case 'sr3':
      return _$permissionHintRulesEnum_sr3;
    case 'unknownDefaultOpenApi':
      return _$permissionHintRulesEnum_unknownDefaultOpenApi;
    default:
      return _$permissionHintRulesEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<PermissionHintRulesEnum> _$permissionHintRulesEnumValues =
    BuiltSet<PermissionHintRulesEnum>(const <PermissionHintRulesEnum>[
      _$permissionHintRulesEnum_sr2,
      _$permissionHintRulesEnum_sr3,
      _$permissionHintRulesEnum_unknownDefaultOpenApi,
    ]);

Serializer<PermissionHintRulesEnum> _$permissionHintRulesEnumSerializer =
    _$PermissionHintRulesEnumSerializer();

class _$PermissionHintRulesEnumSerializer
    implements PrimitiveSerializer<PermissionHintRulesEnum> {
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
  final Iterable<Type> types = const <Type>[PermissionHintRulesEnum];
  @override
  final String wireName = 'PermissionHintRulesEnum';

  @override
  Object serialize(
    Serializers serializers,
    PermissionHintRulesEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  PermissionHintRulesEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => PermissionHintRulesEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$PermissionHint extends PermissionHint {
  @override
  final PermissionHintKind kind;
  @override
  final String? agentId;
  @override
  final String? personId;
  @override
  final String? spaceId;
  @override
  final String? channelId;
  @override
  final Grant? grant;
  @override
  final BuiltList<PermissionHintRulesEnum>? rules;
  @override
  final String? credentialHolderId;
  @override
  final BuiltList<String>? triggerSet;

  factory _$PermissionHint([void Function(PermissionHintBuilder)? updates]) =>
      (PermissionHintBuilder()..update(updates))._build();

  _$PermissionHint._({
    required this.kind,
    this.agentId,
    this.personId,
    this.spaceId,
    this.channelId,
    this.grant,
    this.rules,
    this.credentialHolderId,
    this.triggerSet,
  }) : super._();
  @override
  PermissionHint rebuild(void Function(PermissionHintBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PermissionHintBuilder toBuilder() => PermissionHintBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PermissionHint &&
        kind == other.kind &&
        agentId == other.agentId &&
        personId == other.personId &&
        spaceId == other.spaceId &&
        channelId == other.channelId &&
        grant == other.grant &&
        rules == other.rules &&
        credentialHolderId == other.credentialHolderId &&
        triggerSet == other.triggerSet;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, personId.hashCode);
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, grant.hashCode);
    _$hash = $jc(_$hash, rules.hashCode);
    _$hash = $jc(_$hash, credentialHolderId.hashCode);
    _$hash = $jc(_$hash, triggerSet.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PermissionHint')
          ..add('kind', kind)
          ..add('agentId', agentId)
          ..add('personId', personId)
          ..add('spaceId', spaceId)
          ..add('channelId', channelId)
          ..add('grant', grant)
          ..add('rules', rules)
          ..add('credentialHolderId', credentialHolderId)
          ..add('triggerSet', triggerSet))
        .toString();
  }
}

class PermissionHintBuilder
    implements Builder<PermissionHint, PermissionHintBuilder> {
  _$PermissionHint? _$v;

  PermissionHintKind? _kind;
  PermissionHintKind? get kind => _$this._kind;
  set kind(PermissionHintKind? kind) => _$this._kind = kind;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _personId;
  String? get personId => _$this._personId;
  set personId(String? personId) => _$this._personId = personId;

  String? _spaceId;
  String? get spaceId => _$this._spaceId;
  set spaceId(String? spaceId) => _$this._spaceId = spaceId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  GrantBuilder? _grant;
  GrantBuilder get grant => _$this._grant ??= GrantBuilder();
  set grant(GrantBuilder? grant) => _$this._grant = grant;

  ListBuilder<PermissionHintRulesEnum>? _rules;
  ListBuilder<PermissionHintRulesEnum> get rules =>
      _$this._rules ??= ListBuilder<PermissionHintRulesEnum>();
  set rules(ListBuilder<PermissionHintRulesEnum>? rules) =>
      _$this._rules = rules;

  String? _credentialHolderId;
  String? get credentialHolderId => _$this._credentialHolderId;
  set credentialHolderId(String? credentialHolderId) =>
      _$this._credentialHolderId = credentialHolderId;

  ListBuilder<String>? _triggerSet;
  ListBuilder<String> get triggerSet =>
      _$this._triggerSet ??= ListBuilder<String>();
  set triggerSet(ListBuilder<String>? triggerSet) =>
      _$this._triggerSet = triggerSet;

  PermissionHintBuilder() {
    PermissionHint._defaults(this);
  }

  PermissionHintBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _agentId = $v.agentId;
      _personId = $v.personId;
      _spaceId = $v.spaceId;
      _channelId = $v.channelId;
      _grant = $v.grant?.toBuilder();
      _rules = $v.rules?.toBuilder();
      _credentialHolderId = $v.credentialHolderId;
      _triggerSet = $v.triggerSet?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PermissionHint other) {
    _$v = other as _$PermissionHint;
  }

  @override
  void update(void Function(PermissionHintBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PermissionHint build() => _build();

  _$PermissionHint _build() {
    _$PermissionHint _$result;
    try {
      _$result =
          _$v ??
          _$PermissionHint._(
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'PermissionHint',
              'kind',
            ),
            agentId: agentId,
            personId: personId,
            spaceId: spaceId,
            channelId: channelId,
            grant: _grant?.build(),
            rules: _rules?.build(),
            credentialHolderId: credentialHolderId,
            triggerSet: _triggerSet?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'grant';
        _grant?.build();
        _$failedField = 'rules';
        _rules?.build();

        _$failedField = 'triggerSet';
        _triggerSet?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'PermissionHint',
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
