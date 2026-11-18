import SwiftUI

struct HelpCenterView: View {
    @State private var model = HelpCenterViewModel()

    var body: some View {
        List {
            Section("Popular questions") {
                ForEach(model.articles) { article in
                    NavigationLink(article.title) { HelpArticleView(article: article) }
                }
            }
            Section("Contact us") {
                Link("Email \(model.supportEmail)", destination: URL(string: "mailto:\(model.supportEmail)")!)
            }
        }
        .navigationTitle("Help")
        .overlay { if model.isLoading { ProgressView() } }
        .task { await model.load() }
    }
}
