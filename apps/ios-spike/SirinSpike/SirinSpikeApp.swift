import SwiftUI

@main
struct SirinSpikeApp: App {
    @StateObject private var vpnManager = VPNManager.shared

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(vpnManager)
                .onOpenURL { url in
                    vpnManager.handleDeepLink(url)
                }
        }
    }
}
