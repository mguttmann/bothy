// SPDX-License-Identifier: Apache-2.0

package devicekey.core

/**
 * The purpose of a device key (docs/spec/08-identitaet.md 6.1, R-CL-05).
 *
 * @property wire the purpose as it appears in the key alias.
 */
enum class KeyPurpose(val wire: String) {
    /** Signs every DPoP proof without asking the user; the push service uses it in the background. */
    DPOP("dpop"),

    /** Signs step-up confirmations; every signature asks the user through `BiometricPrompt`. */
    STEP_UP("step_up"),
}

/**
 * Where the private key of a Keystore entry lives.
 *
 * @property hardwareBound whether the key is reported to the server as hardware-bound.
 */
enum class SecurityLevel(val hardwareBound: Boolean) {
    /** A StrongBox secure element. */
    STRONGBOX(true),

    /** The trusted execution environment. */
    TRUSTED_ENVIRONMENT(true),

    /** Software only: emulators and devices without secure hardware. */
    SOFTWARE(false),
}

/**
 * The public part of a device key.
 *
 * @property x the x coordinate of the P-256 public key, 32 bytes big-endian.
 * @property y the y coordinate, 32 bytes big-endian.
 * @property hardwareBound whether the private key lives in StrongBox or the trusted execution
 *   environment.
 * @property requiresUserPresence whether every signature asks the user (the step-up key).
 */
class DeviceKeyInfo(
    val x: ByteArray,
    val y: ByteArray,
    val hardwareBound: Boolean,
    val requiresUserPresence: Boolean,
) {
    override fun equals(other: Any?): Boolean =
        other is DeviceKeyInfo &&
            x.contentEquals(other.x) &&
            y.contentEquals(other.y) &&
            hardwareBound == other.hardwareBound &&
            requiresUserPresence == other.requiresUserPresence

    override fun hashCode(): Int =
        listOf(x.contentHashCode(), y.contentHashCode(), hardwareBound.hashCode(), requiresUserPresence.hashCode())
            .fold(17) { acc, h -> acc * 31 + h }

    // No coordinates: a log line never needs them, the thumbprint comes from the Dart side.
    override fun toString(): String = "DeviceKeyInfo(hardwareBound=$hardwareBound, requiresUserPresence=$requiresUserPresence)"
}

/**
 * The texts of the step-up prompt. They come from Dart (`packages/l10n`), never from native code
 * (brief M5-01, rule 8).
 *
 * @property title the prompt's title.
 * @property subtitle an optional second line.
 * @property negativeButton the cancel button; shown only where the prompt accepts biometrics alone
 *   (API 26 to 29), because a prompt that accepts the device credential has no negative button.
 */
data class PromptTexts(val title: String, val subtitle: String?, val negativeButton: String)
