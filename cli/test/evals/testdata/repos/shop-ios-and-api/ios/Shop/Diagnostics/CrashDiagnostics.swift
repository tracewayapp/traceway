import Foundation
import MetricKit
import Traceway

/// Forwards MetricKit crash diagnostics (watchdog terminations included,
/// which no in-process handler can catch) to Traceway on the next launch, in
/// the Traceway iOS frame format so the uploaded dSYM symbolicates them.
final class CrashDiagnostics: NSObject, MXMetricManagerSubscriber {
    static let shared = CrashDiagnostics()

    func start() { MXMetricManager.shared.add(self) }

    func didReceive(_ payloads: [MXDiagnosticPayload]) {
        for payload in payloads {
            for crash in payload.crashDiagnostics ?? [] {
                TracewayReport.send(stackTrace: Self.render(crash), recordedAt: payload.timeStampEnd,
                                    attributes: ["source": "metrickit", "phase": "launch"])
            }
        }
    }

    /// "Watchdog termination: EXC_CRASH (SIGKILL), FRONTBOARD 0x8badf00d ..."
    /// then the *** separator, the os/arch line and one "#NN <uuid> 0x<offset>
    /// <image>" line per frame of the crashed thread.
    static func render(_ crash: MXCrashDiagnostic) -> String {
        var lines = [Self.preamble(crash), String(repeating: "*** ", count: 16).trimmingCharacters(in: .whitespaces), "os: ios arch: arm64"]
        for (i, frame) in CallStackFrames.crashedThread(crash.callStackTree).enumerated() {
            lines.append(String(format: "#%02d %@ 0x%016llx %@", i, frame.binaryUUID, frame.offsetIntoBinaryTextSegment, frame.binaryName))
        }
        return lines.joined(separator: "\n")
    }

    private static func preamble(_ crash: MXCrashDiagnostic) -> String {
        let reason = crash.terminationReason ?? "unknown"
        if reason.contains("0x8badf00d") {
            return "Watchdog termination: EXC_CRASH (SIGKILL), \(reason)"
        }
        return "Fatal Signal \(crash.signal ?? 0): \(reason)"
    }
}
