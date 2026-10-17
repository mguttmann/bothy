//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/target_kind.dart';
import 'package:api_client/src/generated/model/capability.dart';
import 'package:api_client/src/generated/model/level_or_none.dart';
import 'package:api_client/src/generated/model/principal_ref.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'access_effect.g.dart';

/// AccessEffect
///
/// Properties:
/// * [principal]
/// * [targetKind]
/// * [targetId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [beforeLevel]
/// * [afterLevel]
/// * [capabilitiesAdded]
@BuiltValue()
abstract class AccessEffect
    implements Built<AccessEffect, AccessEffectBuilder> {
  @BuiltValueField(wireName: r'principal')
  PrincipalRef get principal;

  @BuiltValueField(wireName: r'target_kind')
  TargetKind get targetKind;
  // enum targetKindEnum {  instance,  space,  channel,  host,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'target_id')
  String? get targetId;

  @BuiltValueField(wireName: r'before_level')
  LevelOrNone get beforeLevel;
  // enum beforeLevelEnum {  none,  read,  write,  trigger,  };

  @BuiltValueField(wireName: r'after_level')
  LevelOrNone get afterLevel;
  // enum afterLevelEnum {  none,  read,  write,  trigger,  };

  @BuiltValueField(wireName: r'capabilities_added')
  BuiltList<Capability> get capabilitiesAdded;

  AccessEffect._();

  factory AccessEffect([void updates(AccessEffectBuilder b)]) = _$AccessEffect;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AccessEffectBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AccessEffect> get serializer => _$AccessEffectSerializer();
}

class _$AccessEffectSerializer implements PrimitiveSerializer<AccessEffect> {
  @override
  final Iterable<Type> types = const [AccessEffect, _$AccessEffect];

  @override
  final String wireName = r'AccessEffect';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AccessEffect object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'principal';
    yield serializers.serialize(
      object.principal,
      specifiedType: const FullType(PrincipalRef),
    );
    yield r'target_kind';
    yield serializers.serialize(
      object.targetKind,
      specifiedType: const FullType(TargetKind),
    );
    if (object.targetId != null) {
      yield r'target_id';
      yield serializers.serialize(
        object.targetId,
        specifiedType: const FullType(String),
      );
    }
    yield r'before_level';
    yield serializers.serialize(
      object.beforeLevel,
      specifiedType: const FullType(LevelOrNone),
    );
    yield r'after_level';
    yield serializers.serialize(
      object.afterLevel,
      specifiedType: const FullType(LevelOrNone),
    );
    yield r'capabilities_added';
    yield serializers.serialize(
      object.capabilitiesAdded,
      specifiedType: const FullType(BuiltList, [FullType(Capability)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AccessEffect object, {
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
    required AccessEffectBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'principal':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PrincipalRef),
          ) as PrincipalRef;
          result.principal.replace(valueDes);
          break;
        case r'target_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TargetKind),
          ) as TargetKind;
          result.targetKind = valueDes;
          break;
        case r'target_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.targetId = valueDes;
          break;
        case r'before_level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(LevelOrNone),
          ) as LevelOrNone;
          result.beforeLevel = valueDes;
          break;
        case r'after_level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(LevelOrNone),
          ) as LevelOrNone;
          result.afterLevel = valueDes;
          break;
        case r'capabilities_added':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Capability)]),
          ) as BuiltList<Capability>;
          result.capabilitiesAdded.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AccessEffect deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AccessEffectBuilder();
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
