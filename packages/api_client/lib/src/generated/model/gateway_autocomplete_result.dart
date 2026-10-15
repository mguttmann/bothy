//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/command_choice.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_autocomplete_result.g.dart';

/// Answer of the command owner to `command.autocomplete_requested` (action `command.autocomplete.respond`).
///
/// Properties:
/// * [requestId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [choices] - At most `command.autocomplete_max_choices`.
/// * [ref] - Client reference echoed in `error` frames for this op.
@BuiltValue()
abstract class GatewayAutocompleteResult
    implements
        Built<GatewayAutocompleteResult, GatewayAutocompleteResultBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'request_id')
  String get requestId;

  /// At most `command.autocomplete_max_choices`.
  @BuiltValueField(wireName: r'choices')
  BuiltList<CommandChoice> get choices;

  /// Client reference echoed in `error` frames for this op.
  @BuiltValueField(wireName: r'ref')
  String? get ref;

  GatewayAutocompleteResult._();

  factory GatewayAutocompleteResult([
    void updates(GatewayAutocompleteResultBuilder b),
  ]) = _$GatewayAutocompleteResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayAutocompleteResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayAutocompleteResult> get serializer =>
      _$GatewayAutocompleteResultSerializer();
}

class _$GatewayAutocompleteResultSerializer
    implements PrimitiveSerializer<GatewayAutocompleteResult> {
  @override
  final Iterable<Type> types = const [
    GatewayAutocompleteResult,
    _$GatewayAutocompleteResult,
  ];

  @override
  final String wireName = r'GatewayAutocompleteResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayAutocompleteResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'request_id';
    yield serializers.serialize(
      object.requestId,
      specifiedType: const FullType(String),
    );
    yield r'choices';
    yield serializers.serialize(
      object.choices,
      specifiedType: const FullType(BuiltList, [FullType(CommandChoice)]),
    );
    if (object.ref != null) {
      yield r'ref';
      yield serializers.serialize(
        object.ref,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayAutocompleteResult object, {
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
    required GatewayAutocompleteResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'request_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.requestId = valueDes;
          break;
        case r'choices':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(CommandChoice)]),
          ) as BuiltList<CommandChoice>;
          result.choices.replace(valueDes);
          break;
        case r'ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.ref = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayAutocompleteResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayAutocompleteResultBuilder();
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
