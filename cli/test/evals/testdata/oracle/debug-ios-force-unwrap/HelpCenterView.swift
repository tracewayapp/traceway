import SafariServices
import SwiftUI

struct HelpCenterView: View {
    @State private var model = HelpCenterViewModel()
    @State private var chatOpen = false

    var body: some View {
        List {
            Section("Popular questions") {
                ForEach(model.articles) { article in
                    NavigationLink(article.title) { HelpArticleView(article: article) }
                }
            }
            Section("Contact us") {
                if model.chatURL != nil {
                    Button("Chat with us") { chatOpen = true }
                }
                Link("Email \(model.supportEmail)", destination: URL(string: "mailto:\(model.supportEmail)")!)
            }
        }
        .navigationTitle("Help")
        .overlay { if model.isLoading { ProgressView() } }
        .task { await model.load() }
        .sheet(isPresented: $chatOpen) {
            if let url = model.chatURL {
                SafariView(url: url)
                    .ignoresSafeArea()
            }
        }
    }
}

struct SafariView: UIViewControllerRepresentable {
    let url: URL
    func makeUIViewController(context: Context) -> SFSafariViewController { SFSafariViewController(url: url) }
    func updateUIViewController(_ controller: SFSafariViewController, context: Context) {}
}
