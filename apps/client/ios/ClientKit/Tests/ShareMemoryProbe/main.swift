// SPDX-License-Identifier: Apache-2.0
//
// The memory probe of the ClientKitShare tests (brief M5-06 Design note 2: record the measured
// peak memory for a large photo and a large file). It loads one shared file the way the Share
// Extension does (an `NSItemProvider` of the file, `ShareItemLoader` over the live staging file
// system, so a HEIC goes through `JPEGConverter`) in a process of its own, and prints the
// process's memory figures before and after: the kernel's high-water marks of `phys_footprint`
// (what counts against an extension's limit) and of the resident size cover every moment of the
// load, and nothing else runs in the process.
//
// Usage: ShareMemoryProbe <file> <staging folder>
// Output: one line of key=value pairs; exit 1 when the item was not loaded, 2 on usage errors.

import ClientKitShare
import Darwin
import Foundation

/// The current and peak `phys_footprint` and the resident high-water mark, in bytes.
func figures() -> (footprint: UInt64, footprintPeak: UInt64, residentMax: UInt64) {
    var vm = task_vm_info_data_t()
    var vmCount = mach_msg_type_number_t(MemoryLayout<task_vm_info_data_t>.size / MemoryLayout<natural_t>.size)
    let vmResult = withUnsafeMutablePointer(to: &vm) {
        $0.withMemoryRebound(to: integer_t.self, capacity: Int(vmCount)) { task_info(mach_task_self_, task_flavor_t(TASK_VM_INFO), $0, &vmCount) }
    }
    var basic = mach_task_basic_info()
    var basicCount = mach_msg_type_number_t(MemoryLayout<mach_task_basic_info>.size / MemoryLayout<natural_t>.size)
    let basicResult = withUnsafeMutablePointer(to: &basic) {
        $0.withMemoryRebound(to: integer_t.self, capacity: Int(basicCount)) { task_info(mach_task_self_, task_flavor_t(MACH_TASK_BASIC_INFO), $0, &basicCount) }
    }
    return (
        vmResult == KERN_SUCCESS ? vm.phys_footprint : 0,
        vmResult == KERN_SUCCESS ? UInt64(vm.ledger_phys_footprint_peak) : 0,
        basicResult == KERN_SUCCESS ? basic.resident_size_max : 0
    )
}

let arguments = CommandLine.arguments
guard arguments.count == 3, let provider = NSItemProvider(contentsOf: URL(fileURLWithPath: arguments[1])) else {
    exit(2)
}
let folder = URL(fileURLWithPath: arguments[2], isDirectory: true)
let before = figures()
let loader = ShareItemLoader(fileSystem: LiveStagingFileSystem(makeFolder: {
    try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    return folder
}))
let result = await loader.load([ItemProviderAttachment(provider)])
let after = figures()
guard let item = result.items.first, result.failures == 0 else { exit(1) }
print(
    "name=\(item.name) mime=\(item.mimeType) size=\(item.sizeBytes) footprint_before=\(before.footprint) "
        + "footprint_peak=\(after.footprintPeak) resident_max_before=\(before.residentMax) resident_max=\(after.residentMax)")
