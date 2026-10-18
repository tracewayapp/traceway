import UIKit
import Traceway

@main
class AppDelegate: UIResponder, UIApplicationDelegate {
    func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    ) -> Bool {
        Traceway.start(
            connectionString: Secrets.tracewayConnectionString,
            options: TracewayOptions(version: AppInfo.version)
        )
        CrashDiagnostics.shared.start()

        // Flags decide which home screen and payment methods to show.
        RemoteConfig.shared.loadAtLaunch()
        PushRegistration.shared.register(application)
        return true
    }
}
