import Foundation

/// Build and device facts.
enum AppInfo {
    static let version = Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "0"

    static let apiBaseURL = URL(string: Bundle.main.object(forInfoDictionaryKey: "API_BASE_URL") as? String ?? "https://api.shop.example")!

    /// "iPhone16,2 / iOS 18.0.1"
    static var deviceDescription: String {
        var info = utsname()
        uname(&info)
        let model = withUnsafeBytes(of: &info.machine) { String(decoding: $0.prefix(while: { $0 != 0 }), as: UTF8.self) }
        let os = ProcessInfo.processInfo.operatingSystemVersion
        let version = os.patchVersion > 0 ? "\(os.majorVersion).\(os.minorVersion).\(os.patchVersion)" : "\(os.majorVersion).\(os.minorVersion)"
        return "\(model) / iOS \(version)"
    }
}

/// Values injected from the build settings into Info.plist.
enum Secrets {
    static let tracewayConnectionString = Bundle.main.object(forInfoDictionaryKey: "TRACEWAY_CONNECTION_STRING") as? String ?? ""

    /// The connection string split into its token and report URL.
    static var tracewayConnection: (String, URL) {
        let parts = tracewayConnectionString.split(separator: "@", maxSplits: 1).map(String.init)
        return (parts.first ?? "", URL(string: parts.count > 1 ? parts[1] : "https://traceway.invalid/api/report")!)
    }
}
