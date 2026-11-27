import SwiftUI
import Traceway

@main
struct ShopApp: App {
    @State private var cart = Cart()

    init() {
        // TracewayToken and TracewayURL are $(TRACEWAY_TOKEN) and $(TRACEWAY_URL) in Info.plist,
        // set in the gitignored Config/Secrets.xcconfig.
        Traceway.initialize(
            token: Bundle.main.object(forInfoDictionaryKey: "TracewayToken") as? String ?? "",
            baseUrl: Bundle.main.object(forInfoDictionaryKey: "TracewayURL") as? String ?? "",
            options: TracewayOptions(screenCapture: true)
        )
    }

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(cart)
        }
    }
}
