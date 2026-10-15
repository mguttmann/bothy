// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_behavior_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelBehaviorPut extends ChannelBehaviorPut {
  @override
  final bool requireMention;
  @override
  final bool threadRequireMention;
  @override
  final bool freeResponse;
  @override
  final bool autoThread;
  @override
  final bool allowMentionedAgents;
  @override
  final int historyBackfillLimit;
  @override
  final String prompt;
  @override
  final BuiltList<String> skills;

  factory _$ChannelBehaviorPut([
    void Function(ChannelBehaviorPutBuilder)? updates,
  ]) => (ChannelBehaviorPutBuilder()..update(updates))._build();

  _$ChannelBehaviorPut._({
    required this.requireMention,
    required this.threadRequireMention,
    required this.freeResponse,
    required this.autoThread,
    required this.allowMentionedAgents,
    required this.historyBackfillLimit,
    required this.prompt,
    required this.skills,
  }) : super._();
  @override
  ChannelBehaviorPut rebuild(
    void Function(ChannelBehaviorPutBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ChannelBehaviorPutBuilder toBuilder() =>
      ChannelBehaviorPutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelBehaviorPut &&
        requireMention == other.requireMention &&
        threadRequireMention == other.threadRequireMention &&
        freeResponse == other.freeResponse &&
        autoThread == other.autoThread &&
        allowMentionedAgents == other.allowMentionedAgents &&
        historyBackfillLimit == other.historyBackfillLimit &&
        prompt == other.prompt &&
        skills == other.skills;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, requireMention.hashCode);
    _$hash = $jc(_$hash, threadRequireMention.hashCode);
    _$hash = $jc(_$hash, freeResponse.hashCode);
    _$hash = $jc(_$hash, autoThread.hashCode);
    _$hash = $jc(_$hash, allowMentionedAgents.hashCode);
    _$hash = $jc(_$hash, historyBackfillLimit.hashCode);
    _$hash = $jc(_$hash, prompt.hashCode);
    _$hash = $jc(_$hash, skills.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ChannelBehaviorPut')
          ..add('requireMention', requireMention)
          ..add('threadRequireMention', threadRequireMention)
          ..add('freeResponse', freeResponse)
          ..add('autoThread', autoThread)
          ..add('allowMentionedAgents', allowMentionedAgents)
          ..add('historyBackfillLimit', historyBackfillLimit)
          ..add('prompt', prompt)
          ..add('skills', skills))
        .toString();
  }
}

class ChannelBehaviorPutBuilder
    implements Builder<ChannelBehaviorPut, ChannelBehaviorPutBuilder> {
  _$ChannelBehaviorPut? _$v;

  bool? _requireMention;
  bool? get requireMention => _$this._requireMention;
  set requireMention(bool? requireMention) =>
      _$this._requireMention = requireMention;

  bool? _threadRequireMention;
  bool? get threadRequireMention => _$this._threadRequireMention;
  set threadRequireMention(bool? threadRequireMention) =>
      _$this._threadRequireMention = threadRequireMention;

  bool? _freeResponse;
  bool? get freeResponse => _$this._freeResponse;
  set freeResponse(bool? freeResponse) => _$this._freeResponse = freeResponse;

  bool? _autoThread;
  bool? get autoThread => _$this._autoThread;
  set autoThread(bool? autoThread) => _$this._autoThread = autoThread;

  bool? _allowMentionedAgents;
  bool? get allowMentionedAgents => _$this._allowMentionedAgents;
  set allowMentionedAgents(bool? allowMentionedAgents) =>
      _$this._allowMentionedAgents = allowMentionedAgents;

  int? _historyBackfillLimit;
  int? get historyBackfillLimit => _$this._historyBackfillLimit;
  set historyBackfillLimit(int? historyBackfillLimit) =>
      _$this._historyBackfillLimit = historyBackfillLimit;

  String? _prompt;
  String? get prompt => _$this._prompt;
  set prompt(String? prompt) => _$this._prompt = prompt;

  ListBuilder<String>? _skills;
  ListBuilder<String> get skills => _$this._skills ??= ListBuilder<String>();
  set skills(ListBuilder<String>? skills) => _$this._skills = skills;

  ChannelBehaviorPutBuilder() {
    ChannelBehaviorPut._defaults(this);
  }

  ChannelBehaviorPutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _requireMention = $v.requireMention;
      _threadRequireMention = $v.threadRequireMention;
      _freeResponse = $v.freeResponse;
      _autoThread = $v.autoThread;
      _allowMentionedAgents = $v.allowMentionedAgents;
      _historyBackfillLimit = $v.historyBackfillLimit;
      _prompt = $v.prompt;
      _skills = $v.skills.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelBehaviorPut other) {
    _$v = other as _$ChannelBehaviorPut;
  }

  @override
  void update(void Function(ChannelBehaviorPutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelBehaviorPut build() => _build();

  _$ChannelBehaviorPut _build() {
    _$ChannelBehaviorPut _$result;
    try {
      _$result =
          _$v ??
          _$ChannelBehaviorPut._(
            requireMention: BuiltValueNullFieldError.checkNotNull(
              requireMention,
              r'ChannelBehaviorPut',
              'requireMention',
            ),
            threadRequireMention: BuiltValueNullFieldError.checkNotNull(
              threadRequireMention,
              r'ChannelBehaviorPut',
              'threadRequireMention',
            ),
            freeResponse: BuiltValueNullFieldError.checkNotNull(
              freeResponse,
              r'ChannelBehaviorPut',
              'freeResponse',
            ),
            autoThread: BuiltValueNullFieldError.checkNotNull(
              autoThread,
              r'ChannelBehaviorPut',
              'autoThread',
            ),
            allowMentionedAgents: BuiltValueNullFieldError.checkNotNull(
              allowMentionedAgents,
              r'ChannelBehaviorPut',
              'allowMentionedAgents',
            ),
            historyBackfillLimit: BuiltValueNullFieldError.checkNotNull(
              historyBackfillLimit,
              r'ChannelBehaviorPut',
              'historyBackfillLimit',
            ),
            prompt: BuiltValueNullFieldError.checkNotNull(
              prompt,
              r'ChannelBehaviorPut',
              'prompt',
            ),
            skills: skills.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'skills';
        skills.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ChannelBehaviorPut',
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
