// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_assist_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const SearchAssistResultModeEnum _$searchAssistResultModeEnum_assisted =
    const SearchAssistResultModeEnum._('assisted');
const SearchAssistResultModeEnum _$searchAssistResultModeEnum_fulltext =
    const SearchAssistResultModeEnum._('fulltext');
const SearchAssistResultModeEnum
_$searchAssistResultModeEnum_unknownDefaultOpenApi =
    const SearchAssistResultModeEnum._('unknownDefaultOpenApi');

SearchAssistResultModeEnum _$searchAssistResultModeEnumValueOf(String name) {
  switch (name) {
    case 'assisted':
      return _$searchAssistResultModeEnum_assisted;
    case 'fulltext':
      return _$searchAssistResultModeEnum_fulltext;
    case 'unknownDefaultOpenApi':
      return _$searchAssistResultModeEnum_unknownDefaultOpenApi;
    default:
      return _$searchAssistResultModeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<SearchAssistResultModeEnum> _$searchAssistResultModeEnumValues =
    BuiltSet<SearchAssistResultModeEnum>(const <SearchAssistResultModeEnum>[
      _$searchAssistResultModeEnum_assisted,
      _$searchAssistResultModeEnum_fulltext,
      _$searchAssistResultModeEnum_unknownDefaultOpenApi,
    ]);

const SearchAssistResultFallbackReasonEnum
_$searchAssistResultFallbackReasonEnum_assistantNotConfigured =
    const SearchAssistResultFallbackReasonEnum._('assistantNotConfigured');
const SearchAssistResultFallbackReasonEnum
_$searchAssistResultFallbackReasonEnum_assistantUnavailable =
    const SearchAssistResultFallbackReasonEnum._('assistantUnavailable');
const SearchAssistResultFallbackReasonEnum
_$searchAssistResultFallbackReasonEnum_assistantTimeout =
    const SearchAssistResultFallbackReasonEnum._('assistantTimeout');
const SearchAssistResultFallbackReasonEnum
_$searchAssistResultFallbackReasonEnum_unknownDefaultOpenApi =
    const SearchAssistResultFallbackReasonEnum._('unknownDefaultOpenApi');

SearchAssistResultFallbackReasonEnum
_$searchAssistResultFallbackReasonEnumValueOf(String name) {
  switch (name) {
    case 'assistantNotConfigured':
      return _$searchAssistResultFallbackReasonEnum_assistantNotConfigured;
    case 'assistantUnavailable':
      return _$searchAssistResultFallbackReasonEnum_assistantUnavailable;
    case 'assistantTimeout':
      return _$searchAssistResultFallbackReasonEnum_assistantTimeout;
    case 'unknownDefaultOpenApi':
      return _$searchAssistResultFallbackReasonEnum_unknownDefaultOpenApi;
    default:
      return _$searchAssistResultFallbackReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<SearchAssistResultFallbackReasonEnum>
_$searchAssistResultFallbackReasonEnumValues =
    BuiltSet<SearchAssistResultFallbackReasonEnum>(
      const <SearchAssistResultFallbackReasonEnum>[
        _$searchAssistResultFallbackReasonEnum_assistantNotConfigured,
        _$searchAssistResultFallbackReasonEnum_assistantUnavailable,
        _$searchAssistResultFallbackReasonEnum_assistantTimeout,
        _$searchAssistResultFallbackReasonEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<SearchAssistResultModeEnum> _$searchAssistResultModeEnumSerializer =
    _$SearchAssistResultModeEnumSerializer();
Serializer<SearchAssistResultFallbackReasonEnum>
_$searchAssistResultFallbackReasonEnumSerializer =
    _$SearchAssistResultFallbackReasonEnumSerializer();

class _$SearchAssistResultModeEnumSerializer
    implements PrimitiveSerializer<SearchAssistResultModeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'assisted': 'assisted',
    'fulltext': 'fulltext',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'assisted': 'assisted',
    'fulltext': 'fulltext',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[SearchAssistResultModeEnum];
  @override
  final String wireName = 'SearchAssistResultModeEnum';

  @override
  Object serialize(
    Serializers serializers,
    SearchAssistResultModeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  SearchAssistResultModeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => SearchAssistResultModeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$SearchAssistResultFallbackReasonEnumSerializer
    implements PrimitiveSerializer<SearchAssistResultFallbackReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'assistantNotConfigured': 'assistant_not_configured',
    'assistantUnavailable': 'assistant_unavailable',
    'assistantTimeout': 'assistant_timeout',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'assistant_not_configured': 'assistantNotConfigured',
    'assistant_unavailable': 'assistantUnavailable',
    'assistant_timeout': 'assistantTimeout',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    SearchAssistResultFallbackReasonEnum,
  ];
  @override
  final String wireName = 'SearchAssistResultFallbackReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    SearchAssistResultFallbackReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  SearchAssistResultFallbackReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => SearchAssistResultFallbackReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$SearchAssistResult extends SearchAssistResult {
  @override
  final BuiltList<SearchAssistHit> hits;
  @override
  final SearchAssistResultModeEnum mode;
  @override
  final SearchAssistResultFallbackReasonEnum? fallbackReason;
  @override
  final BuiltList<String> terms;
  @override
  final SearchAssistCost? costEstimate;

  factory _$SearchAssistResult([
    void Function(SearchAssistResultBuilder)? updates,
  ]) => (SearchAssistResultBuilder()..update(updates))._build();

  _$SearchAssistResult._({
    required this.hits,
    required this.mode,
    this.fallbackReason,
    required this.terms,
    this.costEstimate,
  }) : super._();
  @override
  SearchAssistResult rebuild(
    void Function(SearchAssistResultBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  SearchAssistResultBuilder toBuilder() =>
      SearchAssistResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchAssistResult &&
        hits == other.hits &&
        mode == other.mode &&
        fallbackReason == other.fallbackReason &&
        terms == other.terms &&
        costEstimate == other.costEstimate;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, hits.hashCode);
    _$hash = $jc(_$hash, mode.hashCode);
    _$hash = $jc(_$hash, fallbackReason.hashCode);
    _$hash = $jc(_$hash, terms.hashCode);
    _$hash = $jc(_$hash, costEstimate.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchAssistResult')
          ..add('hits', hits)
          ..add('mode', mode)
          ..add('fallbackReason', fallbackReason)
          ..add('terms', terms)
          ..add('costEstimate', costEstimate))
        .toString();
  }
}

class SearchAssistResultBuilder
    implements Builder<SearchAssistResult, SearchAssistResultBuilder> {
  _$SearchAssistResult? _$v;

  ListBuilder<SearchAssistHit>? _hits;
  ListBuilder<SearchAssistHit> get hits =>
      _$this._hits ??= ListBuilder<SearchAssistHit>();
  set hits(ListBuilder<SearchAssistHit>? hits) => _$this._hits = hits;

  SearchAssistResultModeEnum? _mode;
  SearchAssistResultModeEnum? get mode => _$this._mode;
  set mode(SearchAssistResultModeEnum? mode) => _$this._mode = mode;

  SearchAssistResultFallbackReasonEnum? _fallbackReason;
  SearchAssistResultFallbackReasonEnum? get fallbackReason =>
      _$this._fallbackReason;
  set fallbackReason(SearchAssistResultFallbackReasonEnum? fallbackReason) =>
      _$this._fallbackReason = fallbackReason;

  ListBuilder<String>? _terms;
  ListBuilder<String> get terms => _$this._terms ??= ListBuilder<String>();
  set terms(ListBuilder<String>? terms) => _$this._terms = terms;

  SearchAssistCostBuilder? _costEstimate;
  SearchAssistCostBuilder get costEstimate =>
      _$this._costEstimate ??= SearchAssistCostBuilder();
  set costEstimate(SearchAssistCostBuilder? costEstimate) =>
      _$this._costEstimate = costEstimate;

  SearchAssistResultBuilder() {
    SearchAssistResult._defaults(this);
  }

  SearchAssistResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _hits = $v.hits.toBuilder();
      _mode = $v.mode;
      _fallbackReason = $v.fallbackReason;
      _terms = $v.terms.toBuilder();
      _costEstimate = $v.costEstimate?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchAssistResult other) {
    _$v = other as _$SearchAssistResult;
  }

  @override
  void update(void Function(SearchAssistResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchAssistResult build() => _build();

  _$SearchAssistResult _build() {
    _$SearchAssistResult _$result;
    try {
      _$result =
          _$v ??
          _$SearchAssistResult._(
            hits: hits.build(),
            mode: BuiltValueNullFieldError.checkNotNull(
              mode,
              r'SearchAssistResult',
              'mode',
            ),
            fallbackReason: fallbackReason,
            terms: terms.build(),
            costEstimate: _costEstimate?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'hits';
        hits.build();

        _$failedField = 'terms';
        terms.build();
        _$failedField = 'costEstimate';
        _costEstimate?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SearchAssistResult',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
