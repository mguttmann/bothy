//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/command_option_value.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'autocomplete_query.g.dart';

/// AutocompleteQuery
///
/// Properties:
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [option]
/// * [value]
/// * [options] - Values of the other options entered so far.
@BuiltValue()
abstract class AutocompleteQuery
    implements Built<AutocompleteQuery, AutocompleteQueryBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String? get threadId;

  @BuiltValueField(wireName: r'option')
  String get option;

  @BuiltValueField(wireName: r'value')
  String get value;

  /// Values of the other options entered so far.
  @BuiltValueField(wireName: r'options')
  BuiltList<CommandOptionValue>? get options;

  AutocompleteQuery._();

  factory AutocompleteQuery([void updates(AutocompleteQueryBuilder b)]) =
      _$AutocompleteQuery;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AutocompleteQueryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AutocompleteQuery> get serializer =>
      _$AutocompleteQuerySerializer();
}

class _$AutocompleteQuerySerializer
    implements PrimitiveSerializer<AutocompleteQuery> {
  @override
  final Iterable<Type> types = const [AutocompleteQuery, _$AutocompleteQuery];

  @override
  final String wireName = r'AutocompleteQuery';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AutocompleteQuery object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.threadId != null) {
      yield r'thread_id';
      yield serializers.serialize(
        object.threadId,
        specifiedType: const FullType(String),
      );
    }
    yield r'option';
    yield serializers.serialize(
      object.option,
      specifiedType: const FullType(String),
    );
    yield r'value';
    yield serializers.serialize(
      object.value,
      specifiedType: const FullType(String),
    );
    if (object.options != null) {
      yield r'options';
      yield serializers.serialize(
        object.options,
        specifiedType: const FullType(BuiltList, [
          FullType(CommandOptionValue),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AutocompleteQuery object, {
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
    required AutocompleteQueryBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.threadId = valueDes;
          break;
        case r'option':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.option = valueDes;
          break;
        case r'value':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.value = valueDes;
          break;
        case r'options':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(CommandOptionValue),
            ]),
          ) as BuiltList<CommandOptionValue>?;
          if (valueDes == null) continue;
          result.options.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AutocompleteQuery deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AutocompleteQueryBuilder();
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
