import Foundation
import Traceway

/// Feature flags and the minimum supported version, from GET /api/config.
final class RemoteConfig {
    static let shared = RemoteConfig()

    private(set) var flags: [String: Bool] = [:]
    private(set) var minimumVersion: String?

    private let client = ConfigClient(baseURL: AppInfo.apiBaseURL)

    /// Loads the config before the first screen is built, so the home screen
    /// never renders with stale flags. Falls back to the bundled defaults if
    /// the request fails.
    func loadAtLaunch() {
        flags = Self.bundledDefaults
        do {
            let config = try client.fetchBlocking(timeout: 30)
            flags = config.flags
            minimumVersion = config.minimumVersion
        } catch {
            Traceway.capture(error)
        }
    }

    static let bundledDefaults: [String: Bool] = ["new_home": false, "apple_pay": true]
}
