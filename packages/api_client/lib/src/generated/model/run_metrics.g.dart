// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_metrics.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunMetrics extends RunMetrics {
  @override
  final double? contextPct;
  @override
  final int? tokensIn;
  @override
  final int? tokensOut;
  @override
  final int? tokensCacheRead;
  @override
  final int? tokensCacheWrite;
  @override
  final int? tokensCacheWrite1h;
  @override
  final int? subagents;
  @override
  final int? backgroundJobs;
  @override
  final int? apiRetries;
  @override
  final RunApiRetry? lastApiRetry;

  factory _$RunMetrics([void Function(RunMetricsBuilder)? updates]) =>
      (RunMetricsBuilder()..update(updates))._build();

  _$RunMetrics._({
    this.contextPct,
    this.tokensIn,
    this.tokensOut,
    this.tokensCacheRead,
    this.tokensCacheWrite,
    this.tokensCacheWrite1h,
    this.subagents,
    this.backgroundJobs,
    this.apiRetries,
    this.lastApiRetry,
  }) : super._();
  @override
  RunMetrics rebuild(void Function(RunMetricsBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunMetricsBuilder toBuilder() => RunMetricsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunMetrics &&
        contextPct == other.contextPct &&
        tokensIn == other.tokensIn &&
        tokensOut == other.tokensOut &&
        tokensCacheRead == other.tokensCacheRead &&
        tokensCacheWrite == other.tokensCacheWrite &&
        tokensCacheWrite1h == other.tokensCacheWrite1h &&
        subagents == other.subagents &&
        backgroundJobs == other.backgroundJobs &&
        apiRetries == other.apiRetries &&
        lastApiRetry == other.lastApiRetry;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, contextPct.hashCode);
    _$hash = $jc(_$hash, tokensIn.hashCode);
    _$hash = $jc(_$hash, tokensOut.hashCode);
    _$hash = $jc(_$hash, tokensCacheRead.hashCode);
    _$hash = $jc(_$hash, tokensCacheWrite.hashCode);
    _$hash = $jc(_$hash, tokensCacheWrite1h.hashCode);
    _$hash = $jc(_$hash, subagents.hashCode);
    _$hash = $jc(_$hash, backgroundJobs.hashCode);
    _$hash = $jc(_$hash, apiRetries.hashCode);
    _$hash = $jc(_$hash, lastApiRetry.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunMetrics')
          ..add('contextPct', contextPct)
          ..add('tokensIn', tokensIn)
          ..add('tokensOut', tokensOut)
          ..add('tokensCacheRead', tokensCacheRead)
          ..add('tokensCacheWrite', tokensCacheWrite)
          ..add('tokensCacheWrite1h', tokensCacheWrite1h)
          ..add('subagents', subagents)
          ..add('backgroundJobs', backgroundJobs)
          ..add('apiRetries', apiRetries)
          ..add('lastApiRetry', lastApiRetry))
        .toString();
  }
}

class RunMetricsBuilder implements Builder<RunMetrics, RunMetricsBuilder> {
  _$RunMetrics? _$v;

  double? _contextPct;
  double? get contextPct => _$this._contextPct;
  set contextPct(double? contextPct) => _$this._contextPct = contextPct;

  int? _tokensIn;
  int? get tokensIn => _$this._tokensIn;
  set tokensIn(int? tokensIn) => _$this._tokensIn = tokensIn;

  int? _tokensOut;
  int? get tokensOut => _$this._tokensOut;
  set tokensOut(int? tokensOut) => _$this._tokensOut = tokensOut;

  int? _tokensCacheRead;
  int? get tokensCacheRead => _$this._tokensCacheRead;
  set tokensCacheRead(int? tokensCacheRead) =>
      _$this._tokensCacheRead = tokensCacheRead;

  int? _tokensCacheWrite;
  int? get tokensCacheWrite => _$this._tokensCacheWrite;
  set tokensCacheWrite(int? tokensCacheWrite) =>
      _$this._tokensCacheWrite = tokensCacheWrite;

  int? _tokensCacheWrite1h;
  int? get tokensCacheWrite1h => _$this._tokensCacheWrite1h;
  set tokensCacheWrite1h(int? tokensCacheWrite1h) =>
      _$this._tokensCacheWrite1h = tokensCacheWrite1h;

  int? _subagents;
  int? get subagents => _$this._subagents;
  set subagents(int? subagents) => _$this._subagents = subagents;

  int? _backgroundJobs;
  int? get backgroundJobs => _$this._backgroundJobs;
  set backgroundJobs(int? backgroundJobs) =>
      _$this._backgroundJobs = backgroundJobs;

  int? _apiRetries;
  int? get apiRetries => _$this._apiRetries;
  set apiRetries(int? apiRetries) => _$this._apiRetries = apiRetries;

  RunApiRetryBuilder? _lastApiRetry;
  RunApiRetryBuilder get lastApiRetry =>
      _$this._lastApiRetry ??= RunApiRetryBuilder();
  set lastApiRetry(RunApiRetryBuilder? lastApiRetry) =>
      _$this._lastApiRetry = lastApiRetry;

  RunMetricsBuilder() {
    RunMetrics._defaults(this);
  }

  RunMetricsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _contextPct = $v.contextPct;
      _tokensIn = $v.tokensIn;
      _tokensOut = $v.tokensOut;
      _tokensCacheRead = $v.tokensCacheRead;
      _tokensCacheWrite = $v.tokensCacheWrite;
      _tokensCacheWrite1h = $v.tokensCacheWrite1h;
      _subagents = $v.subagents;
      _backgroundJobs = $v.backgroundJobs;
      _apiRetries = $v.apiRetries;
      _lastApiRetry = $v.lastApiRetry?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunMetrics other) {
    _$v = other as _$RunMetrics;
  }

  @override
  void update(void Function(RunMetricsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunMetrics build() => _build();

  _$RunMetrics _build() {
    _$RunMetrics _$result;
    try {
      _$result =
          _$v ??
          _$RunMetrics._(
            contextPct: contextPct,
            tokensIn: tokensIn,
            tokensOut: tokensOut,
            tokensCacheRead: tokensCacheRead,
            tokensCacheWrite: tokensCacheWrite,
            tokensCacheWrite1h: tokensCacheWrite1h,
            subagents: subagents,
            backgroundJobs: backgroundJobs,
            apiRetries: apiRetries,
            lastApiRetry: _lastApiRetry?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'lastApiRetry';
        _lastApiRetry?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunMetrics',
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
