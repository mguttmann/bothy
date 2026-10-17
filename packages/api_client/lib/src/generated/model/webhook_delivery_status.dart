//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'webhook_delivery_status.g.dart';

class WebhookDeliveryStatus extends EnumClass {
  @BuiltValueEnumConst(wireName: r'pending')
  static const WebhookDeliveryStatus pending = _$pending;
  @BuiltValueEnumConst(wireName: r'succeeded')
  static const WebhookDeliveryStatus succeeded = _$succeeded;
  @BuiltValueEnumConst(wireName: r'failed')
  static const WebhookDeliveryStatus failed = _$failed;
  @BuiltValueEnumConst(wireName: r'dead')
  static const WebhookDeliveryStatus dead = _$dead;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const WebhookDeliveryStatus unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<WebhookDeliveryStatus> get serializer =>
      _$webhookDeliveryStatusSerializer;

  const WebhookDeliveryStatus._(String name) : super(name);

  static BuiltSet<WebhookDeliveryStatus> get values => _$values;
  static WebhookDeliveryStatus valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class WebhookDeliveryStatusMixin = Object
    with _$WebhookDeliveryStatusMixin;
