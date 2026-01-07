# taws - AWS TUI Console

**taws** is a terminal-based user interface (TUI) for managing AWS resources, inspired by [k9s](https://github.com/derailed/k9s). It allows developers and DevOps engineers to navigate and interact with their AWS cloud infrastructure directly from the terminal with a Vim-like experience.

## Key Features

*   **Unified Interface**: Navigate EC2, S3, EKS, ECR, IAM, Route53, and VPC resources in a single TUI.
*   **Vim-Style Navigation**: Use `j`, `k`, `g`, `G`, `/` for efficient browsing.
*   **Multi-Account & Region Support**: Switch AWS profiles (including SSO) and regions at runtime without restarting.
*   **Resource Management**:
    *   **EC2**: Start, stop, reboot, and terminate instances with confirmation prompts for destructive actions.
    *   **S3**: Browse buckets/objects, download files with progress tracking, and delete objects/folders.
    *   **Containers**: Inspect EKS clusters and ECR repositories.
    *   **Networking**: View VPCs, subnets, and Route53 hosted zones.
    *   **IAM**: List users and their details.
*   **Command Palette**: Quickly jump to resources using `:`.
*   **Theming**: Custom color themes via JSON configuration.
*   **Cross-Platform**: Works on Linux, macOS, and Windows.

## Installation

### Using Go

If you have Go installed (1.21+):

```bash
go install github.com/skdltmxn/taws/cmd/taws@latest
```

### Building from Source

```bash
git clone https://github.com/skdltmxn/taws.git
cd taws
make build
# Binary will be in bin/taws
```

## Usage

Run `taws` to start the application using your default AWS profile/region.

```bash
taws
```

### Options

```bash
taws --profile my-profile  # Use specific AWS profile
taws --region us-west-2    # Override AWS region
```

### Authentication

`taws` supports standard AWS credential chains:
1. Environment variables (`AWS_ACCESS_KEY_ID`, etc.)
2. `~/.aws/credentials`
3. `~/.aws/config` (including AWS SSO)

If your SSO session is expired, `taws` will attempt to refresh it automatically.

## Keyboard Shortcuts

| Key | Action |
| --- | --- |
| `?` | Toggle Help Overlay |
| `:` | Open Command Palette |
| `/` | Search / Filter List |
| `j` / `k` | Move Down / Up |
| `g` / `G` | Go to Top / Bottom |
| `enter` | Select / View Details |
| `esc` / `backspace` | Go Back / Clear Filter |
| `tab` | Switch Tabs (in Profiles view) |
| `ctrl+c` | Quit |

### Resource Specific
*   **EC2**: `S` (Start), `s` (Stop), `R` (Reboot), `T` (Terminate)
    *   Stop and Terminate require typing confirmation keyword
    *   Multi-select supported for batch operations
*   **S3**: `d` (Download Object), `x` (Delete Object/Folder), `c` (Cancel Download)
    *   Delete requires typing confirmation keyword (`delete`)
    *   Folder deletion recursively removes all contents
*   **Selection**: `space` (Toggle selection)

## Configuration & Themes

`taws` saves your session state (last profile, region, theme) automatically.

*   **Linux/macOS**: `~/.config/taws/config.json`
*   **Windows**: `%APPDATA%\taws\config.json`

### Custom Themes

You can customize the look and feel by placing JSON theme files in the `themes` directory next to your config.

Example `~/.config/taws/themes/my-theme.json`:
```json
{
  "name": "my-theme",
  "colors": {
    "primary": "#7D56F4",
    "secondary": "#2d2d2d",
    "error": "#ff5555",
    "success": "#50fa7b",
    "text": "#f8f8f2",
    "header_bg": "#282a36"
  }
}
```

Check [examples/themes](./examples/themes) for included themes like Dracula, Nord, and Solarized.

## Development

### Prerequisites
*   Go 1.25+
*   Make

### Commands
```bash
make run    # Run locally
make test   # Run tests
make fmt    # Format code
```

## License

MIT
