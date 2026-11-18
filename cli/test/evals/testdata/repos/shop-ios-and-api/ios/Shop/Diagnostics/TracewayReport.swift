import Foundation

/// Posts one exception to Traceway's /api/report with the app's connection
/// string, for reports the SDK cannot capture itself (MetricKit diagnostics).
enum TracewayReport {
    static func send(stackTrace: String, recordedAt: Date, attributes: [String: String]) {
        let (token, url) = Secrets.tracewayConnection
        let body: [String: Any] = [
            "appVersion": AppInfo.version,
            "serverName": AppInfo.deviceDescription,
            "collectionFrames": [[
                "stackTraces": [[
                    "stackTrace": stackTrace,
                    "recordedAt": ISO8601DateFormatter().string(from: recordedAt),
                    "attributes": attributes,
                    "isMessage": false,
                ]],
            ]],
        ]
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try? JSONSerialization.data(withJSONObject: body)
        URLSession.shared.dataTask(with: request).resume()
    }
}
