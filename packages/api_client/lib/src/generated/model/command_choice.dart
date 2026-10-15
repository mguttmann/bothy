//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'command_choice.g.dart';

/// CommandChoice
///
/// Properties:
/// * [name]
/// * [value] - Canonical string form of the value; the option type defines its syntax.
@BuiltValue()
abstract class CommandChoice
    implements Built<CommandChoice, CommandChoiceBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  /// Canonical string form of the value; the option type defines its syntax.
  @BuiltValueField(wireName: r'value')
  String get value;

  CommandChoice._();

  factory CommandChoice([void updates(CommandChoiceBuilder b)]) =
      _$CommandChoice;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CommandChoiceBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CommandChoice> get serializer =>
      _$CommandChoiceSerializer();
}

class _$CommandChoiceSerializer implements PrimitiveSerializer<CommandChoice> {
  @override
  final Iterable<Type> types = const [CommandChoice, _$CommandChoice];

  @override
  final String wireName = r'CommandChoice';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CommandChoice object, {
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
    CommandChoice object, {
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
    required CommandChoiceBuilder result,
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
  CommandChoice deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CommandChoiceBuilder();
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
