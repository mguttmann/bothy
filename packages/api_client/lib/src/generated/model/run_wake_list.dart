//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/run_wake.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_list.g.dart';

/// RunWakeList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class RunWakeList implements Built<RunWakeList, RunWakeListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<RunWake> get items;

  RunWakeList._();

  factory RunWakeList([void updates(RunWakeListBuilder b)]) = _$RunWakeList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunWakeListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunWakeList> get serializer => _$RunWakeListSerializer();
}

class _$RunWakeListSerializer implements PrimitiveSerializer<RunWakeList> {
  @override
  final Iterable<Type> types = const [RunWakeList, _$RunWakeList];

  @override
  final String wireName = r'RunWakeList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunWakeList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(RunWake)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    RunWakeList object, {
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
    required RunWakeListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(RunWake)]),
          ) as BuiltList<RunWake>;
          result.items.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunWakeList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunWakeListBuilder();
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
