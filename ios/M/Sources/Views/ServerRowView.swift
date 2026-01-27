import SwiftUI

/// Row displaying a single server with status indicator.
struct ServerRowView: View {
    let server: MServer
    let status: ConnectionStatus
    let lastActivity: Date?

    var body: some View {
        HStack(spacing: 12) {
            statusIndicator

            VStack(alignment: .leading, spacing: 2) {
                Text(server.name)
                    .font(.body)
                    .foregroundStyle(.primary)

                Text(server.url.host ?? server.url.absoluteString)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }

            Spacer()

            if let lastActivity {
                Text(lastActivity, style: .relative)
                    .font(.caption)
                    .foregroundStyle(.tertiary)
            }
        }
        .padding(.vertical, 4)
    }

    @ViewBuilder
    private var statusIndicator: some View {
        switch status {
        case .unknown:
            Circle()
                .fill(.gray.opacity(0.3))
                .frame(width: 8, height: 8)
        case .connecting:
            ProgressView()
                .scaleEffect(0.6)
                .frame(width: 8, height: 8)
        case .connected:
            Circle()
                .fill(.green)
                .frame(width: 8, height: 8)
        case .error:
            Circle()
                .fill(.red)
                .frame(width: 8, height: 8)
        }
    }
}
