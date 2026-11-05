// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_assist_hit.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const SearchAssistHitSource_Enum _$searchAssistHitSourceEnum_assistant =
    const SearchAssistHitSource_Enum._('assistant');
const SearchAssistHitSource_Enum _$searchAssistHitSourceEnum_fulltext =
    const SearchAssistHitSource_Enum._('fulltext');
const SearchAssistHitSource_Enum
_$searchAssistHitSourceEnum_unknownDefaultOpenApi =
    const SearchAssistHitSource_Enum._('unknownDefaultOpenApi');

SearchAssistHitSource_Enum _$searchAssistHitSourceEnumValueOf(String name) {
  switch (name) {
    case 'assistant':
      return _$searchAssistHitSourceEnum_assistant;
    case 'fulltext':
      return _$searchAssistHitSourceEnum_fulltext;
    case 'unknownDefaultOpenApi':
      return _$searchAssistHitSourceEnum_unknownDefaultOpenApi;
    default:
      return _$searchAssistHitSourceEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<SearchAssistHitSource_Enum> _$searchAssistHitSourceEnumValues =
    BuiltSet<SearchAssistHitSource_Enum>(const <SearchAssistHitSource_Enum>[
      _$searchAssistHitSourceEnum_assistant,
      _$searchAssistHitSourceEnum_fulltext,
      _$searchAssistHitSourceEnum_unknownDefaultOpenApi,
    ]);

Serializer<SearchAssistHitSource_Enum> _$searchAssistHitSourceEnumSerializer =
    _$SearchAssistHitSource_EnumSerializer();

class _$SearchAssistHitSource_EnumSerializer
    implements PrimitiveSerializer<SearchAssistHitSource_Enum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'assistant': 'assistant',
    'fulltext': 'fulltext',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'assistant': 'assistant',
    'fulltext': 'fulltext',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[SearchAssistHitSource_Enum];
  @override
  final String wireName = 'SearchAssistHitSource_Enum';

  @override
  Object serialize(
    Serializers serializers,
    SearchAssistHitSource_Enum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  SearchAssistHitSource_Enum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => SearchAssistHitSource_Enum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$SearchAssistHit extends SearchAssistHit {
  @override
  final String threadId;
  @override
  final String? messageId;
  @override
  final String channelId;
  @override
  final String? threadTitle;
  @override
  final String channelName;
  @override
  final String? spaceName;
  @override
  final DateTime at;
  @override
  final SearchAssistHitSource_Enum source_;
  @override
  final String? reason;
  @override
  final BuiltList<SearchHitSegmentsInner> segments;

  factory _$SearchAssistHit([void Function(SearchAssistHitBuilder)? updates]) =>
      (SearchAssistHitBuilder()..update(updates))._build();

  _$SearchAssistHit._({
    required this.threadId,
    this.messageId,
    required this.channelId,
    this.threadTitle,
    required this.channelName,
    this.spaceName,
    required this.at,
    required this.source_,
    this.reason,
    required this.segments,
  }) : super._();
  @override
  SearchAssistHit rebuild(void Function(SearchAssistHitBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SearchAssistHitBuilder toBuilder() => SearchAssistHitBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchAssistHit &&
        threadId == other.threadId &&
        messageId == other.messageId &&
        channelId == other.channelId &&
        threadTitle == other.threadTitle &&
        channelName == other.channelName &&
        spaceName == other.spaceName &&
        at == other.at &&
        source_ == other.source_ &&
        reason == other.reason &&
        segments == other.segments;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, threadTitle.hashCode);
    _$hash = $jc(_$hash, channelName.hashCode);
    _$hash = $jc(_$hash, spaceName.hashCode);
    _$hash = $jc(_$hash, at.hashCode);
    _$hash = $jc(_$hash, source_.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, segments.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchAssistHit')
          ..add('threadId', threadId)
          ..add('messageId', messageId)
          ..add('channelId', channelId)
          ..add('threadTitle', threadTitle)
          ..add('channelName', channelName)
          ..add('spaceName', spaceName)
          ..add('at', at)
          ..add('source_', source_)
          ..add('reason', reason)
          ..add('segments', segments))
        .toString();
  }
}

class SearchAssistHitBuilder
    implements Builder<SearchAssistHit, SearchAssistHitBuilder> {
  _$SearchAssistHit? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _threadTitle;
  String? get threadTitle => _$this._threadTitle;
  set threadTitle(String? threadTitle) => _$this._threadTitle = threadTitle;

  String? _channelName;
  String? get channelName => _$this._channelName;
  set channelName(String? channelName) => _$this._channelName = channelName;

  String? _spaceName;
  String? get spaceName => _$this._spaceName;
  set spaceName(String? spaceName) => _$this._spaceName = spaceName;

  DateTime? _at;
  DateTime? get at => _$this._at;
  set at(DateTime? at) => _$this._at = at;

  SearchAssistHitSource_Enum? _source_;
  SearchAssistHitSource_Enum? get source_ => _$this._source_;
  set source_(SearchAssistHitSource_Enum? source_) => _$this._source_ = source_;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ListBuilder<SearchHitSegmentsInner>? _segments;
  ListBuilder<SearchHitSegmentsInner> get segments =>
      _$this._segments ??= ListBuilder<SearchHitSegmentsInner>();
  set segments(ListBuilder<SearchHitSegmentsInner>? segments) =>
      _$this._segments = segments;

  SearchAssistHitBuilder() {
    SearchAssistHit._defaults(this);
  }

  SearchAssistHitBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _messageId = $v.messageId;
      _channelId = $v.channelId;
      _threadTitle = $v.threadTitle;
      _channelName = $v.channelName;
      _spaceName = $v.spaceName;
      _at = $v.at;
      _source_ = $v.source_;
      _reason = $v.reason;
      _segments = $v.segments.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchAssistHit other) {
    _$v = other as _$SearchAssistHit;
  }

  @override
  void update(void Function(SearchAssistHitBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchAssistHit build() => _build();

  _$SearchAssistHit _build() {
    _$SearchAssistHit _$result;
    try {
      _$result =
          _$v ??
          _$SearchAssistHit._(
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'SearchAssistHit',
              'threadId',
            ),
            messageId: messageId,
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'SearchAssistHit',
              'channelId',
            ),
            threadTitle: threadTitle,
            channelName: BuiltValueNullFieldError.checkNotNull(
              channelName,
              r'SearchAssistHit',
              'channelName',
            ),
            spaceName: spaceName,
            at: BuiltValueNullFieldError.checkNotNull(
              at,
              r'SearchAssistHit',
              'at',
            ),
            source_: BuiltValueNullFieldError.checkNotNull(
              source_,
              r'SearchAssistHit',
              'source_',
            ),
            reason: reason,
            segments: segments.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'segments';
        segments.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SearchAssistHit',
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
