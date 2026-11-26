//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/computer_session.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'computer_session_list.g.dart';

/// ComputerSessionList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class ComputerSessionList
    implements Built<ComputerSessionList, ComputerSessionListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<ComputerSession> get items;

  ComputerSessionList._();

  factory ComputerSessionList([void updates(ComputerSessionListBuilder b)]) =
      _$ComputerSessionList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ComputerSessionListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ComputerSessionList> get serializer =>
      _$ComputerSessionListSerializer();
}

class _$ComputerSessionListSerializer
    implements PrimitiveSerializer<ComputerSessionList> {
  @override
  final Iterable<Type> types = const [
    ComputerSessionList,
    _$ComputerSessionList,
  ];

  @override
  final String wireName = r'ComputerSessionList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ComputerSessionList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(ComputerSession)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ComputerSessionList object, {
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
    required ComputerSessionListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(ComputerSession),
            ]),
          ) as BuiltList<ComputerSession>;
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
  ComputerSessionList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ComputerSessionListBuilder();
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
