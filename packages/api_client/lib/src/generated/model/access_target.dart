//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/access_target_kind.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/capability.dart';
import 'package:api_client/src/generated/model/level.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'access_target.g.dart';

/// AccessTarget
///
/// Properties:
/// * [kind]
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [level]
/// * [previousLevel]
/// * [capabilities]
@BuiltValue()
abstract class AccessTarget
    implements Built<AccessTarget, AccessTargetBuilder> {
  @BuiltValueField(wireName: r'kind')
  AccessTargetKind get kind;
  // enum kindEnum {  space,  channel,  host,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'level')
  Level? get level;
  // enum levelEnum {  read,  write,  trigger,  };

  @BuiltValueField(wireName: r'previous_level')
  Level? get previousLevel;
  // enum previousLevelEnum {  read,  write,  trigger,  };

  @BuiltValueField(wireName: r'capabilities')
  BuiltList<Capability>? get capabilities;

  AccessTarget._();

  factory AccessTarget([void updates(AccessTargetBuilder b)]) = _$AccessTarget;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AccessTargetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AccessTarget> get serializer => _$AccessTargetSerializer();
}

class _$AccessTargetSerializer implements PrimitiveSerializer<AccessTarget> {
  @override
  final Iterable<Type> types = const [AccessTarget, _$AccessTarget];

  @override
  final String wireName = r'AccessTarget';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AccessTarget object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(AccessTargetKind),
    );
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    if (object.level != null) {
      yield r'level';
      yield serializers.serialize(
        object.level,
        specifiedType: const FullType(Level),
      );
    }
    if (object.previousLevel != null) {
      yield r'previous_level';
      yield serializers.serialize(
        object.previousLevel,
        specifiedType: const FullType(Level),
      );
    }
    if (object.capabilities != null) {
      yield r'capabilities';
      yield serializers.serialize(
        object.capabilities,
        specifiedType: const FullType(BuiltList, [FullType(Capability)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AccessTarget object, {
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
    required AccessTargetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AccessTargetKind),
          ) as AccessTargetKind;
          result.kind = valueDes;
          break;
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Level),
          ) as Level?;
          if (valueDes == null) continue;
          result.level = valueDes;
          break;
        case r'previous_level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Level),
          ) as Level?;
          if (valueDes == null) continue;
          result.previousLevel = valueDes;
          break;
        case r'capabilities':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(Capability),
            ]),
          ) as BuiltList<Capability>?;
          if (valueDes == null) continue;
          result.capabilities.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AccessTarget deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AccessTargetBuilder();
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
