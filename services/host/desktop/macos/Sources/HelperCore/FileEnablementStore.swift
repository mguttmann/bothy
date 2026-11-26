import Foundation

/// The enablement in the helper's own state file (D-CU-02-4):
/// `{"v": 1, "enabled": <bool>, "generation": <int>}`, file mode `0600`, folder mode `0700`.
///
/// The file is read at start only and never watched: a change of the file is no order, so a
/// process that rewrites it reaches nothing before the next start, and only the click in the
/// menu switches on while the helper runs.
///
/// Example:
/// ```swift
/// let store = FileEnablementStore(folder: FileEnablementStore.defaultFolder())
/// var state = store.load()
/// ```
public final class FileEnablementStore: EnablementStore {
    /// The folder that holds `state.json`.
    public let folder: URL
    private var file: URL { folder.appendingPathComponent("state.json") }

    /// Creates a store.
    /// - Parameter folder: the folder of the state file; created with mode `0700` on save.
    public init(folder: URL) {
        self.folder = folder
    }

    /// `~/Library/Application Support/bothy-computer-helper` of the account the helper runs as.
    public static func defaultFolder() -> URL {
        let support =
            FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
            ?? FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Application Support")
        return support.appendingPathComponent("bothy-computer-helper", isDirectory: true)
    }

    /// Reads the state.
    /// - Returns: the stored state, or `EnablementState.off` for a missing, unreadable or
    ///   malformed file, so a damaged file never switches anything on.
    public func load() -> EnablementState {
        guard let data = try? Data(contentsOf: file),
            let value = try? JSONValue.parse(data),
            value["v"]?.intValue == 1,
            let enabled = value["enabled"]?.boolValue,
            let generation = value["generation"]?.intValue, generation >= 0,
            generation <= Int64(Int.max)
        else { return .off }
        return EnablementState(enabled: enabled, generation: Int(generation))
    }

    /// Writes the state: a temporary file in the same folder, then a rename, so a crash leaves
    /// either the old or the new state.
    /// - Parameter state: the state to keep.
    /// - Throws: when the folder or the file cannot be written.
    public func save(_ state: EnablementState) throws {
        let manager = FileManager.default
        try manager.createDirectory(
            at: folder, withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700])
        // An existing folder keeps its mode on createDirectory; narrow it again.
        try manager.setAttributes([.posixPermissions: 0o700], ofItemAtPath: folder.path)
        let value: JSONValue = [
            "v": 1, "enabled": .bool(state.enabled), "generation": .int(Int64(state.generation)),
        ]
        let data = try value.serialized()
        let temporary = folder.appendingPathComponent(".state-\(UUID().uuidString).tmp")
        guard
            manager.createFile(
                atPath: temporary.path, contents: data, attributes: [.posixPermissions: 0o600])
        else {
            throw CocoaError(.fileWriteUnknown)
        }
        do {
            if rename(temporary.path, file.path) != 0 {
                throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
            }
        } catch {
            try? manager.removeItem(at: temporary)
            throw error
        }
    }
}
