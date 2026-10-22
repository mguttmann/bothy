// SPDX-License-Identifier: Apache-2.0

/// The outgoing webhook subscriptions of an app and their deliveries (11 6, 7; brief M6-08
/// Design note 3).
library;

import 'package:flutter/material.dart';

import '../data/admin_repository.dart';
import '../data/apps_repository.dart';

/// The subscriptions of [app] (placeholder of the build in progress).
class AppSubscriptionsSection extends StatelessWidget {
  /// Creates the section.
  const AppSubscriptionsSection({
    super.key,
    required this.repository,
    required this.app,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The app with its granted events.
  final AppRecord app;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after every change, or null.
  final VoidCallback? onChanged;

  @override
  Widget build(BuildContext context) => const SizedBox.shrink();
}
