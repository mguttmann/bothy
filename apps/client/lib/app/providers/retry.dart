// SPDX-License-Identifier: Apache-2.0

/// The one rule for Riverpod's retry of a failed provider in the app (AUD-PKG-P02-client-b).
library;

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// The retry of every provider without its own: a REST request that failed
/// ([DioException]) is not built again, because the read retry of the API client already made
/// its attempts (`ReadRetryInterceptor`, D-M4-02-5), and Riverpod's default would repeat all of
/// them up to ten more times against a server that answered `429` or `503`. Any other failure
/// keeps Riverpod's default ([ProviderContainer.defaultRetry]).
///
/// Example: `ProviderScope(retry: appProviderRetry, child: const ClientApp())`.
Duration? appProviderRetry(int retryCount, Object error) =>
    error is DioException
    ? null
    : ProviderContainer.defaultRetry(retryCount, error);
