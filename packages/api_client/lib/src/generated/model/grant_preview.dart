//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/invariant_violation.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/access_effect.dart';
import 'package:api_client/src/generated/model/preview_hint.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'grant_preview.g.dart';

/// Effect of a grant before it is created. No state changes (07, 9.3).
///
/// Properties:
/// * [allowed] - False if a delegation limit of the caller (`denial_reason`) or an invariant would reject the grant.
/// * [denialReason] - Reason code of 07 section 4.1 when a delegation limit rejects the grant, for example `delegation_exceeds_own_level`.
/// * [effects]
/// * [invariantViolations]
/// * [hints]
/// * [text] - Plain text from templates in `text_locale`, lines separated by a line feed, no trailing line feed.
/// * [textLocale]
@BuiltValue()
abstract class GrantPreview
    implements Built<GrantPreview, GrantPreviewBuilder> {
  /// False if a delegation limit of the caller (`denial_reason`) or an invariant would reject the grant.
  @BuiltValueField(wireName: r'allowed')
  bool get allowed;

  /// Reason code of 07 section 4.1 when a delegation limit rejects the grant, for example `delegation_exceeds_own_level`.
  @BuiltValueField(wireName: r'denial_reason')
  String? get denialReason;

  @BuiltValueField(wireName: r'effects')
  BuiltList<AccessEffect> get effects;

  @BuiltValueField(wireName: r'invariant_violations')
  BuiltList<InvariantViolation> get invariantViolations;

  @BuiltValueField(wireName: r'hints')
  BuiltList<PreviewHint> get hints;

  /// Plain text from templates in `text_locale`, lines separated by a line feed, no trailing line feed.
  @BuiltValueField(wireName: r'text')
  String get text;

  @BuiltValueField(wireName: r'text_locale')
  GrantPreviewTextLocaleEnum get textLocale;
  // enum textLocaleEnum {  de,  en,  };

  GrantPreview._();

  factory GrantPreview([void updates(GrantPreviewBuilder b)]) = _$GrantPreview;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GrantPreviewBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GrantPreview> get serializer => _$GrantPreviewSerializer();
}

class _$GrantPreviewSerializer implements PrimitiveSerializer<GrantPreview> {
  @override
  final Iterable<Type> types = const [GrantPreview, _$GrantPreview];

  @override
  final String wireName = r'GrantPreview';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GrantPreview object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'allowed';
    yield serializers.serialize(
      object.allowed,
      specifiedType: const FullType(bool),
    );
    if (object.denialReason != null) {
      yield r'denial_reason';
      yield serializers.serialize(
        object.denialReason,
        specifiedType: const FullType(String),
      );
    }
    yield r'effects';
    yield serializers.serialize(
      object.effects,
      specifiedType: const FullType(BuiltList, [FullType(AccessEffect)]),
    );
    yield r'invariant_violations';
    yield serializers.serialize(
      object.invariantViolations,
      specifiedType: const FullType(BuiltList, [FullType(InvariantViolation)]),
    );
    yield r'hints';
    yield serializers.serialize(
      object.hints,
      specifiedType: const FullType(BuiltList, [FullType(PreviewHint)]),
    );
    yield r'text';
    yield serializers.serialize(
      object.text,
      specifiedType: const FullType(String),
    );
    yield r'text_locale';
    yield serializers.serialize(
      object.textLocale,
      specifiedType: const FullType(GrantPreviewTextLocaleEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GrantPreview object, {
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
    required GrantPreviewBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'allowed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.allowed = valueDes;
          break;
        case r'denial_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.denialReason = valueDes;
          break;
        case r'effects':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AccessEffect)]),
          ) as BuiltList<AccessEffect>;
          result.effects.replace(valueDes);
          break;
        case r'invariant_violations':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(InvariantViolation),
            ]),
          ) as BuiltList<InvariantViolation>;
          result.invariantViolations.replace(valueDes);
          break;
        case r'hints':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(PreviewHint)]),
          ) as BuiltList<PreviewHint>;
          result.hints.replace(valueDes);
          break;
        case r'text':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.text = valueDes;
          break;
        case r'text_locale':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(GrantPreviewTextLocaleEnum),
          ) as GrantPreviewTextLocaleEnum;
          result.textLocale = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GrantPreview deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GrantPreviewBuilder();
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

class GrantPreviewTextLocaleEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'de')
  static const GrantPreviewTextLocaleEnum de = _$grantPreviewTextLocaleEnum_de;
  @BuiltValueEnumConst(wireName: r'en')
  static const GrantPreviewTextLocaleEnum en = _$grantPreviewTextLocaleEnum_en;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const GrantPreviewTextLocaleEnum unknownDefaultOpenApi =
      _$grantPreviewTextLocaleEnum_unknownDefaultOpenApi;

  static Serializer<GrantPreviewTextLocaleEnum> get serializer =>
      _$grantPreviewTextLocaleEnumSerializer;

  const GrantPreviewTextLocaleEnum._(String name) : super(name);

  static BuiltSet<GrantPreviewTextLocaleEnum> get values =>
      _$grantPreviewTextLocaleEnumValues;
  static GrantPreviewTextLocaleEnum valueOf(String name) =>
      _$grantPreviewTextLocaleEnumValueOf(name);
}
