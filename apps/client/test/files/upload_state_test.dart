// SPDX-License-Identifier: Apache-2.0

// FIX-upload-retry-storm: an upload that the server refused for the moment (`429`, `503`, no
// network) and that waits for its next attempt says so ("Paused, trying again shortly")
// instead of "Uploading 0 %" or "Waiting to upload" forever; one whose turn has not come waits.

import 'package:client/files/attachment_mapping.dart';
import 'package:flutter/widgets.dart' show Locale;
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

OutboxEntryView _view(OutboxStateLabel label, {String? code}) {
  final at = DateTime.utc(2026, 10, 30, 2, 48);
  return OutboxEntryView(
    entry: OutboxEntry(
      id: '01a18ab8-e43c-7262-ae41-759360c40a52',
      kind: 'upload',
      payload: '{}',
      state: switch (label) {
        OutboxStateLabel.pending => OutboxState.pending,
        OutboxStateLabel.sending => OutboxState.sending,
        OutboxStateLabel.failed => OutboxState.failed,
        _ => OutboxState.waiting,
      },
      attempts: 1,
      lastErrorCode: code,
      createdAt: at,
      stateChangedAt: at,
    ),
    stateLabel: label,
  );
}

void main() {
  final l = lookupAppLocalizations(const Locale('en'));
  final de = lookupAppLocalizations(const Locale('de'));

  test('the views of an upload entry', () {
    expect(
      stateOfUpload(_view(OutboxStateLabel.pending)),
      AttachmentState.waiting,
    );
    expect(
      stateOfUpload(_view(OutboxStateLabel.sending)),
      AttachmentState.uploading,
    );
    expect(
      stateOfUpload(_view(OutboxStateLabel.waiting)),
      AttachmentState.retrying,
    );
    expect(
      stateOfUpload(_view(OutboxStateLabel.delayed)),
      AttachmentState.retrying,
    );
    expect(
      stateOfUpload(_view(OutboxStateLabel.failed, code: 'gone')),
      AttachmentState.canceled,
    );
    expect(
      stateOfUpload(_view(OutboxStateLabel.failed, code: 'conflict')),
      AttachmentState.failed,
    );
  });

  test('a paused upload names the pause, not a percentage', () {
    expect(
      attachmentStateLabel(l, AttachmentState.retrying),
      'Paused, trying again shortly',
    );
    expect(
      attachmentStateLabel(de, AttachmentState.retrying),
      'Pausiert, gleich neuer Versuch',
    );
  });
}
