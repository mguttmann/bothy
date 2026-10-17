//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_followup.g.dart';

/// RunFollowup
///
/// Properties:
/// * [messageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
@BuiltValue()
abstract class RunFollowup implements Built<RunFollowup, RunFollowupBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'message_id')
  String get messageId;

  @BuiltValueField(wireName: r'state')
  RunFollowupStateEnum get state;
  // enum stateEnum {  queued,  delivered,  absorbed,  deferred,  discarded,  };

  RunFollowup._();

  factory RunFollowup([void updates(RunFollowupBuilder b)]) = _$RunFollowup;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunFollowupBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunFollowup> get serializer => _$RunFollowupSerializer();
}

class _$RunFollowupSerializer implements PrimitiveSerializer<RunFollowup> {
  @override
  final Iterable<Type> types = const [RunFollowup, _$RunFollowup];

  @override
  final String wireName = r'RunFollowup';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunFollowup object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'message_id';
    yield serializers.serialize(
      object.messageId,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(RunFollowupStateEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    RunFollowup object, {
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
    required RunFollowupBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.messageId = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunFollowupStateEnum),
          ) as RunFollowupStateEnum;
          result.state = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunFollowup deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunFollowupBuilder();
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

class RunFollowupStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'queued')
  static const RunFollowupStateEnum queued = _$runFollowupStateEnum_queued;
  @BuiltValueEnumConst(wireName: r'delivered')
  static const RunFollowupStateEnum delivered =
      _$runFollowupStateEnum_delivered;
  @BuiltValueEnumConst(wireName: r'absorbed')
  static const RunFollowupStateEnum absorbed = _$runFollowupStateEnum_absorbed;
  @BuiltValueEnumConst(wireName: r'deferred')
  static const RunFollowupStateEnum deferred_ =
      _$runFollowupStateEnum_deferred_;
  @BuiltValueEnumConst(wireName: r'discarded')
  static const RunFollowupStateEnum discarded =
      _$runFollowupStateEnum_discarded;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunFollowupStateEnum unknownDefaultOpenApi =
      _$runFollowupStateEnum_unknownDefaultOpenApi;

  static Serializer<RunFollowupStateEnum> get serializer =>
      _$runFollowupStateEnumSerializer;

  const RunFollowupStateEnum._(String name) : super(name);

  static BuiltSet<RunFollowupStateEnum> get values =>
      _$runFollowupStateEnumValues;
  static RunFollowupStateEnum valueOf(String name) =>
      _$runFollowupStateEnumValueOf(name);
}
