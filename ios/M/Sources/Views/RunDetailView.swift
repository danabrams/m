import SwiftUI

/// Screen showing details of a single run with event feed and controls.
struct RunDetailView: View {
    let run: Run
    let apiClient: APIClient

    @State private var currentRun: Run
    @State private var events: [RunEvent] = []
    @State private var isLoading = true
    @State private var error: MError?
    @State private var webSocketClient: WebSocketClient?
    @State private var isAutoScrollEnabled = true
    @State private var pendingApproval: Approval?
    @State private var pendingInputQuestion: String?
    @State private var showingApproval = false
    @State private var showingInput = false
    @State private var showingRetrySheet = false
    @State private var retryPrompt = ""

    init(run: Run, apiClient: APIClient) {
        self.run = run
        self.apiClient = apiClient
        self._currentRun = State(initialValue: run)
    }

    var body: some View {
        VStack(spacing: 0) {
            statusBar

            if let approval = pendingApproval, currentRun.state == .waitingApproval {
                pendingActionCard(approval: approval)
            } else if let question = pendingInputQuestion, currentRun.state == .waitingInput {
                pendingInputCard(question: question)
            }

            eventFeed

            footerButtons
        }
        .navigationTitle(run.prompt.prefix(20) + (run.prompt.count > 20 ? "..." : ""))
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Menu {
                    if currentRun.state == .running {
                        Button(role: .destructive) {
                            cancelRun()
                        } label: {
                            Label("Cancel Run", systemImage: "xmark.circle")
                        }
                    }
                    Button {
                        Task { await refreshRun() }
                    } label: {
                        Label("Refresh", systemImage: "arrow.clockwise")
                    }
                } label: {
                    Image(systemName: "ellipsis.circle")
                }
            }
        }
        .task {
            await loadInitialData()
            startWebSocket()
        }
        .onDisappear {
            webSocketClient?.disconnect()
        }
        .alert("Error", isPresented: .constant(error != nil)) {
            Button("OK") { error = nil }
        } message: {
            if let error { Text(error.localizedDescription) }
        }
        .sheet(isPresented: $showingApproval) {
            if let approval = pendingApproval {
                ApprovalDetailView(
                    pending: PendingApproval(
                        approval: approval,
                        server: MServer(name: "Server", url: apiClient.serverURL),
                        apiClient: apiClient
                    ),
                    onResolved: {
                        pendingApproval = nil
                        Task { await refreshRun() }
                    }
                )
            }
        }
        .sheet(isPresented: $showingInput) {
            if let question = pendingInputQuestion {
                InputPromptView(
                    apiClient: apiClient,
                    runID: run.id,
                    question: question,
                    onSent: {
                        pendingInputQuestion = nil
                        Task { await refreshRun() }
                    }
                )
            }
        }
        .sheet(isPresented: $showingRetrySheet) {
            retrySheet
        }
    }

    // MARK: - Status Bar

    private var statusBar: some View {
        HStack(spacing: 12) {
            statusIcon
            VStack(alignment: .leading, spacing: 2) {
                Text(statusText)
                    .font(.system(.subheadline, weight: .semibold))
                    .accessibilityIdentifier("run-status-text")
                Text(elapsedTimeText)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            if currentRun.state == .running {
                ProgressView()
                    .scaleEffect(0.8)
            }
        }
        .padding(.horizontal)
        .padding(.vertical, 12)
        .background(.fill.tertiary)
        .accessibilityIdentifier("run-status-section")
    }

    @ViewBuilder
    private var statusIcon: some View {
        switch currentRun.state {
        case .running:
            Image(systemName: "play.circle.fill")
                .foregroundStyle(.blue)
                .font(.title2)
        case .waitingApproval, .waitingInput:
            Image(systemName: "clock.fill")
                .foregroundStyle(.orange)
                .font(.title2)
        case .completed:
            Image(systemName: "checkmark.circle.fill")
                .foregroundStyle(.green)
                .font(.title2)
        case .failed:
            Image(systemName: "xmark.circle.fill")
                .foregroundStyle(.red)
                .font(.title2)
        case .cancelled:
            Image(systemName: "minus.circle.fill")
                .foregroundStyle(.secondary)
                .font(.title2)
        }
    }

    private var statusText: String {
        switch currentRun.state {
        case .running: return "Running"
        case .waitingApproval: return "Waiting for Approval"
        case .waitingInput: return "Waiting for Input"
        case .completed: return "Completed"
        case .failed: return "Failed"
        case .cancelled: return "Cancelled"
        }
    }

    private var elapsedTimeText: String {
        let elapsed = currentRun.updatedAt.timeIntervalSince(currentRun.createdAt)
        return formatDuration(elapsed)
    }

    private func formatDuration(_ interval: TimeInterval) -> String {
        let seconds = Int(interval)
        if seconds < 60 {
            return "\(seconds)s"
        } else if seconds < 3600 {
            let minutes = seconds / 60
            let secs = seconds % 60
            return "\(minutes)m \(secs)s"
        } else {
            let hours = seconds / 3600
            let minutes = (seconds % 3600) / 60
            return "\(hours)h \(minutes)m"
        }
    }

    // MARK: - Pending Action Cards

    private func pendingActionCard(approval: Approval) -> some View {
        Button {
            showingApproval = true
        } label: {
            HStack {
                Image(systemName: "exclamationmark.circle.fill")
                    .foregroundStyle(.white)
                Text("Needs approval")
                    .font(.subheadline.weight(.medium))
                    .foregroundStyle(.white)
                Spacer()
                Image(systemName: "chevron.right")
                    .foregroundStyle(.white.opacity(0.7))
            }
            .padding()
            .background(Color.orange)
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("pending-approval-card")
    }

    private func pendingInputCard(question: String) -> some View {
        Button {
            showingInput = true
        } label: {
            HStack {
                Image(systemName: "questionmark.circle.fill")
                    .foregroundStyle(.white)
                Text("Waiting for you")
                    .font(.subheadline.weight(.medium))
                    .foregroundStyle(.white)
                Spacer()
                Image(systemName: "chevron.right")
                    .foregroundStyle(.white.opacity(0.7))
            }
            .padding()
            .background(Color.orange)
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("pending-input-card")
    }

    // MARK: - Event Feed

    private var eventFeed: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 8) {
                    if isLoading && events.isEmpty {
                        ProgressView()
                            .frame(maxWidth: .infinity, alignment: .center)
                            .padding()
                    } else if events.isEmpty {
                        Text("No events yet")
                            .foregroundStyle(.secondary)
                            .frame(maxWidth: .infinity, alignment: .center)
                            .padding()
                    } else {
                        ForEach(displayableEvents) { event in
                            EventRowView(event: event)
                                .id(event.id)
                        }
                    }

                    // Anchor for scrolling
                    Color.clear
                        .frame(height: 1)
                        .id("bottom")
                }
                .padding()
            }
            .onChange(of: events.count) { _, _ in
                if isAutoScrollEnabled {
                    withAnimation {
                        proxy.scrollTo("bottom", anchor: .bottom)
                    }
                }
            }
            .simultaneousGesture(
                DragGesture().onChanged { _ in
                    isAutoScrollEnabled = false
                }
            )
        }
    }

    /// Filter out run_started events (implied by run existing)
    private var displayableEvents: [RunEvent] {
        events.filter { $0.type != .runStarted }
    }

    // MARK: - Footer Buttons

    @ViewBuilder
    private var footerButtons: some View {
        switch currentRun.state {
        case .running:
            Button(role: .destructive) {
                cancelRun()
            } label: {
                Text("Cancel Run")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.bordered)
            .padding()
            .background(.bar)
            .accessibilityIdentifier("cancel-run")

        case .completed, .failed, .cancelled:
            HStack(spacing: 16) {
                Button {
                    retryRun()
                } label: {
                    Text("Retry")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.bordered)

                Button {
                    retryPrompt = currentRun.prompt
                    showingRetrySheet = true
                } label: {
                    Text("Edit & Retry")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
            }
            .padding()
            .background(.bar)

        default:
            EmptyView()
        }
    }

    private var retrySheet: some View {
        NavigationStack {
            VStack(spacing: 16) {
                TextEditor(text: $retryPrompt)
                    .font(.body)
                    .scrollContentBackground(.hidden)
                    .padding(12)
                    .background(.fill.tertiary)
                    .clipShape(RoundedRectangle(cornerRadius: 8))
                    .overlay(alignment: .topLeading) {
                        if retryPrompt.isEmpty {
                            Text("What should the agent do?")
                                .font(.body)
                                .foregroundStyle(.tertiary)
                                .padding(.horizontal, 16)
                                .padding(.vertical, 20)
                                .allowsHitTesting(false)
                        }
                    }
                Spacer()
            }
            .padding()
            .navigationTitle("Edit & Retry")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        showingRetrySheet = false
                    }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Start") {
                        showingRetrySheet = false
                        createNewRun(prompt: retryPrompt)
                    }
                    .disabled(retryPrompt.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
            }
        }
    }

    // MARK: - Data Loading

    private func loadInitialData() async {
        isLoading = true

        async let runRefresh: () = refreshRun()
        async let eventsLoad: () = loadEvents()

        _ = await (runRefresh, eventsLoad)

        isLoading = false
    }

    private func refreshRun() async {
        do {
            currentRun = try await apiClient.getRun(id: run.id)

            // Check for pending approval
            if currentRun.state == .waitingApproval {
                await loadPendingApproval()
            }
        } catch let mError as MError {
            error = mError
        } catch {
            self.error = .unknown(statusCode: 0, message: error.localizedDescription)
        }
    }

    private func loadEvents() async {
        do {
            events = try await apiClient.listEvents(runID: run.id)

            // Extract pending input question from events
            for event in events.reversed() {
                if event.type == .inputRequested, let question = event.data.question {
                    pendingInputQuestion = question
                    break
                } else if event.type == .inputReceived {
                    pendingInputQuestion = nil
                    break
                }
            }
        } catch let mError as MError {
            error = mError
        } catch {
            self.error = .unknown(statusCode: 0, message: error.localizedDescription)
        }
    }

    private func loadPendingApproval() async {
        do {
            let approvals = try await apiClient.listPendingApprovals()
            pendingApproval = approvals.first { $0.runID == run.id }
        } catch {
            // Silently fail - not critical
        }
    }

    // MARK: - WebSocket

    private func startWebSocket() {
        guard let server = extractServer() else { return }
        guard let apiKey = extractAPIKey() else { return }

        webSocketClient = WebSocketClient(
            server: server,
            apiKey: apiKey,
            runID: run.id
        )

        Task {
            guard let client = webSocketClient else { return }
            do {
                for try await message in client.messages {
                    await handleServerMessage(message)
                }
            } catch {
                // WebSocket disconnected - that's ok for terminal states
            }
        }
    }

    private func handleServerMessage(_ message: ServerMessage) async {
        switch message {
        case .event(let event):
            // Append event if not already present
            if !events.contains(where: { $0.id == event.id }) {
                events.append(event)
                events.sort { $0.seq < $1.seq }
            }

            // Handle special event types
            switch event.type {
            case .approvalRequested:
                await loadPendingApproval()
            case .approvalResolved:
                pendingApproval = nil
            case .inputRequested:
                pendingInputQuestion = event.data.question
            case .inputReceived:
                pendingInputQuestion = nil
            default:
                break
            }

        case .state(let state):
            currentRun = Run(
                id: currentRun.id,
                repoID: currentRun.repoID,
                prompt: currentRun.prompt,
                state: state,
                createdAt: currentRun.createdAt,
                updatedAt: Date()
            )

        case .ping:
            break // Handled by WebSocketClient
        }
    }

    private func extractServer() -> MServer? {
        // The server URL should be accessible from the APIClient
        return MServer(name: "", url: apiClient.serverURL)
    }

    private func extractAPIKey() -> String? {
        // This should be passed through or retrieved from keychain
        // For now, we'll rely on the API client having been configured
        return apiClient.apiKeyForWebSocket
    }

    // MARK: - Actions

    private func cancelRun() {
        Task {
            do {
                try await apiClient.cancelRun(id: run.id)
                await refreshRun()
            } catch let mError as MError {
                error = mError
            } catch {
                self.error = .unknown(statusCode: 0, message: error.localizedDescription)
            }
        }
    }

    private func retryRun() {
        createNewRun(prompt: currentRun.prompt)
    }

    private func createNewRun(prompt: String) {
        Task {
            do {
                let newRun = try await apiClient.createRun(repoID: currentRun.repoID, prompt: prompt)
                // Navigate to new run (this will need navigation coordination)
                currentRun = newRun
                events = []
                pendingApproval = nil
                pendingInputQuestion = nil
                webSocketClient?.disconnect()
                await loadInitialData()
                startWebSocket()
            } catch let mError as MError {
                error = mError
            } catch {
                self.error = .unknown(statusCode: 0, message: error.localizedDescription)
            }
        }
    }
}

// MARK: - Event Row View

private struct EventRowView: View {
    let event: RunEvent
    @State private var isExpanded = false

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            eventContent
        }
    }

    @ViewBuilder
    private var eventContent: some View {
        switch event.type {
        case .stdout:
            if let text = event.data.text {
                Text(text)
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.primary)
                    .textSelection(.enabled)
            }

        case .stderr:
            if let text = event.data.text {
                Text(text)
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.red.opacity(0.8))
                    .textSelection(.enabled)
            }

        case .toolCallStart:
            HStack(spacing: 6) {
                ProgressView()
                    .scaleEffect(0.6)
                Text("\(event.data.tool ?? "tool")...")
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.secondary)
            }

        case .toolCallEnd:
            let success = event.data.success ?? false
            let duration = event.data.durationMs.map { String(format: "%.1fs", Double($0) / 1000) } ?? ""
            HStack(spacing: 4) {
                Image(systemName: success ? "checkmark" : "xmark")
                    .font(.caption2)
                    .foregroundStyle(success ? .green : .red)
                Text("\(event.data.tool ?? "tool") (\(duration))")
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.secondary)
            }

        case .inputReceived:
            HStack(spacing: 4) {
                Text("You:")
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.blue)
                Text(event.data.text ?? "")
                    .font(.system(.caption, design: .monospaced))
            }

        case .approvalResolved:
            let approved = event.data.approved ?? false
            HStack(spacing: 4) {
                Image(systemName: approved ? "checkmark" : "xmark")
                    .font(.caption2)
                    .foregroundStyle(approved ? .green : .red)
                Text(approved ? "Changes applied" : "Changes rejected")
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.secondary)
            }

        case .runCompleted:
            HStack(spacing: 4) {
                Image(systemName: "checkmark.circle.fill")
                    .font(.caption)
                    .foregroundStyle(.green)
                Text("Run completed")
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.secondary)
            }

        case .runFailed:
            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 4) {
                    Image(systemName: "xmark.circle.fill")
                        .font(.caption)
                        .foregroundStyle(.red)
                    Text("Run failed")
                        .font(.system(.caption, design: .monospaced))
                        .foregroundStyle(.red)
                }
                if let errorText = event.data.error {
                    Text(errorText)
                        .font(.system(.caption2, design: .monospaced))
                        .foregroundStyle(.red.opacity(0.8))
                }
            }

        case .runCancelled:
            HStack(spacing: 4) {
                Image(systemName: "minus.circle.fill")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("Run cancelled")
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.secondary)
            }

        case .runStarted, .approvalRequested, .inputRequested:
            // Not displayed in feed per spec
            EmptyView()
        }
    }
}
