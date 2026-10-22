//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/command_choice.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'autocomplete_result.g.dart';

/// AutocompleteResult
///
/// Properties:
/// * [choices] - At most `command.autocomplete_max_choices`.
@BuiltValue()
abstract class AutocompleteResult
    implements Built<AutocompleteResult, AutocompleteResultBuilder> {
  /// At most `command.autocomplete_max_choices`.
  @BuiltValueField(wireName: r'choices')
  BuiltList<CommandChoice> get choices;

  AutocompleteResult._();

  factory AutocompleteResult([void updates(AutocompleteResultBuilder b)]) =
      _$AutocompleteResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AutocompleteResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AutocompleteResult> get serializer =>
      _$AutocompleteResultSerializer();
}

class _$AutocompleteResultSerializer
    implements PrimitiveSerializer<AutocompleteResult> {
  @override
  final Iterable<Type> types = const [AutocompleteResult, _$AutocompleteResult];

  @override
  final String wireName = r'AutocompleteResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AutocompleteResult object, {
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
    AutocompleteResult object, {
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
    required AutocompleteResultBuilder result,
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
  AutocompleteResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AutocompleteResultBuilder();
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
