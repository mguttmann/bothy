//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_computer_settings.g.dart';

/// The server values of a computer session (27 18), sent with `computer.open`.
///
/// Properties:
/// * [imageMaxEdge]
/// * [jpegQuality]
/// * [settleMs]
/// * [autoScreenshot]
/// * [waitMaxMs]
/// * [keyRepeatMax]
/// * [scrollAmountMax]
/// * [auditAppsMax]
/// * [statusIntervalMs]
@BuiltValue()
abstract class HostComputerSettings
    implements Built<HostComputerSettings, HostComputerSettingsBuilder> {
  @BuiltValueField(wireName: r'image_max_edge')
  int get imageMaxEdge;

  @BuiltValueField(wireName: r'jpeg_quality')
  int get jpegQuality;

  @BuiltValueField(wireName: r'settle_ms')
  int get settleMs;

  @BuiltValueField(wireName: r'auto_screenshot')
  bool get autoScreenshot;

  @BuiltValueField(wireName: r'wait_max_ms')
  int get waitMaxMs;

  @BuiltValueField(wireName: r'key_repeat_max')
  int get keyRepeatMax;

  @BuiltValueField(wireName: r'scroll_amount_max')
  int get scrollAmountMax;

  @BuiltValueField(wireName: r'audit_apps_max')
  int get auditAppsMax;

  @BuiltValueField(wireName: r'status_interval_ms')
  int get statusIntervalMs;

  HostComputerSettings._();

  factory HostComputerSettings([void updates(HostComputerSettingsBuilder b)]) =
      _$HostComputerSettings;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostComputerSettingsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostComputerSettings> get serializer =>
      _$HostComputerSettingsSerializer();
}

class _$HostComputerSettingsSerializer
    implements PrimitiveSerializer<HostComputerSettings> {
  @override
  final Iterable<Type> types = const [
    HostComputerSettings,
    _$HostComputerSettings,
  ];

  @override
  final String wireName = r'HostComputerSettings';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostComputerSettings object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'image_max_edge';
    yield serializers.serialize(
      object.imageMaxEdge,
      specifiedType: const FullType(int),
    );
    yield r'jpeg_quality';
    yield serializers.serialize(
      object.jpegQuality,
      specifiedType: const FullType(int),
    );
    yield r'settle_ms';
    yield serializers.serialize(
      object.settleMs,
      specifiedType: const FullType(int),
    );
    yield r'auto_screenshot';
    yield serializers.serialize(
      object.autoScreenshot,
      specifiedType: const FullType(bool),
    );
    yield r'wait_max_ms';
    yield serializers.serialize(
      object.waitMaxMs,
      specifiedType: const FullType(int),
    );
    yield r'key_repeat_max';
    yield serializers.serialize(
      object.keyRepeatMax,
      specifiedType: const FullType(int),
    );
    yield r'scroll_amount_max';
    yield serializers.serialize(
      object.scrollAmountMax,
      specifiedType: const FullType(int),
    );
    yield r'audit_apps_max';
    yield serializers.serialize(
      object.auditAppsMax,
      specifiedType: const FullType(int),
    );
    yield r'status_interval_ms';
    yield serializers.serialize(
      object.statusIntervalMs,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostComputerSettings object, {
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
    required HostComputerSettingsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'image_max_edge':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.imageMaxEdge = valueDes;
          break;
        case r'jpeg_quality':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.jpegQuality = valueDes;
          break;
        case r'settle_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.settleMs = valueDes;
          break;
        case r'auto_screenshot':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.autoScreenshot = valueDes;
          break;
        case r'wait_max_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.waitMaxMs = valueDes;
          break;
        case r'key_repeat_max':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.keyRepeatMax = valueDes;
          break;
        case r'scroll_amount_max':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.scrollAmountMax = valueDes;
          break;
        case r'audit_apps_max':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.auditAppsMax = valueDes;
          break;
        case r'status_interval_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.statusIntervalMs = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostComputerSettings deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostComputerSettingsBuilder();
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
