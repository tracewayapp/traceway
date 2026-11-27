import Foundation

struct Product: Decodable, Identifiable, Hashable {
    let id: String
    let title: String
    let priceCents: Int
}

struct APIClient {
    static let shared = APIClient()

    /// The APIBaseURL Info.plist key, set from API_BASE_URL in Config/Shop.xcconfig.
    let baseURL: URL = {
        let value = Bundle.main.object(forInfoDictionaryKey: "APIBaseURL") as? String
        return value.flatMap(URL.init(string:)) ?? URL(string: "https://api.shop.example")!
    }()

    func products() async throws -> [Product] {
        let (data, response) = try await URLSession.shared.data(from: baseURL.appending(path: "v1/products"))
        guard (response as? HTTPURLResponse)?.statusCode == 200 else {
            throw URLError(.badServerResponse)
        }
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        return try decoder.decode([Product].self, from: data)
    }
}
