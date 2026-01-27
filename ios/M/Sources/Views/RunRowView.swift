import SwiftUI

/// Row displaying a single run with status icon, prompt preview, and timestamp.
struct RunRowView: View {
    let run: Run

    var body: some View {
        HStack(spacing: 12) {
            statusIcon

            VStack(alignment: .leading, spacing: 2) {
                Text(run.prompt)
                    .font(.body)
                    .foregroundStyle(.primary)
                    .lineLimit(1)
                    .truncationMode(.tail)

                Text(run.createdAt, style: .relative)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Spacer()
        }
        .padding(.vertical, 4)
    }

    @ViewBuilder
    private var statusIcon: some View {
        switch run.state {
        case .running:
            ProgressView()
                .scaleEffect(0.8)
                .frame(width: 20, height: 20)
        case .waitingApproval, .waitingInput:
            Circle()
                .fill(.orange)
                .frame(width: 10, height: 10)
        case .completed:
            Image(systemName: "checkmark.circle.fill")
                .foregroundStyle(.green)
                .font(.system(size: 16))
        case .failed:
            Image(systemName: "xmark.circle.fill")
                .foregroundStyle(.red)
                .font(.system(size: 16))
        case .cancelled:
            Image(systemName: "minus.circle.fill")
                .foregroundStyle(.secondary)
                .font(.system(size: 16))
        }
    }
}
