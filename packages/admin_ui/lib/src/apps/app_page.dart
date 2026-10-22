// SPDX-License-Identifier: Apache-2.0

/// The page of one app (docs/spec/01-product.md 5.2; 11 4.2, 4.3, 7; brief M6-08).
library;

import 'package:flutter/material.dart';

import '../data/admin_repository.dart';
import 'manifest_picker.dart';

/// One app with its consent, its lifecycle and its subscriptions (placeholder of the build in
/// progress).
class AppPage extends StatelessWidget {
  /// Creates the page.
  const AppPage({
    super.key,
    required this.repository,
    required this.appId,
    this.pickManifest,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onBack,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The app.
  final String appId;

  /// The host's file dialog for a manifest, or null when the host has none.
  final ManifestPicker? pickManifest;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Goes back to the list, or null.
  final VoidCallback? onBack;

  /// Runs after every change, or null.
  final VoidCallback? onChanged;

  @override
  Widget build(BuildContext context) => const SizedBox.shrink();
}
