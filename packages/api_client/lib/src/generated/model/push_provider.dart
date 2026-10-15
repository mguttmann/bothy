//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'push_provider.g.dart';

class PushProvider extends EnumClass {
  @BuiltValueEnumConst(wireName: r'apns')
  static const PushProvider apns = _$apns;
  @BuiltValueEnumConst(wireName: r'fcm')
  static const PushProvider fcm = _$fcm;
  @BuiltValueEnumConst(wireName: r'unifiedpush')
  static const PushProvider unifiedpush = _$unifiedpush;
  @BuiltValueEnumConst(wireName: r'wns')
  static const PushProvider wns = _$wns;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const PushProvider unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<PushProvider> get serializer => _$pushProviderSerializer;

  const PushProvider._(String name) : super(name);

  static BuiltSet<PushProvider> get values => _$values;
  static PushProvider valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class PushProviderMixin = Object with _$PushProviderMixin;
