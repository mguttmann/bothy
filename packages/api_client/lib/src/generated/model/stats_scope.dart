//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_scope.g.dart';

/// StatsScope
///
/// Properties:
/// * [kind]
/// * [spaceId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class StatsScope implements Built<StatsScope, StatsScopeBuilder> {
  @BuiltValueField(wireName: r'kind')
  StatsScopeKindEnum get kind;
  // enum kindEnum {  instance,  space,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'space_id')
  String? get spaceId;

  StatsScope._();

  factory StatsScope([void updates(StatsScopeBuilder b)]) = _$StatsScope;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsScopeBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsScope> get serializer => _$StatsScopeSerializer();
}

class _$StatsScopeSerializer implements PrimitiveSerializer<StatsScope> {
  @override
  final Iterable<Type> types = const [StatsScope, _$StatsScope];

  @override
  final String wireName = r'StatsScope';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsScope object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(StatsScopeKindEnum),
    );
    if (object.spaceId != null) {
      yield r'space_id';
      yield serializers.serialize(
        object.spaceId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsScope object, {
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
    required StatsScopeBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsScopeKindEnum),
          ) as StatsScopeKindEnum;
          result.kind = valueDes;
          break;
        case r'space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.spaceId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsScope deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsScopeBuilder();
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

class StatsScopeKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'instance')
  static const StatsScopeKindEnum instance = _$statsScopeKindEnum_instance;
  @BuiltValueEnumConst(wireName: r'space')
  static const StatsScopeKindEnum space = _$statsScopeKindEnum_space;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatsScopeKindEnum unknownDefaultOpenApi =
      _$statsScopeKindEnum_unknownDefaultOpenApi;

  static Serializer<StatsScopeKindEnum> get serializer =>
      _$statsScopeKindEnumSerializer;

  const StatsScopeKindEnum._(String name) : super(name);

  static BuiltSet<StatsScopeKindEnum> get values => _$statsScopeKindEnumValues;
  static StatsScopeKindEnum valueOf(String name) =>
      _$statsScopeKindEnumValueOf(name);
}
