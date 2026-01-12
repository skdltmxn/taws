# AI Agent Guidelines for taws

This document outlines the context, standards, and rules for AI Agents working on the `taws` repository.

## Core Directives

*   **No Unnecessary Comments**: Code should be self-documenting. Comments should explain *why*, not *what*.
*   **No Unnecessary Abstraction**: Keep the design simple. Introduce interfaces only when necessary for decoupling or testing.
*   **No Over-engineering**: Solve the problem at hand with the simplest effective solution.
*   **Obsess Over Code Quality**: Adhere strictly to idiomatic Go practices and maintain high standards.
*   **Concise & Readable**: Write code that is easy to understand and maintain.
*   **English Only**: All code, comments, documentation, and commit messages must be in English.
*   **Keep Docs Updated**: After making changes, update `README.md` and `docs/spec.md` as needed to reflect new behavior, UX, commands, or configuration.

## Architecture & Design

The project follows **Clean Architecture**:

1.  **Presentation (`internal/ui`)**: `bubbletea` Model-View-Update pattern.
2.  **Application (`internal/app`)**: Orchestration and state management.
3.  **Domain (`internal/domain`)**: Core interfaces and entities (Pure Go).
4.  **Infrastructure (`internal/aws`)**: AWS SDK implementations.

**Tech Stack**:
*   Go 1.25+
*   UI: `charmbracelet/bubbletea`, `lipgloss`, `bubbles`
*   AWS SDK: `aws-sdk-go-v2`

## Supported Services

The application currently supports the following AWS services:
*   **EC2**: Instances (Start, Stop, Reboot, Terminate).
*   **VPC**: VPCs.
*   **S3**: Buckets and Objects (Download, Delete).
*   **EKS**: Clusters.
*   **ECR**: Repositories.
*   **IAM**: Users.
*   **Route53**: Hosted Zones.
*   **CloudWatch**: Logs.
*   **Lambda**: Functions.
*   **Profiles**: AWS Profile/Region switching.

## Coding Standards

*   **Error Handling**: Never `panic`. Propagate errors to be handled in the UI.
*   **Cross-Platform Compatibility**:
    *   Use `filepath.Join()` for paths.
    *   Use `os.UserHomeDir()` and `os.Getenv()`.
    *   Avoid platform-specific shell commands.
*   **Style**: Follow standard Go formatting (`gofmt`).

## UI/UX Context

*   **Layout**: k9s-inspired (Header, Content, Status Bar).
*   **Navigation**:
    *   `j`/`down`: Move down.
    *   `k`/`up`: Move up.
    *   `g`: Go to top.
    *   `G`: Go to bottom.
    *   `backspace`/`esc`: Go back / Close overlay.
    *   `r`: Refresh current view.
*   **Search**: Press `/` to filter the current list/table.
*   **Selection**: Press `space` to toggle row selection.
*   **Detail/Open**: Press `enter` to view details or enter a folder/resource.
*   **YAML View**: Press `y` to view resource details in YAML format (available in EC2, S3, etc.).
*   **Command Palette**: Accessed via `:`, provides navigation to resources.
*   **Help**: Press `?` to toggle the help overlay.

### Service-Specific Actions

*   **EC2**:
    *   `s`: Stop instance(s).
    *   `S`: Start instance(s).
    *   `R`: Reboot instance(s).
    *   `T`: Terminate instance(s).
*   **S3**:
    *   `d`: Download selected object(s)/folder(s).
    *   `x`: Delete selected object(s)/folder(s).

## Build & Verify

*   `make build`: Compile the binary.
*   `make run`: Run the application.
*   `make install`: Install the binary to `$GOPATH/bin`.
*   `make test`: Run unit tests.
*   `make fmt`: Format code.
*   `make vet`: Run static analysis.

Refer to `docs/spec.md` for comprehensive project specifications.
