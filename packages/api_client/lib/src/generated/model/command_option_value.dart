//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'command_option_value.g.dart';

/// CommandOptionValue
///
/// Properties:
/// * [name]
/// * [value]
@BuiltValue()
abstract class CommandOptionValue
    implements Built<CommandOptionValue, CommandOptionValueBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'value')
  String get value;

  CommandOptionValue._();

  factory CommandOptionValue([void updates(CommandOptionValueBuilder b)]) =
      _$CommandOptionValue;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CommandOptionValueBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CommandOptionValue> get serializer =>
      _$CommandOptionValueSerializer();
}

class _$CommandOptionValueSerializer
    implements PrimitiveSerializer<CommandOptionValue> {
  @override
  final Iterable<Type> types = const [CommandOptionValue, _$CommandOptionValue];

  @override
  final String wireName = r'CommandOptionValue';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CommandOptionValue object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'value';
    yield serializers.serialize(
      object.value,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CommandOptionValue object, {
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
    required CommandOptionValueBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'value':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.value = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CommandOptionValue deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CommandOptionValueBuilder();
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
