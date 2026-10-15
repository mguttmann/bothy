//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/search_hit.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_hit_page.g.dart';

/// One page. `next_cursor` is absent on the last page.
///
/// Properties:
/// * [items]
/// * [nextCursor] - Opaque pagination cursor. Clients must not parse or build cursors.
/// * [candidatesCapped] - The candidate limit `search.max_candidates` was reached; ask for a narrower query.
@BuiltValue()
abstract class SearchHitPage
    implements Built<SearchHitPage, SearchHitPageBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<SearchHit> get items;

  /// Opaque pagination cursor. Clients must not parse or build cursors.
  @BuiltValueField(wireName: r'next_cursor')
  String? get nextCursor;

  /// The candidate limit `search.max_candidates` was reached; ask for a narrower query.
  @BuiltValueField(wireName: r'candidates_capped')
  bool? get candidatesCapped;

  SearchHitPage._();

  factory SearchHitPage([void updates(SearchHitPageBuilder b)]) =
      _$SearchHitPage;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchHitPageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchHitPage> get serializer =>
      _$SearchHitPageSerializer();
}

class _$SearchHitPageSerializer implements PrimitiveSerializer<SearchHitPage> {
  @override
  final Iterable<Type> types = const [SearchHitPage, _$SearchHitPage];

  @override
  final String wireName = r'SearchHitPage';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchHitPage object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(SearchHit)]),
    );
    if (object.nextCursor != null) {
      yield r'next_cursor';
      yield serializers.serialize(
        object.nextCursor,
        specifiedType: const FullType(String),
      );
    }
    if (object.candidatesCapped != null) {
      yield r'candidates_capped';
      yield serializers.serialize(
        object.candidatesCapped,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    SearchHitPage object, {
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
    required SearchHitPageBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(SearchHit)]),
          ) as BuiltList<SearchHit>;
          result.items.replace(valueDes);
          break;
        case r'next_cursor':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.nextCursor = valueDes;
          break;
        case r'candidates_capped':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.candidatesCapped = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SearchHitPage deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchHitPageBuilder();
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
