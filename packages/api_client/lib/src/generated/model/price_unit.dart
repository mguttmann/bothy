//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'price_unit.g.dart';

class PriceUnit extends EnumClass {
  @BuiltValueEnumConst(wireName: r'input_mtok')
  static const PriceUnit inputMtok = _$inputMtok;
  @BuiltValueEnumConst(wireName: r'output_mtok')
  static const PriceUnit outputMtok = _$outputMtok;
  @BuiltValueEnumConst(wireName: r'cache_read_mtok')
  static const PriceUnit cacheReadMtok = _$cacheReadMtok;
  @BuiltValueEnumConst(wireName: r'cache_write_mtok')
  static const PriceUnit cacheWriteMtok = _$cacheWriteMtok;
  @BuiltValueEnumConst(wireName: r'cache_write_1h_mtok')
  static const PriceUnit cacheWrite1hMtok = _$cacheWrite1hMtok;
  @BuiltValueEnumConst(wireName: r'request')
  static const PriceUnit request = _$request;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const PriceUnit unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<PriceUnit> get serializer => _$priceUnitSerializer;

  const PriceUnit._(String name) : super(name);

  static BuiltSet<PriceUnit> get values => _$values;
  static PriceUnit valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class PriceUnitMixin = Object with _$PriceUnitMixin;
