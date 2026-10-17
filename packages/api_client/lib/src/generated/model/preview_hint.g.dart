// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'preview_hint.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const PreviewHintCodeEnum _$previewHintCodeEnum_readHorizon =
    const PreviewHintCodeEnum._('readHorizon');
const PreviewHintCodeEnum _$previewHintCodeEnum_triggerBlockedToolClass =
    const PreviewHintCodeEnum._('triggerBlockedToolClass');
const PreviewHintCodeEnum _$previewHintCodeEnum_groupReach =
    const PreviewHintCodeEnum._('groupReach');
const PreviewHintCodeEnum _$previewHintCodeEnum_expiryMissing =
    const PreviewHintCodeEnum._('expiryMissing');
const PreviewHintCodeEnum _$previewHintCodeEnum_terminalNotEffective =
    const PreviewHintCodeEnum._('terminalNotEffective');
const PreviewHintCodeEnum _$previewHintCodeEnum_unknownDefaultOpenApi =
    const PreviewHintCodeEnum._('unknownDefaultOpenApi');

PreviewHintCodeEnum _$previewHintCodeEnumValueOf(String name) {
  switch (name) {
    case 'readHorizon':
      return _$previewHintCodeEnum_readHorizon;
    case 'triggerBlockedToolClass':
      return _$previewHintCodeEnum_triggerBlockedToolClass;
    case 'groupReach':
      return _$previewHintCodeEnum_groupReach;
    case 'expiryMissing':
      return _$previewHintCodeEnum_expiryMissing;
    case 'terminalNotEffective':
      return _$previewHintCodeEnum_terminalNotEffective;
    case 'unknownDefaultOpenApi':
      return _$previewHintCodeEnum_unknownDefaultOpenApi;
    default:
      return _$previewHintCodeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<PreviewHintCodeEnum> _$previewHintCodeEnumValues =
    BuiltSet<PreviewHintCodeEnum>(const <PreviewHintCodeEnum>[
      _$previewHintCodeEnum_readHorizon,
      _$previewHintCodeEnum_triggerBlockedToolClass,
      _$previewHintCodeEnum_groupReach,
      _$previewHintCodeEnum_expiryMissing,
      _$previewHintCodeEnum_terminalNotEffective,
      _$previewHintCodeEnum_unknownDefaultOpenApi,
    ]);

Serializer<PreviewHintCodeEnum> _$previewHintCodeEnumSerializer =
    _$PreviewHintCodeEnumSerializer();

class _$PreviewHintCodeEnumSerializer
    implements PrimitiveSerializer<PreviewHintCodeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'readHorizon': 'read_horizon',
    'triggerBlockedToolClass': 'trigger_blocked_tool_class',
    'groupReach': 'group_reach',
    'expiryMissing': 'expiry_missing',
    'terminalNotEffective': 'terminal_not_effective',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'read_horizon': 'readHorizon',
    'trigger_blocked_tool_class': 'triggerBlockedToolClass',
    'group_reach': 'groupReach',
    'expiry_missing': 'expiryMissing',
    'terminal_not_effective': 'terminalNotEffective',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[PreviewHintCodeEnum];
  @override
  final String wireName = 'PreviewHintCodeEnum';

  @override
  Object serialize(
    Serializers serializers,
    PreviewHintCodeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  PreviewHintCodeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => PreviewHintCodeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$PreviewHint extends PreviewHint {
  @override
  final PreviewHintCodeEnum code;
  @override
  final String? agentId;
  @override
  final String? principalId;
  @override
  final String? groupId;
  @override
  final String? spaceId;
  @override
  final String? channelId;
  @override
  final int? count;

  factory _$PreviewHint([void Function(PreviewHintBuilder)? updates]) =>
      (PreviewHintBuilder()..update(updates))._build();

  _$PreviewHint._({
    required this.code,
    this.agentId,
    this.principalId,
    this.groupId,
    this.spaceId,
    this.channelId,
    this.count,
  }) : super._();
  @override
  PreviewHint rebuild(void Function(PreviewHintBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PreviewHintBuilder toBuilder() => PreviewHintBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PreviewHint &&
        code == other.code &&
        agentId == other.agentId &&
        principalId == other.principalId &&
        groupId == other.groupId &&
        spaceId == other.spaceId &&
        channelId == other.channelId &&
        count == other.count;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, code.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, principalId.hashCode);
    _$hash = $jc(_$hash, groupId.hashCode);
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, count.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PreviewHint')
          ..add('code', code)
          ..add('agentId', agentId)
          ..add('principalId', principalId)
          ..add('groupId', groupId)
          ..add('spaceId', spaceId)
          ..add('channelId', channelId)
          ..add('count', count))
        .toString();
  }
}

class PreviewHintBuilder implements Builder<PreviewHint, PreviewHintBuilder> {
  _$PreviewHint? _$v;

  PreviewHintCodeEnum? _code;
  PreviewHintCodeEnum? get code => _$this._code;
  set code(PreviewHintCodeEnum? code) => _$this._code = code;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _principalId;
  String? get principalId => _$this._principalId;
  set principalId(String? principalId) => _$this._principalId = principalId;

  String? _groupId;
  String? get groupId => _$this._groupId;
  set groupId(String? groupId) => _$this._groupId = groupId;

  String? _spaceId;
  String? get spaceId => _$this._spaceId;
  set spaceId(String? spaceId) => _$this._spaceId = spaceId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  int? _count;
  int? get count => _$this._count;
  set count(int? count) => _$this._count = count;

  PreviewHintBuilder() {
    PreviewHint._defaults(this);
  }

  PreviewHintBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _code = $v.code;
      _agentId = $v.agentId;
      _principalId = $v.principalId;
      _groupId = $v.groupId;
      _spaceId = $v.spaceId;
      _channelId = $v.channelId;
      _count = $v.count;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PreviewHint other) {
    _$v = other as _$PreviewHint;
  }

  @override
  void update(void Function(PreviewHintBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PreviewHint build() => _build();

  _$PreviewHint _build() {
    final _$result =
        _$v ??
        _$PreviewHint._(
          code: BuiltValueNullFieldError.checkNotNull(
            code,
            r'PreviewHint',
            'code',
          ),
          agentId: agentId,
          principalId: principalId,
          groupId: groupId,
          spaceId: spaceId,
          channelId: channelId,
          count: count,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
