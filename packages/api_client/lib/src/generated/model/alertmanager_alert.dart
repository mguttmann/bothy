//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'alertmanager_alert.g.dart';

/// One alert of an Alertmanager webhook payload; `status` and `labels` are required (checked with `errors[].code` `required`).
///
/// Properties:
/// * [status]
/// * [labels] - Labels of the alert; every value is a string.
/// * [annotations] - Annotations of the alert; every value is a string.
/// * [startsAt]
/// * [endsAt]
/// * [generatorURL] - Address of the rule's source; written as text, never fetched.
/// * [fingerprint]
@BuiltValue()
abstract class AlertmanagerAlert
    implements Built<AlertmanagerAlert, AlertmanagerAlertBuilder> {
  @BuiltValueField(wireName: r'status')
  AlertmanagerAlertStatusEnum? get status;
  // enum statusEnum {  firing,  resolved,  };

  /// Labels of the alert; every value is a string.
  @BuiltValueField(wireName: r'labels')
  BuiltMap<String, JsonObject?>? get labels;

  /// Annotations of the alert; every value is a string.
  @BuiltValueField(wireName: r'annotations')
  BuiltMap<String, JsonObject?>? get annotations;

  @BuiltValueField(wireName: r'startsAt')
  DateTime? get startsAt;

  @BuiltValueField(wireName: r'endsAt')
  DateTime? get endsAt;

  /// Address of the rule's source; written as text, never fetched.
  @BuiltValueField(wireName: r'generatorURL')
  String? get generatorURL;

  @BuiltValueField(wireName: r'fingerprint')
  String? get fingerprint;

  AlertmanagerAlert._();

  factory AlertmanagerAlert([void updates(AlertmanagerAlertBuilder b)]) =
      _$AlertmanagerAlert;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AlertmanagerAlertBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AlertmanagerAlert> get serializer =>
      _$AlertmanagerAlertSerializer();
}

class _$AlertmanagerAlertSerializer
    implements PrimitiveSerializer<AlertmanagerAlert> {
  @override
  final Iterable<Type> types = const [AlertmanagerAlert, _$AlertmanagerAlert];

  @override
  final String wireName = r'AlertmanagerAlert';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AlertmanagerAlert object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.status != null) {
      yield r'status';
      yield serializers.serialize(
        object.status,
        specifiedType: const FullType(AlertmanagerAlertStatusEnum),
      );
    }
    if (object.labels != null) {
      yield r'labels';
      yield serializers.serialize(
        object.labels,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
    if (object.annotations != null) {
      yield r'annotations';
      yield serializers.serialize(
        object.annotations,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
    if (object.startsAt != null) {
      yield r'startsAt';
      yield serializers.serialize(
        object.startsAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.endsAt != null) {
      yield r'endsAt';
      yield serializers.serialize(
        object.endsAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.generatorURL != null) {
      yield r'generatorURL';
      yield serializers.serialize(
        object.generatorURL,
        specifiedType: const FullType(String),
      );
    }
    if (object.fingerprint != null) {
      yield r'fingerprint';
      yield serializers.serialize(
        object.fingerprint,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AlertmanagerAlert object, {
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
    required AlertmanagerAlertBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'status':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AlertmanagerAlertStatusEnum),
          ) as AlertmanagerAlertStatusEnum?;
          if (valueDes == null) continue;
          result.status = valueDes;
          break;
        case r'labels':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.labels.replace(valueDes);
          break;
        case r'annotations':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.annotations.replace(valueDes);
          break;
        case r'startsAt':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.startsAt = valueDes;
          break;
        case r'endsAt':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.endsAt = valueDes;
          break;
        case r'generatorURL':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.generatorURL = valueDes;
          break;
        case r'fingerprint':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.fingerprint = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AlertmanagerAlert deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AlertmanagerAlertBuilder();
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

class AlertmanagerAlertStatusEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'firing')
  static const AlertmanagerAlertStatusEnum firing =
      _$alertmanagerAlertStatusEnum_firing;
  @BuiltValueEnumConst(wireName: r'resolved')
  static const AlertmanagerAlertStatusEnum resolved =
      _$alertmanagerAlertStatusEnum_resolved;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AlertmanagerAlertStatusEnum unknownDefaultOpenApi =
      _$alertmanagerAlertStatusEnum_unknownDefaultOpenApi;

  static Serializer<AlertmanagerAlertStatusEnum> get serializer =>
      _$alertmanagerAlertStatusEnumSerializer;

  const AlertmanagerAlertStatusEnum._(String name) : super(name);

  static BuiltSet<AlertmanagerAlertStatusEnum> get values =>
      _$alertmanagerAlertStatusEnumValues;
  static AlertmanagerAlertStatusEnum valueOf(String name) =>
      _$alertmanagerAlertStatusEnumValueOf(name);
}
