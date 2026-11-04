// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_assist_candidate.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SearchAssistCandidate extends SearchAssistCandidate {
  @override
  final String ref;
  @override
  final String? title;
  @override
  final String channelName;
  @override
  final String? spaceName;
  @override
  final DateTime at;
  @override
  final String excerpt;
  @override
  final bool shownBefore;

  factory _$SearchAssistCandidate([
    void Function(SearchAssistCandidateBuilder)? updates,
  ]) => (SearchAssistCandidateBuilder()..update(updates))._build();

  _$SearchAssistCandidate._({
    required this.ref,
    this.title,
    required this.channelName,
    this.spaceName,
    required this.at,
    required this.excerpt,
    required this.shownBefore,
  }) : super._();
  @override
  SearchAssistCandidate rebuild(
    void Function(SearchAssistCandidateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  SearchAssistCandidateBuilder toBuilder() =>
      SearchAssistCandidateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchAssistCandidate &&
        ref == other.ref &&
        title == other.title &&
        channelName == other.channelName &&
        spaceName == other.spaceName &&
        at == other.at &&
        excerpt == other.excerpt &&
        shownBefore == other.shownBefore;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, ref.hashCode);
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jc(_$hash, channelName.hashCode);
    _$hash = $jc(_$hash, spaceName.hashCode);
    _$hash = $jc(_$hash, at.hashCode);
    _$hash = $jc(_$hash, excerpt.hashCode);
    _$hash = $jc(_$hash, shownBefore.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchAssistCandidate')
          ..add('ref', ref)
          ..add('title', title)
          ..add('channelName', channelName)
          ..add('spaceName', spaceName)
          ..add('at', at)
          ..add('excerpt', excerpt)
          ..add('shownBefore', shownBefore))
        .toString();
  }
}

class SearchAssistCandidateBuilder
    implements Builder<SearchAssistCandidate, SearchAssistCandidateBuilder> {
  _$SearchAssistCandidate? _$v;

  String? _ref;
  String? get ref => _$this._ref;
  set ref(String? ref) => _$this._ref = ref;

  String? _title;
  String? get title => _$this._title;
  set title(String? title) => _$this._title = title;

  String? _channelName;
  String? get channelName => _$this._channelName;
  set channelName(String? channelName) => _$this._channelName = channelName;

  String? _spaceName;
  String? get spaceName => _$this._spaceName;
  set spaceName(String? spaceName) => _$this._spaceName = spaceName;

  DateTime? _at;
  DateTime? get at => _$this._at;
  set at(DateTime? at) => _$this._at = at;

  String? _excerpt;
  String? get excerpt => _$this._excerpt;
  set excerpt(String? excerpt) => _$this._excerpt = excerpt;

  bool? _shownBefore;
  bool? get shownBefore => _$this._shownBefore;
  set shownBefore(bool? shownBefore) => _$this._shownBefore = shownBefore;

  SearchAssistCandidateBuilder() {
    SearchAssistCandidate._defaults(this);
  }

  SearchAssistCandidateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _ref = $v.ref;
      _title = $v.title;
      _channelName = $v.channelName;
      _spaceName = $v.spaceName;
      _at = $v.at;
      _excerpt = $v.excerpt;
      _shownBefore = $v.shownBefore;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchAssistCandidate other) {
    _$v = other as _$SearchAssistCandidate;
  }

  @override
  void update(void Function(SearchAssistCandidateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchAssistCandidate build() => _build();

  _$SearchAssistCandidate _build() {
    final _$result =
        _$v ??
        _$SearchAssistCandidate._(
          ref: BuiltValueNullFieldError.checkNotNull(
            ref,
            r'SearchAssistCandidate',
            'ref',
          ),
          title: title,
          channelName: BuiltValueNullFieldError.checkNotNull(
            channelName,
            r'SearchAssistCandidate',
            'channelName',
          ),
          spaceName: spaceName,
          at: BuiltValueNullFieldError.checkNotNull(
            at,
            r'SearchAssistCandidate',
            'at',
          ),
          excerpt: BuiltValueNullFieldError.checkNotNull(
            excerpt,
            r'SearchAssistCandidate',
            'excerpt',
          ),
          shownBefore: BuiltValueNullFieldError.checkNotNull(
            shownBefore,
            r'SearchAssistCandidate',
            'shownBefore',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
