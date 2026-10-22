// SPDX-License-Identifier: Apache-2.0

/// The incoming webhooks per channel (11 8; brief M6-08 Design notes 4 and 5).
library;

import 'package:flutter/material.dart';

import '../data/admin_repository.dart';

/// The incoming webhooks of one channel (placeholder of the build in progress).
class IncomingWebhooksView extends StatelessWidget {
  /// Creates the view.
  const IncomingWebhooksView({
    super.key,
    required this.repository,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
    this.newSlugSuffix,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after every change, or null.
  final VoidCallback? onChanged;

  /// The random suffix of the slug of an "incoming app", or null for eight random characters
  /// of `[a-z0-9]` (tests pass a fixed one).
  final String Function()? newSlugSuffix;

  @override
  Widget build(BuildContext context) => const SizedBox.shrink();
}
