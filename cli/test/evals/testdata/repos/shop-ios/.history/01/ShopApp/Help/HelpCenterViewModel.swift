import Foundation
import Observation

@Observable
final class HelpCenterViewModel {
    var articles: [HelpArticle] = []
    var isLoading = false

    private let config: RemoteConfig
    private let support: SupportClient

    init(config: RemoteConfig = .shared, support: SupportClient = .live) {
        self.config = config
        self.support = support
    }

    var supportEmail: String {
        config.string(for: .supportEmail) ?? "help@shop.example"
    }

    func load() async {
        isLoading = true
        defer { isLoading = false }
        articles = (try? await support.topArticles()) ?? []
    }
}
