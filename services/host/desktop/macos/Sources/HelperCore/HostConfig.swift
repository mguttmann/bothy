import Foundation

/// The one value the helper takes from the host service's configuration file (D-CU-02-8):
/// `host_service.computer_frame_max_bytes`.
///
/// The helper has no configuration of its own. It reads the host service's file when the file
/// is readable for the helper's account and otherwise uses the default of
/// docs/spec/27-computer-use.md section 18. The reader understands only the block style the host
/// service writes (`host_service:` at the start of a line, its keys indented below); any other
/// shape falls back to the default, because the host service refuses a channel frame above its
/// own limit anyway and a smaller helper limit only ends a connection early.
public enum HostConfig {
    /// The default of `host_service.computer_frame_max_bytes` (27 section 18).
    public static let defaultFrameMaxBytes = 67_108_864

    /// The path of the host service's configuration file on macOS (24 2.2, M2-42).
    public static let macOSConfigPath = "/Library/Application Support/bothy-host/config.yaml"

    /// Reads `host_service.computer_frame_max_bytes` from a configuration file.
    ///
    /// - Parameter configPath: the YAML file of the host service.
    /// - Returns: the configured positive value, else `defaultFrameMaxBytes` (missing or
    ///   unreadable file, missing key, a value that is not a positive integer).
    public static func frameMaxBytes(configPath: String = macOSConfigPath) -> Int {
        guard let data = FileManager.default.contents(atPath: configPath),
            let text = String(data: data, encoding: .utf8)
        else { return defaultFrameMaxBytes }
        return frameMaxBytes(fromYAML: text)
    }

    /// Reads `host_service.computer_frame_max_bytes` from the text of a configuration file.
    ///
    /// - Parameter text: the YAML text.
    /// - Returns: the configured positive value, else `defaultFrameMaxBytes`.
    ///
    /// Example: `host_service:\n  computer_frame_max_bytes: 1048576` gives 1048576.
    public static func frameMaxBytes(fromYAML text: String) -> Int {
        var inSection = false
        var keyIndent: Int?
        for rawLine in text.components(separatedBy: .newlines) {
            let line = stripComment(rawLine)
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if trimmed.isEmpty { continue }
            let indent = line.prefix { $0 == " " }.count
            if indent == 0 {
                inSection = trimmed == "host_service:"
                keyIndent = nil
                continue
            }
            guard inSection else { continue }
            // Only direct children of host_service count; a deeper key of the same name
            // belongs to a nested map.
            if keyIndent == nil { keyIndent = indent }
            guard indent == keyIndent, trimmed.hasPrefix("computer_frame_max_bytes:") else {
                continue
            }
            let value = trimmed.dropFirst("computer_frame_max_bytes:".count)
                .trimmingCharacters(in: .whitespaces)
            if let number = Int(value), number > 0 { return number }
            return defaultFrameMaxBytes
        }
        return defaultFrameMaxBytes
    }

    /// Drops a `#` comment that starts the line or follows whitespace; quoted values are not
    /// expected for the key the helper reads.
    private static func stripComment(_ line: String) -> String {
        var previous: Character = " "
        for (offset, character) in line.enumerated() {
            if character == "#", previous == " " || previous == "\t" {
                return String(line.prefix(offset))
            }
            previous = character
        }
        return line
    }
}
