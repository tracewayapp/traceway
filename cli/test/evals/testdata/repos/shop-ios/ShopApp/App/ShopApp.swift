import SwiftUI
import Traceway

@main
struct ShopApp: App {
    init() {
        Traceway.start(connectionString: Secrets.tracewayConnectionString, options: TracewayOptions(version: Bundle.main.shortVersion))
        Task { await RemoteConfig.shared.refresh() }
    }

    var body: some Scene {
        WindowGroup {
            RootView()
        }
    }
}

extension Bundle {
    var shortVersion: String { infoDictionary?["CFBundleShortVersionString"] as? String ?? "0" }
}
