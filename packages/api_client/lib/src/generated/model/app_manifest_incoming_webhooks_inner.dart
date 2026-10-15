//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app_manifest_incoming_webhooks_inner.g.dart';

/// AppManifestIncomingWebhooksInner
///
/// Properties:
/// * [name]
/// * [purpose]
@BuiltValue()
abstract class AppManifestIncomingWebhooksInner
    implements
        Built<
          AppManifestIncomingWebhooksInner,
          AppManifestIncomingWebhooksInnerBuilder
        > {
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'purpose')
  String get purpose;

  AppManifestIncomingWebhooksInner._();

  factory AppManifestIncomingWebhooksInner([
    void updates(AppManifestIncomingWebhooksInnerBuilder b),
  ]) = _$AppManifestIncomingWebhooksInner;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AppManifestIncomingWebhooksInnerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AppManifestIncomingWebhooksInner> get serializer =>
      _$AppManifestIncomingWebhooksInnerSerializer();
}

class _$AppManifestIncomingWebhooksInnerSerializer
    implements PrimitiveSerializer<AppManifestIncomingWebhooksInner> {
  @override
  final Iterable<Type> types = const [
    AppManifestIncomingWebhooksInner,
    _$AppManifestIncomingWebhooksInner,
  ];

  @override
  final String wireName = r'AppManifestIncomingWebhooksInner';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AppManifestIncomingWebhooksInner object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'purpose';
    yield serializers.serialize(
      object.purpose,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AppManifestIncomingWebhooksInner object, {
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
    required AppManifestIncomingWebhooksInnerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'purpose':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.purpose = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AppManifestIncomingWebhooksInner deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AppManifestIncomingWebhooksInnerBuilder();
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
