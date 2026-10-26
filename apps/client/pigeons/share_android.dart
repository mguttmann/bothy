// SPDX-License-Identifier: Apache-2.0

/// The Pigeon source of the sharing channel between the app and the Kotlin glue of its Android
/// runner (brief M5-06 Design note 6; docs/decisions.d/M5-06.md).
///
/// Regenerate with `dart run pigeon --input pigeons/share_android.dart` in `apps/client`;
/// `make codegen-check` regenerates and compares. The generated files are
/// `lib/src/generated/share_android_api.g.dart` and
/// `android/app/src/main/kotlin/com/example/client/ShareAndroidApi.g.kt`.
library;

import 'package:pigeon/pigeon.dart';

/// One shared file, copied by the runner from its content URI into the app's cache.
class SharedFileRecord {
  /// Creates the file.
  SharedFileRecord({
    required this.path,
    required this.name,
    required this.mimeType,
    required this.sizeBytes,
  });

  /// The absolute path of the copy in the app's cache; the app deletes it after the outbox
  /// took the file into its own storage.
  String path;

  /// The display name the sending app gave, or a name derived from the type.
  String name;

  /// The MIME type of the intent or of the content resolver.
  String mimeType;

  /// The size of the copy in bytes.
  int sizeBytes;
}

/// What another app shared through `ACTION_SEND` or `ACTION_SEND_MULTIPLE`.
class SharedContentRecord {
  /// Creates the content.
  SharedContentRecord({required this.id, required this.files, this.text});

  /// A random ID of this share, so the app takes it over once.
  String id;

  /// `EXTRA_TEXT` (text or a link), or null.
  String? text;

  /// The shared files in their order.
  List<SharedFileRecord> files;
}

/// The share receiver of the Android runner: the intent filters of `MainActivity` take
/// `ACTION_SEND` and `ACTION_SEND_MULTIPLE` for concrete MIME types, copy every content URI
/// into the app's cache through `ContentResolver` streams and keep the share until the app
/// takes it.
@ConfigurePigeon(
  PigeonOptions(
    dartOut: 'lib/src/generated/share_android_api.g.dart',
    dartPackageName: 'client',
    kotlinOut:
        'android/app/src/main/kotlin/com/example/client/ShareAndroidApi.g.kt',
    kotlinOptions: KotlinOptions(
      package: 'com.example.client',
      // AndroidPlatformApi.g.kt declares `FlutterError` in the same package.
      errorClassName: 'ShareAndroidPigeonError',
    ),
    copyrightHeader: 'pigeons/copyright.txt',
  ),
)
@HostApi()
abstract class ShareAndroidHostApi {
  /// The share that arrived last and was not taken yet, or null; taking it removes it from the
  /// runner. Waits until its copies are complete.
  @async
  SharedContentRecord? takePendingShare();
}

/// Calls from the runner into the app.
@FlutterApi()
abstract class ShareAndroidFlutterApi {
  /// A share arrived while the app runs; the app calls
  /// [ShareAndroidHostApi.takePendingShare].
  void onShareReceived();
}
