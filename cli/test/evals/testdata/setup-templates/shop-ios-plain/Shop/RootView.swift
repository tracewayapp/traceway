import SwiftUI

struct RootView: View {
    var body: some View {
        TabView {
            NavigationStack { CatalogView() }
                .tabItem { Label("Shop", systemImage: "bag") }
            NavigationStack { CheckoutView() }
                .tabItem { Label("Checkout", systemImage: "creditcard") }
        }
    }
}
