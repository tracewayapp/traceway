import Foundation

struct ConfigPayload: Decodable {
    let flags: [String: Bool]
    let minimumVersion: String?
}

enum ConfigError: Error { case timedOut, badResponse }

final class ConfigClient {
    private let baseURL: URL
    private let session: URLSession

    init(baseURL: URL, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.session = session
    }

    /// GET /api/config, waiting for the answer on the calling thread.
    func fetchBlocking(timeout: TimeInterval) throws -> ConfigPayload {
        var request = URLRequest(url: baseURL.appendingPathComponent("api/config"))
        request.timeoutInterval = timeout
        let semaphore = DispatchSemaphore(value: 0)
        var result: Result<ConfigPayload, Error> = .failure(ConfigError.timedOut)
        session.dataTask(with: request) { data, response, error in
            defer { semaphore.signal() }
            guard let data, (response as? HTTPURLResponse)?.statusCode == 200 else {
                result = .failure(error ?? ConfigError.badResponse)
                return
            }
            result = Result { try JSONDecoder().decode(ConfigPayload.self, from: data) }
        }.resume()
        _ = semaphore.wait(timeout: .now() + timeout)
        return try result.get()
    }
}
