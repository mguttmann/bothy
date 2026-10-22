// SPDX-License-Identifier: Apache-2.0

/// The apps of the administration (docs/spec/01-product.md 5.2, 5.5; 11 4; brief M6-08).
library;

import 'package:flutter/material.dart';

import '../data/admin_repository.dart';
import 'manifest_picker.dart';

/// The installed apps with "Install app" (placeholder of the build in progress).
class AppsList extends StatelessWidget {
  /// Creates the list.
  const AppsList({
    super.key,
    required this.repository,
    required this.onOpen,
    this.pickManifest,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// Opens the page of an app by its ID.
  final ValueChanged<String> onOpen;

  /// The host's file dialog for a manifest, or null when the host has none.
  final ManifestPicker? pickManifest;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after every change, or null.
  final VoidCallback? onChanged;

  @override
  Widget build(BuildContext context) => const SizedBox.shrink();
}
