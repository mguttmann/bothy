//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/access_target.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'access_changed_payload.g.dart';

/// AccessChangedPayload
///
/// Properties:
/// * [permissionVersion]
/// * [added]
/// * [removed]
/// * [changed]
@BuiltValue()
abstract class AccessChangedPayload
    implements Built<AccessChangedPayload, AccessChangedPayloadBuilder> {
  @BuiltValueField(wireName: r'permission_version')
  int get permissionVersion;

  @BuiltValueField(wireName: r'added')
  BuiltList<AccessTarget> get added;

  @BuiltValueField(wireName: r'removed')
  BuiltList<AccessTarget> get removed;

  @BuiltValueField(wireName: r'changed')
  BuiltList<AccessTarget> get changed;

  AccessChangedPayload._();

  factory AccessChangedPayload([void updates(AccessChangedPayloadBuilder b)]) =
      _$AccessChangedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AccessChangedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AccessChangedPayload> get serializer =>
      _$AccessChangedPayloadSerializer();
}

class _$AccessChangedPayloadSerializer
    implements PrimitiveSerializer<AccessChangedPayload> {
  @override
  final Iterable<Type> types = const [
    AccessChangedPayload,
    _$AccessChangedPayload,
  ];

  @override
  final String wireName = r'AccessChangedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AccessChangedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'permission_version';
    yield serializers.serialize(
      object.permissionVersion,
      specifiedType: const FullType(int),
    );
    yield r'added';
    yield serializers.serialize(
      object.added,
      specifiedType: const FullType(BuiltList, [FullType(AccessTarget)]),
    );
    yield r'removed';
    yield serializers.serialize(
      object.removed,
      specifiedType: const FullType(BuiltList, [FullType(AccessTarget)]),
    );
    yield r'changed';
    yield serializers.serialize(
      object.changed,
      specifiedType: const FullType(BuiltList, [FullType(AccessTarget)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AccessChangedPayload object, {
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
    required AccessChangedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'permission_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.permissionVersion = valueDes;
          break;
        case r'added':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AccessTarget)]),
          ) as BuiltList<AccessTarget>;
          result.added.replace(valueDes);
          break;
        case r'removed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AccessTarget)]),
          ) as BuiltList<AccessTarget>;
          result.removed.replace(valueDes);
          break;
        case r'changed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AccessTarget)]),
          ) as BuiltList<AccessTarget>;
          result.changed.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AccessChangedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AccessChangedPayloadBuilder();
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
