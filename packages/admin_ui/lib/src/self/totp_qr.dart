// SPDX-License-Identifier: Apache-2.0

/// The QR code of a one-time code setup (docs/spec/08-identity.md 2.5: `totp_enroll` returns the
/// secret once as an otpauth URI; design note docs/design/sign-in-methods.md item 4).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:qr/qr.dart';

/// The secret of the otpauth URI [uri] in groups of four for typing, or null when the URI has
/// none.
///
/// Example: `totpSecretOf('otpauth://totp/x:alex?secret=JBSWY3DPEHPK3PXP')` is
/// `'JBSW Y3DP EHPK 3PXP'`.
String? totpSecretOf(String uri) {
  final secret = Uri.tryParse(uri)?.queryParameters['secret'];
  if (secret == null || secret.isEmpty) return null;
  final groups = <String>[
    for (var i = 0; i < secret.length; i += 4)
      secret.substring(i, i + 4 > secret.length ? secret.length : i + 4),
  ];
  return groups.join(' ');
}

/// [data] as a QR code: the text color of the light theme on its canvas with the quiet zone of
/// four modules the standard asks for, whatever the theme, because authenticator apps read dark
/// on light best.
///
/// The view is one image for screen readers with [semanticLabel]; the secret stays readable as
/// text next to it ([totpSecretOf]).
class TotpQrCode extends StatelessWidget {
  /// Creates the code.
  const TotpQrCode({
    super.key,
    required this.data,
    required this.semanticLabel,
    this.size = 200,
  });

  /// The text to encode (the otpauth URI).
  final String data;

  /// What a screen reader says for the image.
  final String semanticLabel;

  /// The side length in logical pixels.
  final double size;

  @override
  Widget build(BuildContext context) {
    final image = QrImage(
      QrCode(
        payload: QrPayload.fromString(data),
        errorCorrectLevel: QrErrorCorrectLevel.medium,
      ),
    );
    return Semantics(
      image: true,
      label: semanticLabel,
      child: SizedBox.square(
        dimension: size,
        child: CustomPaint(painter: QrModulesPainter(image)),
      ),
    );
  }
}

/// Paints the modules of a [QrImage] with a quiet zone of four modules.
class QrModulesPainter extends CustomPainter {
  /// Creates the painter.
  QrModulesPainter(this.image);

  /// The modules.
  final QrImage image;

  /// The quiet zone around the code, in modules (ISO/IEC 18004).
  static const int quietZone = 4;

  @override
  void paint(Canvas canvas, Size size) {
    final count = image.moduleCount + 2 * quietZone;
    final module = size.shortestSide / count;
    canvas.drawRect(
      Offset.zero & size,
      Paint()..color = DsColors.light.surfaceCanvas,
    );
    final dark = Paint()
      ..color = DsColors.light.textPrimary
      ..isAntiAlias = false;
    for (var row = 0; row < image.moduleCount; row++) {
      for (var col = 0; col < image.moduleCount; col++) {
        if (!image.isDark(row, col)) continue;
        canvas.drawRect(
          Rect.fromLTWH(
            (col + quietZone) * module,
            (row + quietZone) * module,
            module,
            module,
          ),
          dark,
        );
      }
    }
  }

  @override
  bool shouldRepaint(QrModulesPainter old) => !identical(old.image, image);
}
