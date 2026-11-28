import SwiftUI

struct SavedCard {
    let holder: String
    let brand: String
    let last4: String
    let expiry: String
}

struct CheckoutView: View {
    @Environment(Cart.self) private var cart
    @State private var promoCode = ""
    private let card = SavedCard(holder: "Ada Lovelace", brand: "Visa", last4: "4242", expiry: "04/29")

    var body: some View {
        Form {
            Section("Pay with") {
                VStack(alignment: .leading, spacing: 4) {
                    Text(card.holder)
                    Text("\(card.brand) •••• \(card.last4), expires \(card.expiry)")
                        .foregroundStyle(.secondary)
                }
            }
            Section("Promo code") {
                TextField("Code", text: $promoCode)
            }
            Section {
                Button("Pay \((Double(cart.totalCents) / 100).formatted(.currency(code: "EUR")))") {}
                    .disabled(cart.items.isEmpty)
            }
        }
        .navigationTitle("Checkout")
    }
}
