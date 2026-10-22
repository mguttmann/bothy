// SPDX-License-Identifier: Apache-2.0

/// The transcript of a voice message (12 8.2; 15 13; brief M4-22 Design note 4): it appears
/// when `file.updated` reports `transcript_status = done` (the text comes from
/// `getTranscript`, the local row holds no text), "Transcribe" calls `requestTranscription`,
/// and `409 conflict` with `reason` `transcription_unavailable` shows a short text.
library;

import 'dart:async';
import 'dart:math';

import 'package:api_client/api_client.dart' show ApiClient, ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../files/providers.dart';

/// The transcript operations of `api_client` for the views.
///
/// Example: `final text = await transcripts.text(file.id);`
final class FileTranscripts {
  /// Creates the operations on [api]; [newKey] makes the `Idempotency-Key` of a request
  /// (default: a UUIDv7 from a secure source).
  FileTranscripts(this._api, {String Function()? newKey})
    : _newKey = newKey ?? _secureIds().next;

  final ApiClient _api;
  final String Function() _newKey;

  static Uuid7Generator _secureIds() {
    final random = Random.secure();
    return Uuid7Generator(
      nowMillis: () => DateTime.now().millisecondsSinceEpoch,
      random: (bits) {
        var value = 0;
        for (var left = bits; left > 0; left -= 30) {
          final take = left > 30 ? 30 : left;
          value = (value << take) | random.nextInt(1 << take);
        }
        return value;
      },
    );
  }

  /// The text of the transcript of [fileId], or null when there is none.
  ///
  /// Throws the `DioException` of the request.
  Future<String?> text(FileId fileId) async {
    final t = (await _api.getFilesApi().getTranscript(fileId: fileId.value))
        .data;
    return t?.text;
  }

  /// Asks the server for a transcript of [fileId] (`file.transcribe`); returns null when it
  /// accepted, else the problem code (with its `reason` for a `409 conflict`).
  Future<String?> request(FileId fileId) async {
    try {
      await _api.getFilesApi().requestTranscription(
        fileId: fileId.value,
        idempotencyKey: _newKey(),
      );
      return null;
    } on DioException catch (e) {
      final response = e.response;
      final p =
          ApiProblem.of(e) ??
          (response == null ? null : ApiProblem.fromResponse(response));
      if (p == null) return 'network';
      return p.reason ?? p.code;
    }
  }
}

/// The transcript of [file] in its state: pending, the text, none or failed with
/// "Transcribe".
class TranscriptView extends ConsumerStatefulWidget {
  /// Creates the view of the voice file [file].
  const TranscriptView({super.key, required this.file});

  /// The voice file.
  final FileMeta file;

  @override
  ConsumerState<TranscriptView> createState() => _TranscriptViewState();
}

class _TranscriptViewState extends ConsumerState<TranscriptView> {
  String? _notice;
  bool _asking = false;

  Future<void> _request() async {
    final l = AppLocalizations.of(context);
    final transcripts = ref.read(fileTranscriptsProvider);
    if (transcripts == null || _asking) return;
    setState(() {
      _asking = true;
      _notice = null;
    });
    final refused = await transcripts.request(widget.file.id);
    if (!mounted) return;
    setState(() {
      _asking = false;
      _notice = switch (refused) {
        null => null,
        'transcription_unavailable' => l.transcriptUnavailable,
        final code => problemText(l, code),
      };
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final status = widget.file.transcriptStatus;
    Text line(String text, {Key? key, bool muted = true}) => Text(
      text,
      key: key,
      style: dsTextStyle(DsType.bodySm)
          .copyWith(color: muted ? colors.textMuted : colors.textPrimary),
    );
    final children = <Widget>[];
    switch (status) {
      case TranscriptStatus.done:
        final text = ref
            .watch(transcriptTextProvider(widget.file.id.value))
            .value;
        children.add(
          text == null
              ? line(l.messageVoiceTranscriptPending)
              : Semantics(
                  label: l.transcriptLabel,
                  child: SelectableText(
                    text,
                    key: const ValueKey('transcript.text'),
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: colors.textPrimary),
                  ),
                ),
        );
      case TranscriptStatus.pending:
        children.add(line(l.messageVoiceTranscriptPending));
      case TranscriptStatus.failed ||
          TranscriptStatus.none ||
          TranscriptStatus.unknown:
        if (status == TranscriptStatus.failed) {
          children.add(line(l.transcriptFailed));
        }
        children.add(
          Align(
            alignment: AlignmentDirectional.centerStart,
            child: DsButton(
              key: const ValueKey('transcript.request'),
              label: l.transcriptRequest,
              style: DsButtonStyle.ghost,
              loading: _asking,
              onPressed: () => unawaited(_request()),
            ),
          ),
        );
    }
    if (_notice case final notice?) {
      children.add(line(notice, key: const ValueKey('transcript.notice')));
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: children,
    );
  }
}
