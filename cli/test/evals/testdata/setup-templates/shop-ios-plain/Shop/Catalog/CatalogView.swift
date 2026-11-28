import SwiftUI

struct CatalogView: View {
    @Environment(Cart.self) private var cart
    @State private var products: [Product] = []
    @State private var failed = false

    var body: some View {
        List(products) { product in
            HStack {
                Text(product.title)
                Spacer()
                Button("Add") { cart.add(product) }
            }
        }
        .overlay {
            if failed {
                ContentUnavailableView("Could not load the catalog", systemImage: "wifi.slash")
            }
        }
        .navigationTitle("Shop")
        .task {
            do {
                products = try await APIClient.shared.products()
            } catch {
                failed = true
            }
        }
    }
}
