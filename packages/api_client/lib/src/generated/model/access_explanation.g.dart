// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'access_explanation.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AccessExplanation extends AccessExplanation {
  @override
  final AccessTargetKind targetKind;
  @override
  final String targetId;
  @override
  final BuiltList<AccessEntry> entries;

  factory _$AccessExplanation([
    void Function(AccessExplanationBuilder)? updates,
  ]) => (AccessExplanationBuilder()..update(updates))._build();

  _$AccessExplanation._({
    required this.targetKind,
    required this.targetId,
    required this.entries,
  }) : super._();
  @override
  AccessExplanation rebuild(void Function(AccessExplanationBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AccessExplanationBuilder toBuilder() =>
      AccessExplanationBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AccessExplanation &&
        targetKind == other.targetKind &&
        targetId == other.targetId &&
        entries == other.entries;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, targetKind.hashCode);
    _$hash = $jc(_$hash, targetId.hashCode);
    _$hash = $jc(_$hash, entries.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AccessExplanation')
          ..add('targetKind', targetKind)
          ..add('targetId', targetId)
          ..add('entries', entries))
        .toString();
  }
}

class AccessExplanationBuilder
    implements Builder<AccessExplanation, AccessExplanationBuilder> {
  _$AccessExplanation? _$v;

  AccessTargetKind? _targetKind;
  AccessTargetKind? get targetKind => _$this._targetKind;
  set targetKind(AccessTargetKind? targetKind) =>
      _$this._targetKind = targetKind;

  String? _targetId;
  String? get targetId => _$this._targetId;
  set targetId(String? targetId) => _$this._targetId = targetId;

  ListBuilder<AccessEntry>? _entries;
  ListBuilder<AccessEntry> get entries =>
      _$this._entries ??= ListBuilder<AccessEntry>();
  set entries(ListBuilder<AccessEntry>? entries) => _$this._entries = entries;

  AccessExplanationBuilder() {
    AccessExplanation._defaults(this);
  }

  AccessExplanationBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _targetKind = $v.targetKind;
      _targetId = $v.targetId;
      _entries = $v.entries.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AccessExplanation other) {
    _$v = other as _$AccessExplanation;
  }

  @override
  void update(void Function(AccessExplanationBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AccessExplanation build() => _build();

  _$AccessExplanation _build() {
    _$AccessExplanation _$result;
    try {
      _$result =
          _$v ??
          _$AccessExplanation._(
            targetKind: BuiltValueNullFieldError.checkNotNull(
              targetKind,
              r'AccessExplanation',
              'targetKind',
            ),
            targetId: BuiltValueNullFieldError.checkNotNull(
              targetId,
              r'AccessExplanation',
              'targetId',
            ),
            entries: entries.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'entries';
        entries.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AccessExplanation',
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
