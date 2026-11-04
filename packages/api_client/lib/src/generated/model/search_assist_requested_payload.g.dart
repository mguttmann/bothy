// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_assist_requested_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SearchAssistRequestedPayload extends SearchAssistRequestedPayload {
  @override
  final String requestId;
  @override
  final String q;
  @override
  final BuiltList<String>? history;
  @override
  final BuiltList<SearchAssistCandidate> candidates;
  @override
  final int maxPicks;
  @override
  final int maxTerms;
  @override
  final DateTime expiresAt;

  factory _$SearchAssistRequestedPayload([
    void Function(SearchAssistRequestedPayloadBuilder)? updates,
  ]) => (SearchAssistRequestedPayloadBuilder()..update(updates))._build();

  _$SearchAssistRequestedPayload._({
    required this.requestId,
    required this.q,
    this.history,
    required this.candidates,
    required this.maxPicks,
    required this.maxTerms,
    required this.expiresAt,
  }) : super._();
  @override
  SearchAssistRequestedPayload rebuild(
    void Function(SearchAssistRequestedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  SearchAssistRequestedPayloadBuilder toBuilder() =>
      SearchAssistRequestedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchAssistRequestedPayload &&
        requestId == other.requestId &&
        q == other.q &&
        history == other.history &&
        candidates == other.candidates &&
        maxPicks == other.maxPicks &&
        maxTerms == other.maxTerms &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, requestId.hashCode);
    _$hash = $jc(_$hash, q.hashCode);
    _$hash = $jc(_$hash, history.hashCode);
    _$hash = $jc(_$hash, candidates.hashCode);
    _$hash = $jc(_$hash, maxPicks.hashCode);
    _$hash = $jc(_$hash, maxTerms.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchAssistRequestedPayload')
          ..add('requestId', requestId)
          ..add('q', q)
          ..add('history', history)
          ..add('candidates', candidates)
          ..add('maxPicks', maxPicks)
          ..add('maxTerms', maxTerms)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class SearchAssistRequestedPayloadBuilder
    implements
        Builder<
          SearchAssistRequestedPayload,
          SearchAssistRequestedPayloadBuilder
        > {
  _$SearchAssistRequestedPayload? _$v;

  String? _requestId;
  String? get requestId => _$this._requestId;
  set requestId(String? requestId) => _$this._requestId = requestId;

  String? _q;
  String? get q => _$this._q;
  set q(String? q) => _$this._q = q;

  ListBuilder<String>? _history;
  ListBuilder<String> get history => _$this._history ??= ListBuilder<String>();
  set history(ListBuilder<String>? history) => _$this._history = history;

  ListBuilder<SearchAssistCandidate>? _candidates;
  ListBuilder<SearchAssistCandidate> get candidates =>
      _$this._candidates ??= ListBuilder<SearchAssistCandidate>();
  set candidates(ListBuilder<SearchAssistCandidate>? candidates) =>
      _$this._candidates = candidates;

  int? _maxPicks;
  int? get maxPicks => _$this._maxPicks;
  set maxPicks(int? maxPicks) => _$this._maxPicks = maxPicks;

  int? _maxTerms;
  int? get maxTerms => _$this._maxTerms;
  set maxTerms(int? maxTerms) => _$this._maxTerms = maxTerms;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  SearchAssistRequestedPayloadBuilder() {
    SearchAssistRequestedPayload._defaults(this);
  }

  SearchAssistRequestedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _requestId = $v.requestId;
      _q = $v.q;
      _history = $v.history?.toBuilder();
      _candidates = $v.candidates.toBuilder();
      _maxPicks = $v.maxPicks;
      _maxTerms = $v.maxTerms;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchAssistRequestedPayload other) {
    _$v = other as _$SearchAssistRequestedPayload;
  }

  @override
  void update(void Function(SearchAssistRequestedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchAssistRequestedPayload build() => _build();

  _$SearchAssistRequestedPayload _build() {
    _$SearchAssistRequestedPayload _$result;
    try {
      _$result =
          _$v ??
          _$SearchAssistRequestedPayload._(
            requestId: BuiltValueNullFieldError.checkNotNull(
              requestId,
              r'SearchAssistRequestedPayload',
              'requestId',
            ),
            q: BuiltValueNullFieldError.checkNotNull(
              q,
              r'SearchAssistRequestedPayload',
              'q',
            ),
            history: _history?.build(),
            candidates: candidates.build(),
            maxPicks: BuiltValueNullFieldError.checkNotNull(
              maxPicks,
              r'SearchAssistRequestedPayload',
              'maxPicks',
            ),
            maxTerms: BuiltValueNullFieldError.checkNotNull(
              maxTerms,
              r'SearchAssistRequestedPayload',
              'maxTerms',
            ),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'SearchAssistRequestedPayload',
              'expiresAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'history';
        _history?.build();
        _$failedField = 'candidates';
        candidates.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SearchAssistRequestedPayload',
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
