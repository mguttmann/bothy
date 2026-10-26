// SPDX-License-Identifier: Apache-2.0

// The targets and files of the share sheet tests and goldens (white label: `example.com`).

import 'package:client/share/share_target.dart';

import '../support/structure.dart';

/// The recent targets of the sheet tests: a thread, a channel and a new post.
final List<ShareTarget> sheetTargets = [
  ShareTarget(
    kind: ShareTargetKind.thread,
    channelId: channelId(1),
    threadId: threadId(1),
    path: 'server > Gateway: resume',
  ),
  ShareTarget(
    kind: ShareTargetKind.newPost,
    channelId: channelId(2),
    path: 'requests',
  ),
  ShareTarget(
    kind: ShareTargetKind.channel,
    channelId: channelId(3),
    threadId: threadId(0x83),
    path: 'general',
  ),
];

/// A shared file of [size] bytes named [name].
ShareFile sharedFile(String name, int size) => ShareFile(
  path: '/cache/$name',
  name: name,
  mimeType: 'image/png',
  sizeBytes: size,
);
