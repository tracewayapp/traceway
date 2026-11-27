import Foundation
import Observation

@Observable
final class Cart {
    private(set) var items: [Product] = []

    var totalCents: Int { items.reduce(0) { $0 + $1.priceCents } }

    func add(_ product: Product) {
        items.append(product)
    }
}
