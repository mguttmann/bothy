// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_slots.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunSlots extends RunSlots {
  @override
  final int used;
  @override
  final int total;

  factory _$RunSlots([void Function(RunSlotsBuilder)? updates]) =>
      (RunSlotsBuilder()..update(updates))._build();

  _$RunSlots._({required this.used, required this.total}) : super._();
  @override
  RunSlots rebuild(void Function(RunSlotsBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunSlotsBuilder toBuilder() => RunSlotsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunSlots && used == other.used && total == other.total;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, used.hashCode);
    _$hash = $jc(_$hash, total.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunSlots')
          ..add('used', used)
          ..add('total', total))
        .toString();
  }
}

class RunSlotsBuilder implements Builder<RunSlots, RunSlotsBuilder> {
  _$RunSlots? _$v;

  int? _used;
  int? get used => _$this._used;
  set used(int? used) => _$this._used = used;

  int? _total;
  int? get total => _$this._total;
  set total(int? total) => _$this._total = total;

  RunSlotsBuilder() {
    RunSlots._defaults(this);
  }

  RunSlotsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _used = $v.used;
      _total = $v.total;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunSlots other) {
    _$v = other as _$RunSlots;
  }

  @override
  void update(void Function(RunSlotsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunSlots build() => _build();

  _$RunSlots _build() {
    final _$result =
        _$v ??
        _$RunSlots._(
          used: BuiltValueNullFieldError.checkNotNull(
            used,
            r'RunSlots',
            'used',
          ),
          total: BuiltValueNullFieldError.checkNotNull(
            total,
            r'RunSlots',
            'total',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
