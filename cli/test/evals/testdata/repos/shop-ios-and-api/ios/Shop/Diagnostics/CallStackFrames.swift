import Foundation
import MetricKit

/// The crashed thread's frames from an MXCallStackTree, innermost first.
enum CallStackFrames {
    struct Frame {
        let binaryUUID: String
        let offsetIntoBinaryTextSegment: UInt64
        let binaryName: String
    }

    static func crashedThread(_ tree: MXCallStackTree) -> [Frame] {
        guard
            let root = try? JSONSerialization.jsonObject(with: tree.jsonRepresentation()) as? [String: Any],
            let stacks = root["callStacks"] as? [[String: Any]],
            let crashed = stacks.first(where: { $0["threadAttributed"] as? Bool == true }),
            var node = (crashed["callStackRootFrames"] as? [[String: Any]])?.first
        else { return [] }
        var frames: [Frame] = []
        while true {
            frames.append(Frame(
                binaryUUID: (node["binaryUUID"] as? String ?? "").replacingOccurrences(of: "-", with: ""),
                offsetIntoBinaryTextSegment: (node["offsetIntoBinaryTextSegment"] as? NSNumber)?.uint64Value ?? 0,
                binaryName: node["binaryName"] as? String ?? "?"
            ))
            guard let next = (node["subFrames"] as? [[String: Any]])?.first else { break }
            node = next
        }
        return frames
    }
}
