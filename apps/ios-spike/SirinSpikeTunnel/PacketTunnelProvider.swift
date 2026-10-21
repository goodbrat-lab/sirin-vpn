import Foundation
import NetworkExtension
import os.log
#if canImport(Mobile)
import Mobile
#endif

class PacketTunnelProvider: NEPacketTunnelProvider {
    private let logger = OSLog(subsystem: "ru.guseynovia.sirin.spike", category: "PacketTunnel")
    private var isRunning: Bool = false
    private let packetQueue = DispatchQueue(label: "ru.guseynovia.sirin.spike.packets", qos: .userInitiated)
    
    // Буфер для накопления пакетов в batch
    private var pendingPackets: [Data] = []
    private let packetSemaphore = DispatchSemaphore(value: 0)
    private let bufferLock = NSLock()

    override func startTunnel(options: [String : NSObject]?, completionHandler: @escaping (Error?) -> Void) {
        os_log("PacketTunnelProvider: Запуск туннеля", log: logger, type: .info)
        
        // 1. Извлечение UUID
        var targetUUID = options?["uuid"] as? String
        if targetUUID == nil || targetUUID?.isEmpty == true {
            let sharedDefaults = UserDefaults(suiteName: "group.ru.guseynovia.sirin.spike")
            targetUUID = sharedDefaults?.string(forKey: "sirin_uuid")
        }
        
        guard let uuid = targetUUID, !uuid.isEmpty else {
            let err = NSError(domain: "SirinSpike", code: 1, userInfo: [NSLocalizedDescriptionKey: "UUID не найден"])
            os_log("PacketTunnelProvider: Ошибка: UUID отсутствует", log: logger, type: .error)
            completionHandler(err)
            return
        }

        // 2. Настройка NEPacketTunnelNetworkSettings (IPv4, MTU 1280, DNS)
        let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "crp.guseynovia.ru")
        let tunnelIP = ["10", "77", "77", "77"].joined(separator: ".")
        let tunnelMask = ["255", "255", "255", "0"].joined(separator: ".")
        let dnsServer = ["1", "1", "1", "1"].joined(separator: ".")
        let ipv4Settings = NEIPv4Settings(addresses: [tunnelIP], subnetMasks: [tunnelMask])
        ipv4Settings.includedRoutes = [NEIPv4Route.default()]
        settings.ipv4Settings = ipv4Settings
        settings.mtu = 1280
        settings.dnsSettings = NEDNSSettings(servers: [dnsServer])

        setTunnelNetworkSettings(settings) { [weak self] error in
            guard let self = self else { return }
            if let error = error {
                os_log("PacketTunnelProvider: Ошибка setTunnelNetworkSettings: %{public}@", log: self.logger, type: .error, error.localizedDescription)
                completionHandler(error)
                return
            }

            self.isRunning = true
            self.startPacketReading()

            #if canImport(Mobile)
            DispatchQueue.global(qos: .userInitiated).async {
                do {
                    try MobileSirinStart(uuid, self, self)
                    os_log("PacketTunnelProvider: MobileSirinStart успешно завершен", log: self.logger, type: .info)
                } catch {
                    os_log("PacketTunnelProvider: Ошибка MobileSirinStart: %{public}@", log: self.logger, type: .error, error.localizedDescription)
                }
            }
            #else
            os_log("PacketTunnelProvider: Запуск в Mock-режиме (без Mobile.xcframework)", log: self.logger, type: .info)
            #endif

            completionHandler(nil)
        }
    }

    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        os_log("PacketTunnelProvider: Остановка туннеля (reason: %ld)", log: logger, type: .info, reason.rawValue)
        isRunning = false
        packetSemaphore.signal()

        #if canImport(Mobile)
        MobileSirinStop()
        #endif

        completionHandler()
    }

    private func startPacketReading() {
        packetQueue.async { [weak self] in
            guard let self = self else { return }
            while self.isRunning {
                let group = DispatchGroup()
                group.enter()

                self.packetFlow.readPackets { [weak self] packets, protocols in
                    guard let self = self, self.isRunning else {
                        group.leave()
                        return
                    }

                    if !packets.isEmpty {
                        self.bufferLock.lock()
                        self.pendingPackets.append(contentsOf: packets)
                        self.bufferLock.unlock()
                        self.packetSemaphore.signal()
                    }
                    group.leave()
                }

                _ = group.wait(timeout: .now() + 1.0)
            }
        }
    }
}

// MARK: - Mobile PacketFlow & Logger Protocol Implementations

#if canImport(Mobile)
extension PacketTunnelProvider: MobilePacketFlowProtocol, MobileLoggerProtocol {
    func readBatch() -> Data? {
        while isRunning {
            bufferLock.lock()
            if !pendingPackets.isEmpty {
                // Сборка пачки в формате [2 байта длины BE][пакет]
                var batch = Data()
                for pkt in pendingPackets {
                    var len = UInt16(pkt.count).bigEndian
                    batch.append(Data(bytes: &len, count: 2))
                    batch.append(pkt)
                }
                pendingPackets.removeAll()
                bufferLock.unlock()
                return batch
            }
            bufferLock.unlock()

            _ = packetSemaphore.wait(timeout: .now() + 0.1)
        }
        return nil
    }

    func writeBatch(_ p: Data?) -> Bool {
        guard let data = p, !data.isEmpty, isRunning else { return false }
        return packetFlow.writePackets([data], withProtocols: [AF_INET as NSNumber])
    }

    func close() {
        isRunning = false
        packetSemaphore.signal()
    }

    func log(_ line: String?) {
        if let line = line {
            os_log("SirinEngine: %{public}@", log: logger, type: .default, line)
        }
    }
}
#endif
