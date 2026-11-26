//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_computer_started.g.dart';

/// The host service started a computer session (`computer.started`, 27 12.3).
///
/// Properties:
/// * [sessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state] - State at the start; `paused` while the screen is locked.
@BuiltValue()
abstract class HostComputerStarted
    implements Built<HostComputerStarted, HostComputerStartedBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'session_id')
  String get sessionId;

  /// State at the start; `paused` while the screen is locked.
  @BuiltValueField(wireName: r'state')
  HostComputerStartedStateEnum get state;
  // enum stateEnum {  active,  paused,  };

  HostComputerStarted._();

  factory HostComputerStarted([void updates(HostComputerStartedBuilder b)]) =
      _$HostComputerStarted;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostComputerStartedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostComputerStarted> get serializer =>
      _$HostComputerStartedSerializer();
}

class _$HostComputerStartedSerializer
    implements PrimitiveSerializer<HostComputerStarted> {
  @override
  final Iterable<Type> types = const [
    HostComputerStarted,
    _$HostComputerStarted,
  ];

  @override
  final String wireName = r'HostComputerStarted';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostComputerStarted object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'session_id';
    yield serializers.serialize(
      object.sessionId,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(HostComputerStartedStateEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostComputerStarted object, {
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
    required HostComputerStartedBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sessionId = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostComputerStartedStateEnum),
          ) as HostComputerStartedStateEnum;
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
  HostComputerStarted deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostComputerStartedBuilder();
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

/// State at the start; `paused` while the screen is locked.
class HostComputerStartedStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'active')
  static const HostComputerStartedStateEnum active =
      _$hostComputerStartedStateEnum_active;
  @BuiltValueEnumConst(wireName: r'paused')
  static const HostComputerStartedStateEnum paused =
      _$hostComputerStartedStateEnum_paused;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostComputerStartedStateEnum unknownDefaultOpenApi =
      _$hostComputerStartedStateEnum_unknownDefaultOpenApi;

  static Serializer<HostComputerStartedStateEnum> get serializer =>
      _$hostComputerStartedStateEnumSerializer;

  const HostComputerStartedStateEnum._(String name) : super(name);

  static BuiltSet<HostComputerStartedStateEnum> get values =>
      _$hostComputerStartedStateEnumValues;
  static HostComputerStartedStateEnum valueOf(String name) =>
      _$hostComputerStartedStateEnumValueOf(name);
}
