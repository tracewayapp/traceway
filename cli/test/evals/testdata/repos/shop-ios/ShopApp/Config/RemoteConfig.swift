import Foundation

/// Values from the config service, fetched at launch.
/// A key that is not set for the user's audience is absent from the payload.
final class RemoteConfig {
    static let shared = RemoteConfig()

    enum Key: String {
        case supportChatURL = "support_chat_url"
        case supportEmail = "support_email"
        case freeShippingThresholdCents = "free_shipping_threshold_cents"
    }

    private var values: [String: String] = [:]
    private let endpoint = URL(string: "https://config.shop.example/v1/remote-config")!

    func string(for key: Key) -> String? {
        values[key.rawValue]
    }

    func url(for key: Key) -> URL? {
        string(for: key).flatMap(URL.init(string:))
    }

    func refresh() async {
        var request = URLRequest(url: endpoint.appending(queryItems: [
            URLQueryItem(name: "locale", value: Locale.current.identifier),
            URLQueryItem(name: "app_version", value: Bundle.main.shortVersion),
        ]))
        request.timeoutInterval = 5
        guard let (data, _) = try? await URLSession.shared.data(for: request),
              let payload = try? JSONDecoder().decode([String: String].self, from: data) else { return }
        values = payload
    }
}
