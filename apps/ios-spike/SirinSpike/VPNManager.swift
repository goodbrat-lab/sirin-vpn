import Foundation
import NetworkExtension

class VPNManager: ObservableObject {
    static let shared = VPNManager()
    static let appGroupID = "group.ru.guseynovia.sirin.spike"
    static let tunnelBundleID = "ru.guseynovia.sirin.spike.tunnel"

    @Published var isConnected: Bool = false
    @Published var statusText: String = "Отключено"
    @Published var logs: String = ""

    private var providerManager: NETunnelProviderManager?
    private let sharedDefaults = UserDefaults(suiteName: appGroupID)

    var savedUUID: String {
        get { sharedDefaults?.string(forKey: "sirin_uuid") ?? "" }
        set { sharedDefaults?.set(newValue, forKey: "sirin_uuid") }
    }

    init() {
        loadAndCreateProviderManager()
        observeStatus()
    }

    func loadAndCreateProviderManager() {
        NETunnelProviderManager.loadAllFromPreferences { [weak self] managers, error in
            guard let self = self else { return }
            if let existing = managers?.first {
                self.providerManager = existing
            } else {
                let manager = NETunnelProviderManager()
                let proto = NETunnelProviderProtocol()
                proto.providerBundleIdentifier = Self.tunnelBundleID
                proto.serverAddress = "crp.guseynovia.ru"
                manager.protocolConfiguration = proto
                manager.localizedDescription = "Sirin VPN Spike"
                manager.isEnabled = true
                manager.saveToPreferences { saveError in
                    if saveError == nil {
                        self.providerManager = manager
                    }
                }
            }
            self.updateStatus()
        }
    }

    func connect(uuid: String) {
        let trimmed = uuid.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return }
        savedUUID = trimmed
        sharedDefaults?.set(trimmed, forKey: "sirin_uuid")
        sharedDefaults?.synchronize()

        guard let manager = providerManager else {
            loadAndCreateProviderManager()
            return
        }

        manager.isEnabled = true
        manager.saveToPreferences { [weak self] error in
            guard let self = self, error == nil else {
                self?.statusText = "Ошибка сохранения настроек"
                return
            }

            do {
                try (manager.connection as? NETunnelProviderSession)?.startTunnel(options: ["uuid": trimmed as NSObject])
                self.statusText = "Подключение..."
            } catch {
                self.statusText = "Ошибка запуска: \(error.localizedDescription)"
            }
        }
    }

    func disconnect() {
        (providerManager?.connection as? NETunnelProviderSession)?.stopTunnel()
        statusText = "Отключение..."
    }

    func handleDeepLink(_ url: URL) {
        guard url.scheme == "sirin", url.host == "key" else { return }
        if let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
           let uuid = components.queryItems?.first(where: { $0.name == "uuid" })?.value {
            self.savedUUID = uuid
            self.connect(uuid: uuid)
        }
    }

    private func observeStatus() {
        NotificationCenter.default.addObserver(
            forName: .NEVPNStatusDidChange,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            self?.updateStatus()
        }
    }

    private func updateStatus() {
        guard let connection = providerManager?.connection else {
            statusText = "Не настроено"
            isConnected = false
            return
        }

        switch connection.status {
        case .connected:
            statusText = "Подключено"
            isConnected = true
        case .connecting:
            statusText = "Подключение..."
            isConnected = false
        case .disconnecting:
            statusText = "Отключение..."
            isConnected = false
        case .disconnected:
            statusText = "Отключено"
            isConnected = false
        case .invalid:
            statusText = "Недействительно"
            isConnected = false
        case .reasserting:
            statusText = "Переподключение..."
            isConnected = true
        @unknown default:
            statusText = "Неизвестно"
            isConnected = false
        }
    }
}
