import Foundation
import UserNotifications
#if os(iOS)
import UIKit
#endif

/// Manages push notification registration, permissions, and device token synchronization.
@MainActor
final class NotificationService: NSObject, ObservableObject {
    static let shared = NotificationService()

    @Published private(set) var isAuthorized = false
    @Published private(set) var deviceToken: String?

    private let serverStore: ServerStore
    private let keychain: KeychainService
    private let tokenStorageKey = "com.m.client.deviceToken"

    /// Closure called when a notification triggers deep link navigation.
    var onDeepLink: ((DeepLink) -> Void)?

    override init() {
        self.serverStore = ServerStore.shared
        self.keychain = KeychainService.shared
        super.init()

        // Restore saved token
        deviceToken = UserDefaults.standard.string(forKey: tokenStorageKey)
    }

    // MARK: - Permissions

    /// Requests notification authorization from the user.
    func requestAuthorization() async -> Bool {
        let center = UNUserNotificationCenter.current()

        do {
            let granted = try await center.requestAuthorization(options: [.alert, .badge, .sound])
            isAuthorized = granted

            if granted {
                await registerForRemoteNotifications()
            }

            return granted
        } catch {
            print("Notification authorization error: \(error)")
            return false
        }
    }

    /// Checks current authorization status.
    func checkAuthorizationStatus() async {
        let center = UNUserNotificationCenter.current()
        let settings = await center.notificationSettings()
        isAuthorized = settings.authorizationStatus == .authorized
    }

    // MARK: - Registration

    /// Registers for remote notifications with APNs.
    private func registerForRemoteNotifications() async {
        #if os(iOS)
        await MainActor.run {
            UIApplication.shared.registerForRemoteNotifications()
        }
        #endif
    }

    /// Called when APNs registration succeeds.
    func didRegisterForRemoteNotifications(deviceToken data: Data) {
        let token = data.map { String(format: "%02.2hhx", $0) }.joined()
        self.deviceToken = token

        // Persist token locally
        UserDefaults.standard.set(token, forKey: tokenStorageKey)

        // Register with all configured servers
        Task {
            await registerTokenWithAllServers(token: token)
        }
    }

    /// Called when APNs registration fails.
    func didFailToRegisterForRemoteNotifications(error: Error) {
        print("Failed to register for remote notifications: \(error)")
    }

    /// Registers the device token with all configured servers.
    func registerTokenWithAllServers(token: String) async {
        let servers = serverStore.servers

        await withTaskGroup(of: Void.self) { group in
            for server in servers {
                guard let apiKey = try? keychain.getAPIKey(for: server.id) else {
                    continue
                }

                group.addTask {
                    let client = APIClient(server: server, apiKey: apiKey)
                    do {
                        try await client.registerDevice(token: token)
                    } catch {
                        // Silently fail for individual servers
                        print("Failed to register device with \(server.name): \(error)")
                    }
                }
            }
        }
    }

    /// Unregisters the current device token from all servers.
    func unregisterFromAllServers() async {
        guard let token = deviceToken else { return }

        let servers = serverStore.servers

        await withTaskGroup(of: Void.self) { group in
            for server in servers {
                guard let apiKey = try? keychain.getAPIKey(for: server.id) else {
                    continue
                }

                group.addTask {
                    let client = APIClient(server: server, apiKey: apiKey)
                    do {
                        try await client.unregisterDevice(token: token)
                    } catch {
                        print("Failed to unregister device from \(server.name): \(error)")
                    }
                }
            }
        }

        // Clear local token
        self.deviceToken = nil
        UserDefaults.standard.removeObject(forKey: tokenStorageKey)
    }

    // MARK: - Notification Handling

    /// Handles a received notification payload and returns a deep link if applicable.
    func handleNotification(userInfo: [AnyHashable: Any], completionHandler: @escaping () -> Void) {
        defer { completionHandler() }

        guard let deepLink = parseDeepLink(from: userInfo) else {
            return
        }

        onDeepLink?(deepLink)
    }

    /// Parses notification payload into a deep link.
    private func parseDeepLink(from userInfo: [AnyHashable: Any]) -> DeepLink? {
        // Expected payload structure:
        // {
        //   "type": "approval",
        //   "approval_id": "...",
        //   "server_id": "...",
        //   "run_id": "..."
        // }

        guard let type = userInfo["type"] as? String else {
            return nil
        }

        switch type {
        case "approval":
            guard let approvalID = userInfo["approval_id"] as? String,
                  let serverID = userInfo["server_id"] as? String else {
                return nil
            }
            return .approval(serverID: serverID, approvalID: approvalID)

        case "run":
            guard let runID = userInfo["run_id"] as? String,
                  let serverID = userInfo["server_id"] as? String else {
                return nil
            }
            return .run(serverID: serverID, runID: runID)

        default:
            return nil
        }
    }
}

// MARK: - UNUserNotificationCenterDelegate

extension NotificationService: UNUserNotificationCenterDelegate {
    /// Called when a notification is received while the app is in the foreground.
    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        // Show banner and play sound even when app is in foreground
        completionHandler([.banner, .sound, .badge])
    }

    /// Called when the user taps on a notification.
    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        let userInfo = response.notification.request.content.userInfo

        Task { @MainActor in
            self.handleNotification(userInfo: userInfo, completionHandler: completionHandler)
        }
    }
}

// MARK: - Deep Link

/// Represents a deep link destination within the app.
enum DeepLink: Equatable {
    case approval(serverID: String, approvalID: String)
    case run(serverID: String, runID: String)

    /// Parses a URL into a deep link.
    /// Supported formats:
    /// - m://approvals/{serverID}/{approvalID}
    /// - m://runs/{serverID}/{runID}
    static func from(url: URL) -> DeepLink? {
        guard url.scheme == "m" else { return nil }

        let pathComponents = url.pathComponents.filter { $0 != "/" }

        guard pathComponents.count >= 3 else { return nil }

        let type = pathComponents[0]
        let serverID = pathComponents[1]
        let itemID = pathComponents[2]

        switch type {
        case "approvals":
            return .approval(serverID: serverID, approvalID: itemID)
        case "runs":
            return .run(serverID: serverID, runID: itemID)
        default:
            return nil
        }
    }
}
