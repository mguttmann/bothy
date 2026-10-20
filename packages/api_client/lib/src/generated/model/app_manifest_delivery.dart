//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app_manifest_delivery.g.dart';

/// Required with `events`, `commands` or `components`.
///
/// Properties:
/// * [mode]
/// * [eventsUrl] - Required for `webhook`, not allowed for `gateway`; `https`, host in `webhook.allowed_hosts`.
/// * [interactionsUrl] - Required for commands or components without the gateway; `https`, host in `webhook.allowed_hosts`.
@BuiltValue()
abstract class AppManifestDelivery
    implements Built<AppManifestDelivery, AppManifestDeliveryBuilder> {
  @BuiltValueField(wireName: r'mode')
  AppManifestDeliveryModeEnum get mode;
  // enum modeEnum {  gateway,  webhook,  };

  /// Required for `webhook`, not allowed for `gateway`; `https`, host in `webhook.allowed_hosts`.
  @BuiltValueField(wireName: r'events_url')
  String? get eventsUrl;

  /// Required for commands or components without the gateway; `https`, host in `webhook.allowed_hosts`.
  @BuiltValueField(wireName: r'interactions_url')
  String? get interactionsUrl;

  AppManifestDelivery._();

  factory AppManifestDelivery([void updates(AppManifestDeliveryBuilder b)]) =
      _$AppManifestDelivery;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AppManifestDeliveryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AppManifestDelivery> get serializer =>
      _$AppManifestDeliverySerializer();
}

class _$AppManifestDeliverySerializer
    implements PrimitiveSerializer<AppManifestDelivery> {
  @override
  final Iterable<Type> types = const [
    AppManifestDelivery,
    _$AppManifestDelivery,
  ];

  @override
  final String wireName = r'AppManifestDelivery';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AppManifestDelivery object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'mode';
    yield serializers.serialize(
      object.mode,
      specifiedType: const FullType(AppManifestDeliveryModeEnum),
    );
    if (object.eventsUrl != null) {
      yield r'events_url';
      yield serializers.serialize(
        object.eventsUrl,
        specifiedType: const FullType(String),
      );
    }
    if (object.interactionsUrl != null) {
      yield r'interactions_url';
      yield serializers.serialize(
        object.interactionsUrl,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AppManifestDelivery object, {
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
    required AppManifestDeliveryBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AppManifestDeliveryModeEnum),
          ) as AppManifestDeliveryModeEnum;
          result.mode = valueDes;
          break;
        case r'events_url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.eventsUrl = valueDes;
          break;
        case r'interactions_url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.interactionsUrl = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AppManifestDelivery deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AppManifestDeliveryBuilder();
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

class AppManifestDeliveryModeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'gateway')
  static const AppManifestDeliveryModeEnum gateway =
      _$appManifestDeliveryModeEnum_gateway;
  @BuiltValueEnumConst(wireName: r'webhook')
  static const AppManifestDeliveryModeEnum webhook =
      _$appManifestDeliveryModeEnum_webhook;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AppManifestDeliveryModeEnum unknownDefaultOpenApi =
      _$appManifestDeliveryModeEnum_unknownDefaultOpenApi;

  static Serializer<AppManifestDeliveryModeEnum> get serializer =>
      _$appManifestDeliveryModeEnumSerializer;

  const AppManifestDeliveryModeEnum._(String name) : super(name);

  static BuiltSet<AppManifestDeliveryModeEnum> get values =>
      _$appManifestDeliveryModeEnumValues;
  static AppManifestDeliveryModeEnum valueOf(String name) =>
      _$appManifestDeliveryModeEnumValueOf(name);
}
