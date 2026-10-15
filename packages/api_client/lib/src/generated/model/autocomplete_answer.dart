//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/command_choice.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'autocomplete_answer.g.dart';

/// AutocompleteAnswer
///
/// Properties:
/// * [choices]
@BuiltValue()
abstract class AutocompleteAnswer
    implements Built<AutocompleteAnswer, AutocompleteAnswerBuilder> {
  @BuiltValueField(wireName: r'choices')
  BuiltList<CommandChoice> get choices;

  AutocompleteAnswer._();

  factory AutocompleteAnswer([void updates(AutocompleteAnswerBuilder b)]) =
      _$AutocompleteAnswer;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AutocompleteAnswerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AutocompleteAnswer> get serializer =>
      _$AutocompleteAnswerSerializer();
}

class _$AutocompleteAnswerSerializer
    implements PrimitiveSerializer<AutocompleteAnswer> {
  @override
  final Iterable<Type> types = const [AutocompleteAnswer, _$AutocompleteAnswer];

  @override
  final String wireName = r'AutocompleteAnswer';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AutocompleteAnswer object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'choices';
    yield serializers.serialize(
      object.choices,
      specifiedType: const FullType(BuiltList, [FullType(CommandChoice)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AutocompleteAnswer object, {
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
    required AutocompleteAnswerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'choices':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(CommandChoice)]),
          ) as BuiltList<CommandChoice>;
          result.choices.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AutocompleteAnswer deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AutocompleteAnswerBuilder();
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
