import UIKit

/// Registers the device for push notifications and stores its token with
/// POST /api/devices.
final class PushRegistration {
    static let shared = PushRegistration()

    func register(_ application: UIApplication) {
        application.registerForRemoteNotifications()
    }

    func didRegister(deviceToken: Data) {
        var request = URLRequest(url: AppInfo.apiBaseURL.appendingPathComponent("api/devices"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let token = deviceToken.map { String(format: "%02x", $0) }.joined()
        let body: [String: Any] = ["userId": UserDefaults.standard.integer(forKey: "userId"), "platform": "ios", "token": token]
        request.httpBody = try? JSONSerialization.data(withJSONObject: body)
        URLSession.shared.dataTask(with: request).resume()
    }
}
