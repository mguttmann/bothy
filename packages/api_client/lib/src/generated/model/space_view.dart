//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/space.dart';
import 'package:api_client/src/generated/model/capability.dart';
import 'package:api_client/src/generated/model/level.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'space_view.g.dart';

/// SpaceView
///
/// Properties:
/// * [space]
/// * [level] - Effective level on the space. A space visible only through one of its channels reports `read` and no capabilities.
/// * [capabilities]
@BuiltValue()
abstract class SpaceView implements Built<SpaceView, SpaceViewBuilder> {
  @BuiltValueField(wireName: r'space')
  Space get space;

  /// Effective level on the space. A space visible only through one of its channels reports `read` and no capabilities.
  @BuiltValueField(wireName: r'level')
  Level get level;
  // enum levelEnum {  read,  write,  trigger,  };

  @BuiltValueField(wireName: r'capabilities')
  BuiltList<Capability> get capabilities;

  SpaceView._();

  factory SpaceView([void updates(SpaceViewBuilder b)]) = _$SpaceView;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SpaceViewBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SpaceView> get serializer => _$SpaceViewSerializer();
}

class _$SpaceViewSerializer implements PrimitiveSerializer<SpaceView> {
  @override
  final Iterable<Type> types = const [SpaceView, _$SpaceView];

  @override
  final String wireName = r'SpaceView';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SpaceView object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'space';
    yield serializers.serialize(
      object.space,
      specifiedType: const FullType(Space),
    );
    yield r'level';
    yield serializers.serialize(
      object.level,
      specifiedType: const FullType(Level),
    );
    yield r'capabilities';
    yield serializers.serialize(
      object.capabilities,
      specifiedType: const FullType(BuiltList, [FullType(Capability)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SpaceView object, {
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
    required SpaceViewBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'space':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Space),
          ) as Space;
          result.space.replace(valueDes);
          break;
        case r'level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Level),
          ) as Level;
          result.level = valueDes;
          break;
        case r'capabilities':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Capability)]),
          ) as BuiltList<Capability>;
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
  SpaceView deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SpaceViewBuilder();
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
