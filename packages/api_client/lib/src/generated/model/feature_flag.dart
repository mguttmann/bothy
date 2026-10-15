//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/flag_rollout.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'feature_flag.g.dart';

/// Registration from the code plus the stored value (ADR 0064). A key without a stored row has its default and `version` 0; a stored key without registration is `orphaned` and off.
///
/// Properties:
/// * [key]
/// * [kind] - Absent for orphaned keys.
/// * [description] - From the registration (English); absent for orphaned keys.
/// * [default_] - Registered default; absent for orphaned keys.
/// * [removeBy] - Server version by which a `release` flag is removed.
/// * [module] - Module that owns the flag.
/// * [orphaned]
/// * [enabled] - Global value; the stored value or the registered default.
/// * [rollout]
/// * [version]
/// * [updatedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [updatedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class FeatureFlag implements Built<FeatureFlag, FeatureFlagBuilder> {
  @BuiltValueField(wireName: r'key')
  String get key;

  /// Absent for orphaned keys.
  @BuiltValueField(wireName: r'kind')
  FeatureFlagKindEnum? get kind;
  // enum kindEnum {  release,  option,  };

  /// From the registration (English); absent for orphaned keys.
  @BuiltValueField(wireName: r'description')
  String? get description;

  /// Registered default; absent for orphaned keys.
  @BuiltValueField(wireName: r'default')
  bool? get default_;

  /// Server version by which a `release` flag is removed.
  @BuiltValueField(wireName: r'remove_by')
  String? get removeBy;

  /// Module that owns the flag.
  @BuiltValueField(wireName: r'module')
  String? get module;

  @BuiltValueField(wireName: r'orphaned')
  bool get orphaned;

  /// Global value; the stored value or the registered default.
  @BuiltValueField(wireName: r'enabled')
  bool get enabled;

  @BuiltValueField(wireName: r'rollout')
  FlagRollout get rollout;

  @BuiltValueField(wireName: r'version')
  int get version;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'updated_by')
  String? get updatedBy;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'updated_at')
  DateTime? get updatedAt;

  FeatureFlag._();

  factory FeatureFlag([void updates(FeatureFlagBuilder b)]) = _$FeatureFlag;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(FeatureFlagBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<FeatureFlag> get serializer => _$FeatureFlagSerializer();
}

class _$FeatureFlagSerializer implements PrimitiveSerializer<FeatureFlag> {
  @override
  final Iterable<Type> types = const [FeatureFlag, _$FeatureFlag];

  @override
  final String wireName = r'FeatureFlag';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    FeatureFlag object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'key';
    yield serializers.serialize(
      object.key,
      specifiedType: const FullType(String),
    );
    if (object.kind != null) {
      yield r'kind';
      yield serializers.serialize(
        object.kind,
        specifiedType: const FullType(FeatureFlagKindEnum),
      );
    }
    if (object.description != null) {
      yield r'description';
      yield serializers.serialize(
        object.description,
        specifiedType: const FullType(String),
      );
    }
    if (object.default_ != null) {
      yield r'default';
      yield serializers.serialize(
        object.default_,
        specifiedType: const FullType(bool),
      );
    }
    if (object.removeBy != null) {
      yield r'remove_by';
      yield serializers.serialize(
        object.removeBy,
        specifiedType: const FullType(String),
      );
    }
    if (object.module != null) {
      yield r'module';
      yield serializers.serialize(
        object.module,
        specifiedType: const FullType(String),
      );
    }
    yield r'orphaned';
    yield serializers.serialize(
      object.orphaned,
      specifiedType: const FullType(bool),
    );
    yield r'enabled';
    yield serializers.serialize(
      object.enabled,
      specifiedType: const FullType(bool),
    );
    yield r'rollout';
    yield serializers.serialize(
      object.rollout,
      specifiedType: const FullType(FlagRollout),
    );
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
    if (object.updatedBy != null) {
      yield r'updated_by';
      yield serializers.serialize(
        object.updatedBy,
        specifiedType: const FullType(String),
      );
    }
    if (object.updatedAt != null) {
      yield r'updated_at';
      yield serializers.serialize(
        object.updatedAt,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    FeatureFlag object, {
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
    required FeatureFlagBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.key = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(FeatureFlagKindEnum),
          ) as FeatureFlagKindEnum?;
          if (valueDes == null) continue;
          result.kind = valueDes;
          break;
        case r'description':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.description = valueDes;
          break;
        case r'default':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.default_ = valueDes;
          break;
        case r'remove_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.removeBy = valueDes;
          break;
        case r'module':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.module = valueDes;
          break;
        case r'orphaned':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.orphaned = valueDes;
          break;
        case r'enabled':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.enabled = valueDes;
          break;
        case r'rollout':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(FlagRollout),
          ) as FlagRollout;
          result.rollout.replace(valueDes);
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        case r'updated_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.updatedBy = valueDes;
          break;
        case r'updated_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.updatedAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  FeatureFlag deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = FeatureFlagBuilder();
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

/// Absent for orphaned keys.
class FeatureFlagKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'release')
  static const FeatureFlagKindEnum release = _$featureFlagKindEnum_release;
  @BuiltValueEnumConst(wireName: r'option')
  static const FeatureFlagKindEnum option = _$featureFlagKindEnum_option;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const FeatureFlagKindEnum unknownDefaultOpenApi =
      _$featureFlagKindEnum_unknownDefaultOpenApi;

  static Serializer<FeatureFlagKindEnum> get serializer =>
      _$featureFlagKindEnumSerializer;

  const FeatureFlagKindEnum._(String name) : super(name);

  static BuiltSet<FeatureFlagKindEnum> get values =>
      _$featureFlagKindEnumValues;
  static FeatureFlagKindEnum valueOf(String name) =>
      _$featureFlagKindEnumValueOf(name);
}
