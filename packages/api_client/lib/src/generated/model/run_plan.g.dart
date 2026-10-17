// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_plan.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunPlan extends RunPlan {
  @override
  final BuiltList<RunPlanItemsInner> items;

  factory _$RunPlan([void Function(RunPlanBuilder)? updates]) =>
      (RunPlanBuilder()..update(updates))._build();

  _$RunPlan._({required this.items}) : super._();
  @override
  RunPlan rebuild(void Function(RunPlanBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunPlanBuilder toBuilder() => RunPlanBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunPlan && items == other.items;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'RunPlan',
    )..add('items', items)).toString();
  }
}

class RunPlanBuilder implements Builder<RunPlan, RunPlanBuilder> {
  _$RunPlan? _$v;

  ListBuilder<RunPlanItemsInner>? _items;
  ListBuilder<RunPlanItemsInner> get items =>
      _$this._items ??= ListBuilder<RunPlanItemsInner>();
  set items(ListBuilder<RunPlanItemsInner>? items) => _$this._items = items;

  RunPlanBuilder() {
    RunPlan._defaults(this);
  }

  RunPlanBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunPlan other) {
    _$v = other as _$RunPlan;
  }

  @override
  void update(void Function(RunPlanBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunPlan build() => _build();

  _$RunPlan _build() {
    _$RunPlan _$result;
    try {
      _$result = _$v ?? _$RunPlan._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunPlan',
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
