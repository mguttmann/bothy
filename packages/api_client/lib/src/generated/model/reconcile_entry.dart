//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'reconcile_entry.g.dart';

/// ReconcileEntry
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [amountMicros]
/// * [currency] - ISO 4217 code.
@BuiltValue()
abstract class ReconcileEntry
    implements Built<ReconcileEntry, ReconcileEntryBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  @BuiltValueField(wireName: r'amount_micros')
  int get amountMicros;

  /// ISO 4217 code.
  @BuiltValueField(wireName: r'currency')
  String get currency;

  ReconcileEntry._();

  factory ReconcileEntry([void updates(ReconcileEntryBuilder b)]) =
      _$ReconcileEntry;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ReconcileEntryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ReconcileEntry> get serializer =>
      _$ReconcileEntrySerializer();
}

class _$ReconcileEntrySerializer
    implements PrimitiveSerializer<ReconcileEntry> {
  @override
  final Iterable<Type> types = const [ReconcileEntry, _$ReconcileEntry];

  @override
  final String wireName = r'ReconcileEntry';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ReconcileEntry object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
    yield r'amount_micros';
    yield serializers.serialize(
      object.amountMicros,
      specifiedType: const FullType(int),
    );
    yield r'currency';
    yield serializers.serialize(
      object.currency,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ReconcileEntry object, {
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
    required ReconcileEntryBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.runId = valueDes;
          break;
        case r'amount_micros':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.amountMicros = valueDes;
          break;
        case r'currency':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.currency = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ReconcileEntry deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ReconcileEntryBuilder();
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
