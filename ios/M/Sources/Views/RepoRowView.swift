import SwiftUI

/// Row displaying a single repo with active run badge and last run status.
struct RepoRowView: View {
    let repo: Repo
    let hasActiveRun: Bool
    let lastRunState: RunState?

    var body: some View {
        HStack(spacing: 12) {
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 6) {
                    Text(repo.name)
                        .font(.body)
                        .foregroundStyle(.primary)

                    if hasActiveRun {
                        Circle()
                            .fill(.orange)
                            .frame(width: 8, height: 8)
                    }
                }

                if let gitURL = repo.gitURL {
                    Text(gitURL)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }

            Spacer()

            lastRunStatusIcon
        }
        .padding(.vertical, 4)
    }

    @ViewBuilder
    private var lastRunStatusIcon: some View {
        switch lastRunState {
        case .completed:
            Image(systemName: "checkmark")
                .foregroundStyle(.green)
                .font(.system(size: 14, weight: .semibold))
        case .failed:
            Image(systemName: "xmark")
                .foregroundStyle(.red)
                .font(.system(size: 14, weight: .semibold))
        case .cancelled:
            Image(systemName: "minus")
                .foregroundStyle(.secondary)
                .font(.system(size: 14, weight: .semibold))
        case .running, .waitingApproval, .waitingInput:
            // Active states shown via badge, not status icon
            EmptyView()
        case nil:
            Image(systemName: "minus")
                .foregroundStyle(.tertiary)
                .font(.system(size: 14, weight: .semibold))
        }
    }
}
