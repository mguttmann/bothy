// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_api_retry.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunApiRetry extends RunApiRetry {
  @override
  final int attempt;
  @override
  final int? maxRetries;
  @override
  final int? retryDelayMs;
  @override
  final String? error;

  factory _$RunApiRetry([void Function(RunApiRetryBuilder)? updates]) =>
      (RunApiRetryBuilder()..update(updates))._build();

  _$RunApiRetry._({
    required this.attempt,
    this.maxRetries,
    this.retryDelayMs,
    this.error,
  }) : super._();
  @override
  RunApiRetry rebuild(void Function(RunApiRetryBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunApiRetryBuilder toBuilder() => RunApiRetryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunApiRetry &&
        attempt == other.attempt &&
        maxRetries == other.maxRetries &&
        retryDelayMs == other.retryDelayMs &&
        error == other.error;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, attempt.hashCode);
    _$hash = $jc(_$hash, maxRetries.hashCode);
    _$hash = $jc(_$hash, retryDelayMs.hashCode);
    _$hash = $jc(_$hash, error.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunApiRetry')
          ..add('attempt', attempt)
          ..add('maxRetries', maxRetries)
          ..add('retryDelayMs', retryDelayMs)
          ..add('error', error))
        .toString();
  }
}

class RunApiRetryBuilder implements Builder<RunApiRetry, RunApiRetryBuilder> {
  _$RunApiRetry? _$v;

  int? _attempt;
  int? get attempt => _$this._attempt;
  set attempt(int? attempt) => _$this._attempt = attempt;

  int? _maxRetries;
  int? get maxRetries => _$this._maxRetries;
  set maxRetries(int? maxRetries) => _$this._maxRetries = maxRetries;

  int? _retryDelayMs;
  int? get retryDelayMs => _$this._retryDelayMs;
  set retryDelayMs(int? retryDelayMs) => _$this._retryDelayMs = retryDelayMs;

  String? _error;
  String? get error => _$this._error;
  set error(String? error) => _$this._error = error;

  RunApiRetryBuilder() {
    RunApiRetry._defaults(this);
  }

  RunApiRetryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _attempt = $v.attempt;
      _maxRetries = $v.maxRetries;
      _retryDelayMs = $v.retryDelayMs;
      _error = $v.error;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunApiRetry other) {
    _$v = other as _$RunApiRetry;
  }

  @override
  void update(void Function(RunApiRetryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunApiRetry build() => _build();

  _$RunApiRetry _build() {
    final _$result =
        _$v ??
        _$RunApiRetry._(
          attempt: BuiltValueNullFieldError.checkNotNull(
            attempt,
            r'RunApiRetry',
            'attempt',
          ),
          maxRetries: maxRetries,
          retryDelayMs: retryDelayMs,
          error: error,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
