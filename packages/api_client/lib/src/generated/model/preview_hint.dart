//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'preview_hint.g.dart';

/// `terminal_not_effective` (grant on a host, 07, 2.12 point 4) says that SR10 or SR12 keep the grant from taking effect: with `principal_id` for a user subject, with `group_id` and `count` (members in effect it does not reach, absent when 0) for a group subject.
///
/// Properties:
/// * [code]
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [principalId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [groupId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [spaceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [count]
@BuiltValue()
abstract class PreviewHint implements Built<PreviewHint, PreviewHintBuilder> {
  @BuiltValueField(wireName: r'code')
  PreviewHintCodeEnum get code;
  // enum codeEnum {  read_horizon,  trigger_blocked_tool_class,  group_reach,  expiry_missing,  terminal_not_effective,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String? get agentId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'principal_id')
  String? get principalId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'group_id')
  String? get groupId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'space_id')
  String? get spaceId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String? get channelId;

  @BuiltValueField(wireName: r'count')
  int? get count;

  PreviewHint._();

  factory PreviewHint([void updates(PreviewHintBuilder b)]) = _$PreviewHint;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PreviewHintBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PreviewHint> get serializer => _$PreviewHintSerializer();
}

class _$PreviewHintSerializer implements PrimitiveSerializer<PreviewHint> {
  @override
  final Iterable<Type> types = const [PreviewHint, _$PreviewHint];

  @override
  final String wireName = r'PreviewHint';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PreviewHint object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'code';
    yield serializers.serialize(
      object.code,
      specifiedType: const FullType(PreviewHintCodeEnum),
    );
    if (object.agentId != null) {
      yield r'agent_id';
      yield serializers.serialize(
        object.agentId,
        specifiedType: const FullType(String),
      );
    }
    if (object.principalId != null) {
      yield r'principal_id';
      yield serializers.serialize(
        object.principalId,
        specifiedType: const FullType(String),
      );
    }
    if (object.groupId != null) {
      yield r'group_id';
      yield serializers.serialize(
        object.groupId,
        specifiedType: const FullType(String),
      );
    }
    if (object.spaceId != null) {
      yield r'space_id';
      yield serializers.serialize(
        object.spaceId,
        specifiedType: const FullType(String),
      );
    }
    if (object.channelId != null) {
      yield r'channel_id';
      yield serializers.serialize(
        object.channelId,
        specifiedType: const FullType(String),
      );
    }
    if (object.count != null) {
      yield r'count';
      yield serializers.serialize(
        object.count,
        specifiedType: const FullType(int),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    PreviewHint object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return _serializeProperties(
      serializers,
      object,
      specifiedType: specifiedType,
    ).toList();
  }

  void _deserializeProperties(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
    required List<Object?> serializedList,
    required PreviewHintBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PreviewHintCodeEnum),
          ) as PreviewHintCodeEnum;
          result.code = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.agentId = valueDes;
          break;
        case r'principal_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.principalId = valueDes;
          break;
        case r'group_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.groupId = valueDes;
          break;
        case r'space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.spaceId = valueDes;
          break;
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.channelId = valueDes;
          break;
        case r'count':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.count = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PreviewHint deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PreviewHintBuilder();
    final serializedList = (serialized as Iterable<Object?>).toList();
    final unhandled = <Object?>[];
    _deserializeProperties(
      serializers,
      serialized,
      specifiedType: specifiedType,
      serializedList: serializedList,
      unhandled: unhandled,
      result: result,
    );
    return result.build();
  }
}

class PreviewHintCodeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'read_horizon')
  static const PreviewHintCodeEnum readHorizon =
      _$previewHintCodeEnum_readHorizon;
  @BuiltValueEnumConst(wireName: r'trigger_blocked_tool_class')
  static const PreviewHintCodeEnum triggerBlockedToolClass =
      _$previewHintCodeEnum_triggerBlockedToolClass;
  @BuiltValueEnumConst(wireName: r'group_reach')
  static const PreviewHintCodeEnum groupReach =
      _$previewHintCodeEnum_groupReach;
  @BuiltValueEnumConst(wireName: r'expiry_missing')
  static const PreviewHintCodeEnum expiryMissing =
      _$previewHintCodeEnum_expiryMissing;
  @BuiltValueEnumConst(wireName: r'terminal_not_effective')
  static const PreviewHintCodeEnum terminalNotEffective =
      _$previewHintCodeEnum_terminalNotEffective;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const PreviewHintCodeEnum unknownDefaultOpenApi =
      _$previewHintCodeEnum_unknownDefaultOpenApi;

  static Serializer<PreviewHintCodeEnum> get serializer =>
      _$previewHintCodeEnumSerializer;

  const PreviewHintCodeEnum._(String name) : super(name);

  static BuiltSet<PreviewHintCodeEnum> get values =>
      _$previewHintCodeEnumValues;
  static PreviewHintCodeEnum valueOf(String name) =>
      _$previewHintCodeEnumValueOf(name);
}
