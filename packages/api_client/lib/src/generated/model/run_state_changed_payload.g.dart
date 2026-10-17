// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_state_changed_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunStateChangedPayload extends RunStateChangedPayload {
  @override
  final RunSummary run;
  @override
  final String? triggeringMessageId;
  @override
  final TriggerState? triggerState;

  factory _$RunStateChangedPayload([
    void Function(RunStateChangedPayloadBuilder)? updates,
  ]) => (RunStateChangedPayloadBuilder()..update(updates))._build();

  _$RunStateChangedPayload._({
    required this.run,
    this.triggeringMessageId,
    this.triggerState,
  }) : super._();
  @override
  RunStateChangedPayload rebuild(
    void Function(RunStateChangedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  RunStateChangedPayloadBuilder toBuilder() =>
      RunStateChangedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunStateChangedPayload &&
        run == other.run &&
        triggeringMessageId == other.triggeringMessageId &&
        triggerState == other.triggerState;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, run.hashCode);
    _$hash = $jc(_$hash, triggeringMessageId.hashCode);
    _$hash = $jc(_$hash, triggerState.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunStateChangedPayload')
          ..add('run', run)
          ..add('triggeringMessageId', triggeringMessageId)
          ..add('triggerState', triggerState))
        .toString();
  }
}

class RunStateChangedPayloadBuilder
    implements Builder<RunStateChangedPayload, RunStateChangedPayloadBuilder> {
  _$RunStateChangedPayload? _$v;

  RunSummaryBuilder? _run;
  RunSummaryBuilder get run => _$this._run ??= RunSummaryBuilder();
  set run(RunSummaryBuilder? run) => _$this._run = run;

  String? _triggeringMessageId;
  String? get triggeringMessageId => _$this._triggeringMessageId;
  set triggeringMessageId(String? triggeringMessageId) =>
      _$this._triggeringMessageId = triggeringMessageId;

  TriggerState? _triggerState;
  TriggerState? get triggerState => _$this._triggerState;
  set triggerState(TriggerState? triggerState) =>
      _$this._triggerState = triggerState;

  RunStateChangedPayloadBuilder() {
    RunStateChangedPayload._defaults(this);
  }

  RunStateChangedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _run = $v.run.toBuilder();
      _triggeringMessageId = $v.triggeringMessageId;
      _triggerState = $v.triggerState;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunStateChangedPayload other) {
    _$v = other as _$RunStateChangedPayload;
  }

  @override
  void update(void Function(RunStateChangedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunStateChangedPayload build() => _build();

  _$RunStateChangedPayload _build() {
    _$RunStateChangedPayload _$result;
    try {
      _$result =
          _$v ??
          _$RunStateChangedPayload._(
            run: run.build(),
            triggeringMessageId: triggeringMessageId,
            triggerState: triggerState,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'run';
        run.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunStateChangedPayload',
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
