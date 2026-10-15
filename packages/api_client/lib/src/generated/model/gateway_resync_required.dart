//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_resync_required.g.dart';

/// GatewayResyncRequired
///
/// Properties:
/// * [reason]
/// * [headSeq]
/// * [syncEpoch] - Time line of the event log; a client stores it with its cursor (04, 7.5).
@BuiltValue()
abstract class GatewayResyncRequired
    implements Built<GatewayResyncRequired, GatewayResyncRequiredBuilder> {
  @BuiltValueField(wireName: r'reason')
  GatewayResyncRequiredReasonEnum get reason;
  // enum reasonEnum {  epoch_changed,  retention,  too_far_behind,  cursor_ahead,  };

  @BuiltValueField(wireName: r'head_seq')
  int get headSeq;

  /// Time line of the event log; a client stores it with its cursor (04, 7.5).
  @BuiltValueField(wireName: r'sync_epoch')
  String get syncEpoch;

  GatewayResyncRequired._();

  factory GatewayResyncRequired([
    void updates(GatewayResyncRequiredBuilder b),
  ]) = _$GatewayResyncRequired;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayResyncRequiredBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayResyncRequired> get serializer =>
      _$GatewayResyncRequiredSerializer();
}

class _$GatewayResyncRequiredSerializer
    implements PrimitiveSerializer<GatewayResyncRequired> {
  @override
  final Iterable<Type> types = const [
    GatewayResyncRequired,
    _$GatewayResyncRequired,
  ];

  @override
  final String wireName = r'GatewayResyncRequired';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayResyncRequired object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(GatewayResyncRequiredReasonEnum),
    );
    yield r'head_seq';
    yield serializers.serialize(
      object.headSeq,
      specifiedType: const FullType(int),
    );
    yield r'sync_epoch';
    yield serializers.serialize(
      object.syncEpoch,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayResyncRequired object, {
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
    required GatewayResyncRequiredBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(GatewayResyncRequiredReasonEnum),
          ) as GatewayResyncRequiredReasonEnum;
          result.reason = valueDes;
          break;
        case r'head_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.headSeq = valueDes;
          break;
        case r'sync_epoch':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.syncEpoch = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayResyncRequired deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayResyncRequiredBuilder();
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

class GatewayResyncRequiredReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'epoch_changed')
  static const GatewayResyncRequiredReasonEnum epochChanged =
      _$gatewayResyncRequiredReasonEnum_epochChanged;
  @BuiltValueEnumConst(wireName: r'retention')
  static const GatewayResyncRequiredReasonEnum retention =
      _$gatewayResyncRequiredReasonEnum_retention;
  @BuiltValueEnumConst(wireName: r'too_far_behind')
  static const GatewayResyncRequiredReasonEnum tooFarBehind =
      _$gatewayResyncRequiredReasonEnum_tooFarBehind;
  @BuiltValueEnumConst(wireName: r'cursor_ahead')
  static const GatewayResyncRequiredReasonEnum cursorAhead =
      _$gatewayResyncRequiredReasonEnum_cursorAhead;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const GatewayResyncRequiredReasonEnum unknownDefaultOpenApi =
      _$gatewayResyncRequiredReasonEnum_unknownDefaultOpenApi;

  static Serializer<GatewayResyncRequiredReasonEnum> get serializer =>
      _$gatewayResyncRequiredReasonEnumSerializer;

  const GatewayResyncRequiredReasonEnum._(String name) : super(name);

  static BuiltSet<GatewayResyncRequiredReasonEnum> get values =>
      _$gatewayResyncRequiredReasonEnumValues;
  static GatewayResyncRequiredReasonEnum valueOf(String name) =>
      _$gatewayResyncRequiredReasonEnumValueOf(name);
}
