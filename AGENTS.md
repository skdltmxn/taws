# AI Agent Guidelines for taws

This document outlines the context, standards, and rules for AI Agents working on the `taws` repository.

## Core Directives

*   **No Unnecessary Comments**: Code should be self-documenting. Comments should explain *why*, not *what*.
*   **No Unnecessary Abstraction**: Keep the design simple. Introduce interfaces only when necessary for decoupling or testing.
*   **No Over-engineering**: Solve the problem at hand with the simplest effective solution.
*   **Obsess Over Code Quality**: Adhere strictly to idiomatic Go practices and maintain high standards.
*   **Concise & Readable**: Write code that is easy to understand and maintain.
*   **English Only**: All code, comments, documentation, and commit messages must be in English.

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

## Coding Standards

*   **Error Handling**: Never `panic`. Propagate errors to be handled in the UI.
*   **Cross-Platform Compatibility**:
    *   Use `filepath.Join()` for paths.
    *   Use `os.UserHomeDir()` and `os.Getenv()`.
    *   Avoid platform-specific shell commands.
*   **Style**: Follow standard Go formatting (`gofmt`).

## UI/UX Context

*   **Layout**: k9s-inspired (Header, Content, Status Bar).
*   **Navigation**: Vim-style (`j`, `k`, `g`, `G`).
*   **Search**: Press `/` to filter the current list/table.
*   **Selection**: Press `space` to toggle row selection; selected rows are highlighted and the selected count is shown in the title.
*   **S3 Downloads**: In S3 objects view, press `d` to download selected object(s) after entering a destination path; progress is shown and `c` cancels an in-flight download.
*   **Command Palette**: Accessed via `:`, provides navigation to resources.

## Build & Verify

*   `make build`: Compile the binary.
*   `make run`: Run the application.
*   `make test`: Run unit tests.
*   `make fmt`: Format code.
*   `make vet`: Run static analysis.

Refer to `docs/spec.md` for comprehensive project specifications.
