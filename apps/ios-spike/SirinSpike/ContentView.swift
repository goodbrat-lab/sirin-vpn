import SwiftUI

struct ContentView: View {
    @EnvironmentObject var vpnManager: VPNManager
    @State private var inputUUID: String = ""

    var body: some View {
        NavigationView {
            VStack(spacing: 20) {
                Text("Sirin iOS Spike")
                    .font(.largeTitle.bold())

                Text("Проверка гипотезы: gomobile + PacketTunnelProvider (< 50 MiB)")
                    .font(.subheadline)
                    .foregroundColor(.secondary)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal)

                VStack(alignment: .leading, spacing: 8) {
                    Text("Ключ доступа (UUID):")
                        .font(.headline)
                    TextField("Вставьте ключ / UUID", text: $inputUUID)
                        .textFieldStyle(RoundedBorderTextFieldStyle())
                        .autocapitalization(.none)
                        .disableAutocorrection(true)
                }
                .padding(.horizontal)

                HStack {
                    Circle()
                        .fill(statusColor)
                        .frame(width: 12, height: 12)
                    Text("Статус: \(vpnManager.statusText)")
                        .font(.headline)
                }

                Button(action: {
                    if vpnManager.isConnected {
                        vpnManager.disconnect()
                    } else {
                        vpnManager.connect(uuid: inputUUID)
                    }
                }) {
                    Text(vpnManager.isConnected ? "Отключить" : "Подключить")
                        .font(.title3.bold())
                        .foregroundColor(.white)
                        .frame(maxWidth: .infinity)
                        .padding()
                        .background(vpnManager.isConnected ? Color.red : Color.blue)
                        .cornerRadius(12)
                }
                .padding(.horizontal)
                .disabled(inputUUID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)

                VStack(alignment: .leading, spacing: 6) {
                    Text("Логи расширения:")
                        .font(.caption.bold())
                    ScrollView {
                        Text(vpnManager.logs.isEmpty ? "Логи отсутствуют" : vpnManager.logs)
                            .font(.system(.caption2, design: .monospaced))
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .padding(8)
                    }
                    .frame(height: 150)
                    .background(Color(.secondarySystemBackground))
                    .cornerRadius(8)
                }
                .padding(.horizontal)

                Spacer()
            }
            .padding(.top)
            .navigationBarHidden(true)
            .onAppear {
                inputUUID = vpnManager.savedUUID
            }
        }
    }

    private var statusColor: Color {
        switch vpnManager.statusText {
        case "Подключено":
            return .green
        case "Подключение...":
            return .orange
        case "Ошибка":
            return .red
        default:
            return .gray
        }
    }
}
