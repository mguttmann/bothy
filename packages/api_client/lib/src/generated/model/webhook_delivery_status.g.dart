// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'webhook_delivery_status.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const WebhookDeliveryStatus _$pending = const WebhookDeliveryStatus._(
  'pending',
);
const WebhookDeliveryStatus _$succeeded = const WebhookDeliveryStatus._(
  'succeeded',
);
const WebhookDeliveryStatus _$failed = const WebhookDeliveryStatus._('failed');
const WebhookDeliveryStatus _$dead = const WebhookDeliveryStatus._('dead');
const WebhookDeliveryStatus _$unknownDefaultOpenApi =
    const WebhookDeliveryStatus._('unknownDefaultOpenApi');

WebhookDeliveryStatus _$valueOf(String name) {
  switch (name) {
    case 'pending':
      return _$pending;
    case 'succeeded':
      return _$succeeded;
    case 'failed':
      return _$failed;
    case 'dead':
      return _$dead;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<WebhookDeliveryStatus> _$values =
    BuiltSet<WebhookDeliveryStatus>(const <WebhookDeliveryStatus>[
      _$pending,
      _$succeeded,
      _$failed,
      _$dead,
      _$unknownDefaultOpenApi,
    ]);

class _$WebhookDeliveryStatusMeta {
  const _$WebhookDeliveryStatusMeta();
  WebhookDeliveryStatus get pending => _$pending;
  WebhookDeliveryStatus get succeeded => _$succeeded;
  WebhookDeliveryStatus get failed => _$failed;
  WebhookDeliveryStatus get dead => _$dead;
  WebhookDeliveryStatus get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  WebhookDeliveryStatus valueOf(String name) => _$valueOf(name);
  BuiltSet<WebhookDeliveryStatus> get values => _$values;
}

mixin _$WebhookDeliveryStatusMixin {
  // ignore: non_constant_identifier_names
  _$WebhookDeliveryStatusMeta get WebhookDeliveryStatus =>
      const _$WebhookDeliveryStatusMeta();
}

Serializer<WebhookDeliveryStatus> _$webhookDeliveryStatusSerializer =
    _$WebhookDeliveryStatusSerializer();

class _$WebhookDeliveryStatusSerializer
    implements PrimitiveSerializer<WebhookDeliveryStatus> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'pending': 'pending',
    'succeeded': 'succeeded',
    'failed': 'failed',
    'dead': 'dead',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'pending': 'pending',
    'succeeded': 'succeeded',
    'failed': 'failed',
    'dead': 'dead',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[WebhookDeliveryStatus];
  @override
  final String wireName = 'WebhookDeliveryStatus';

  @override
  Object serialize(
    Serializers serializers,
    WebhookDeliveryStatus object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  WebhookDeliveryStatus deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => WebhookDeliveryStatus.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
