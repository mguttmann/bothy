//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/command.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'command_list.g.dart';

/// CommandList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class CommandList implements Built<CommandList, CommandListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<Command> get items;

  CommandList._();

  factory CommandList([void updates(CommandListBuilder b)]) = _$CommandList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CommandListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CommandList> get serializer => _$CommandListSerializer();
}

class _$CommandListSerializer implements PrimitiveSerializer<CommandList> {
  @override
  final Iterable<Type> types = const [CommandList, _$CommandList];

  @override
  final String wireName = r'CommandList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CommandList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(Command)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CommandList object, {
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
    required CommandListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Command)]),
          ) as BuiltList<Command>;
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
  CommandList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CommandListBuilder();
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
