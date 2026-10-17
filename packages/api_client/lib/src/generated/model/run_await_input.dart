//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_await_input.g.dart';

/// RunAwaitInput
///
/// Properties:
/// * [statusLine]
@BuiltValue()
abstract class RunAwaitInput
    implements Built<RunAwaitInput, RunAwaitInputBuilder> {
  @BuiltValueField(wireName: r'status_line')
  String get statusLine;

  RunAwaitInput._();

  factory RunAwaitInput([void updates(RunAwaitInputBuilder b)]) =
      _$RunAwaitInput;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunAwaitInputBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunAwaitInput> get serializer =>
      _$RunAwaitInputSerializer();
}

class _$RunAwaitInputSerializer implements PrimitiveSerializer<RunAwaitInput> {
  @override
  final Iterable<Type> types = const [RunAwaitInput, _$RunAwaitInput];

  @override
  final String wireName = r'RunAwaitInput';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunAwaitInput object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'status_line';
    yield serializers.serialize(
      object.statusLine,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    RunAwaitInput object, {
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
    required RunAwaitInputBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'status_line':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.statusLine = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunAwaitInput deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunAwaitInputBuilder();
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
