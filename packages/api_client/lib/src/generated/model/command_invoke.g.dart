// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command_invoke.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CommandInvoke extends CommandInvoke {
  @override
  final String? threadId;
  @override
  final BuiltList<CommandOptionValue>? options;

  factory _$CommandInvoke([void Function(CommandInvokeBuilder)? updates]) =>
      (CommandInvokeBuilder()..update(updates))._build();

  _$CommandInvoke._({this.threadId, this.options}) : super._();
  @override
  CommandInvoke rebuild(void Function(CommandInvokeBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CommandInvokeBuilder toBuilder() => CommandInvokeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CommandInvoke &&
        threadId == other.threadId &&
        options == other.options;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, options.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CommandInvoke')
          ..add('threadId', threadId)
          ..add('options', options))
        .toString();
  }
}

class CommandInvokeBuilder
    implements Builder<CommandInvoke, CommandInvokeBuilder> {
  _$CommandInvoke? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  ListBuilder<CommandOptionValue>? _options;
  ListBuilder<CommandOptionValue> get options =>
      _$this._options ??= ListBuilder<CommandOptionValue>();
  set options(ListBuilder<CommandOptionValue>? options) =>
      _$this._options = options;

  CommandInvokeBuilder() {
    CommandInvoke._defaults(this);
  }

  CommandInvokeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _options = $v.options?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CommandInvoke other) {
    _$v = other as _$CommandInvoke;
  }

  @override
  void update(void Function(CommandInvokeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CommandInvoke build() => _build();

  _$CommandInvoke _build() {
    _$CommandInvoke _$result;
    try {
      _$result =
          _$v ??
          _$CommandInvoke._(threadId: threadId, options: _options?.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'options';
        _options?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CommandInvoke',
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
