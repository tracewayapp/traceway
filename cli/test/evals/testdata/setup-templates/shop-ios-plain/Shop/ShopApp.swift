import SwiftUI

@main
struct ShopApp: App {
    @State private var cart = Cart()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(cart)
        }
    }
}
