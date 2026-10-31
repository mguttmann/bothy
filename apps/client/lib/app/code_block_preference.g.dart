// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'code_block_preference.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Whether the code blocks of messages show one line per line with a horizontal scroll on this
/// device ("Wrap lines" off), from the local view state (`shell_view_state`, M4-49); false (long
/// lines wrap, 01 6.14 item 1) without a local database or until it answers. Kept per device
/// and never synchronized, like the other view state of the shell (D-M4-49-2).

@ProviderFor(codeBlocksUnwrapped)
final codeBlocksUnwrappedProvider = CodeBlocksUnwrappedProvider._();

/// Whether the code blocks of messages show one line per line with a horizontal scroll on this
/// device ("Wrap lines" off), from the local view state (`shell_view_state`, M4-49); false (long
/// lines wrap, 01 6.14 item 1) without a local database or until it answers. Kept per device
/// and never synchronized, like the other view state of the shell (D-M4-49-2).

final class CodeBlocksUnwrappedProvider
    extends $FunctionalProvider<AsyncValue<bool>, bool, Stream<bool>>
    with $FutureModifier<bool>, $StreamProvider<bool> {
  /// Whether the code blocks of messages show one line per line with a horizontal scroll on this
  /// device ("Wrap lines" off), from the local view state (`shell_view_state`, M4-49); false (long
  /// lines wrap, 01 6.14 item 1) without a local database or until it answers. Kept per device
  /// and never synchronized, like the other view state of the shell (D-M4-49-2).
  CodeBlocksUnwrappedProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'codeBlocksUnwrappedProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$codeBlocksUnwrappedHash();

  @$internal
  @override
  $StreamProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<bool> create(Ref ref) {
    return codeBlocksUnwrapped(ref);
  }
}

String _$codeBlocksUnwrappedHash() =>
    r'd316c7871d257299bf3720830736d4e54e677054';
