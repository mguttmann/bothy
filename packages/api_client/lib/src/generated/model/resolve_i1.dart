//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'resolve_i1.g.dart';

/// Guided flow (05 rule 1.6, 10 section 11): switch these agents to `organization` first, in the same transaction as the change.
///
/// Properties:
/// * [switchToOrganization]
/// * [reason] - Reason of the switch, in the audit entry of every switched agent; at most `audit.reason_max_chars` characters, no control or bidi characters.
@BuiltValue()
abstract class ResolveI1 implements Built<ResolveI1, ResolveI1Builder> {
  @BuiltValueField(wireName: r'switch_to_organization')
  BuiltSet<String> get switchToOrganization;

  /// Reason of the switch, in the audit entry of every switched agent; at most `audit.reason_max_chars` characters, no control or bidi characters.
  @BuiltValueField(wireName: r'reason')
  String get reason;

  ResolveI1._();

  factory ResolveI1([void updates(ResolveI1Builder b)]) = _$ResolveI1;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ResolveI1Builder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ResolveI1> get serializer => _$ResolveI1Serializer();
}

class _$ResolveI1Serializer implements PrimitiveSerializer<ResolveI1> {
  @override
  final Iterable<Type> types = const [ResolveI1, _$ResolveI1];

  @override
  final String wireName = r'ResolveI1';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ResolveI1 object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'switch_to_organization';
    yield serializers.serialize(
      object.switchToOrganization,
      specifiedType: const FullType(BuiltSet, [FullType(String)]),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ResolveI1 object, {
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
    required ResolveI1Builder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'switch_to_organization':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltSet, [FullType(String)]),
          ) as BuiltSet<String>;
          result.switchToOrganization.replace(valueDes);
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ResolveI1 deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ResolveI1Builder();
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
