# AWS TUI Console (taws) Development Plan

## 1. Architecture & Design Principles

### Architecture
We adopt a **Clean Architecture** approach to ensure separation of concerns and testability:
- **Presentation Layer (`internal/ui`)**: Implements the `bubbletea` Model-View-Update pattern. Handles user input and rendering.
- **Application Layer (`internal/app`)**: Orchestrates business logic, manages application state, and bridges UI with Infrastructure.
- **Domain Layer (`internal/domain`)**: Defines core interfaces (ports), entities, and business rules. Pure Go, no dependencies on UI or AWS SDK.
- **Infrastructure Layer (`internal/aws`)**: Implements domain interfaces using AWS SDK for Go v2. Handles API calls and credential management.

### Tech Stack
- **Language**: Go 1.21+
- **UI Framework**: [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea)
- **Styling**: [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss)
- **Tables/Lists**: [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles)
- **AWS SDK**: `aws-sdk-go-v2`

## 2. Project Structure

```text
.
├── cmd
│   └── taws
│       └── main.go           # Entry point
├── internal
│   ├── app                   # Application logic & state
│   │   └── app.go
│   ├── aws                   # AWS SDK implementations
│   │   ├── client.go         # Main AWS client with SSO support
│   │   ├── ec2.go
│   │   ├── ecr.go
│   │   ├── eks.go
│   │   ├── iam.go
│   │   ├── route53.go
│   │   ├── s3.go
│   │   └── vpc.go
│   ├── config                # Configuration & Credential loading
│   │   ├── config.go
│   │   ├── profiles.go       # AWS profile listing
│   │   ├── regions.go        # AWS region list
│   │   ├── theme.go          # Theme loading and management
│   │   └── userconfig.go     # User preferences (persistent state)
│   ├── domain                # Interfaces & Structs
│   │   ├── aws.go
│   │   ├── ec2.go
│   │   ├── ecr.go
│   │   ├── eks.go
│   │   ├── iam.go
│   │   ├── profile.go        # AWS profile struct
│   │   ├── route53.go
│   │   ├── s3.go
│   │   └── vpc.go
│   └── ui                    # Bubbletea components
│       ├── model.go          # Main UI model
│       ├── common
│       │   └── styles.go     # Shared styles
│       ├── components
│       │   ├── cmdpalette/   # Command palette component
│       │   └── header/       # Header component
│       └── pages
│           ├── ec2/model.go
│           ├── ecr/model.go
│           ├── eks/model.go
│           ├── iam/model.go
│           ├── profiles/model.go  # AWS profiles & region switching
│           ├── route53/model.go
│           ├── s3/model.go
│           └── vpc/model.go
├── examples
│   └── themes/           # Example theme files
├── docs
│   └── spec.md
├── Makefile
└── go.mod
```

## 3. Implementation Status

### Phase 1: Foundation - COMPLETED
- [x] Project structure and dependencies
- [x] AWS authentication (Env vars, Profile, SSO)
- [x] k9s-style UI layout (header + command palette, no sidebar)
- [x] Command palette navigation (`:` key)
- [x] Help overlay (`?` key)
- [x] Terminal resize handling

### Phase 2: Core Compute & Networking - COMPLETED
- [x] **EC2 Module**:
  - [x] List instances with status (running/stopped/pending/stopping)
  - [x] Detail view for selected instance
  - [x] Actions: Start (`S`), Stop (`s`), Reboot (`R`), Terminate (`T`)
  - [x] Confirmation modal for destructive actions (stop/terminate)
  - [x] Multi-select batch operations with `space`
  - [x] Vim-style navigation (j/k/g/G)
- [x] **VPC Module**:
  - [x] List VPCs with CIDR and state
  - [x] Drill-down to view subnets per VPC
  - [x] Vim-style navigation

### Phase 3: Storage & Containers - COMPLETED
- [x] **S3 Module**:
  - [x] List buckets
  - [x] Browse objects (file explorer style with folder navigation)
  - [x] Cross-region bucket support (auto-detects bucket region)
  - [x] Download objects to local with destination path prompt, progress, and cancel
  - [x] Delete objects/folders with confirmation modal (recursive folder deletion)
  - [x] Vim-style navigation
- [x] **ECR Module**:
  - [x] List repositories with URI
  - [x] Vim-style navigation
- [x] **EKS Module**:
  - [x] List clusters with version and status
  - [x] Vim-style navigation

### Phase 4: Security & Management - COMPLETED
- [x] **IAM Module**:
  - [x] List users with ID, ARN, and creation date
  - [x] Vim-style navigation
- [x] **Route53 Module**:
  - [x] List hosted zones with record count
  - [x] Vim-style navigation

### Phase 5: Runtime Configuration - COMPLETED
- [x] **Profiles Module**:
  - [x] List AWS profiles from ~/.aws/config and ~/.aws/credentials
  - [x] Display profile type (IAM/SSO) and region
  - [x] Runtime profile switching
  - [x] Runtime region switching (tab-based UI)
  - [x] Auto-refresh all pages on profile/region change

### Future Enhancements (Not Yet Implemented)
- [x] ECR: List image tags within repositories
- [x] EKS: List nodegroups within clusters
- [x] IAM: List roles
- [x] Route53: List records within hosted zones
- [x] CloudWatch: View Log Groups and tail logs

## 4. Coding Standards

- **Language**: English only for code, comments, and commit messages.
- **Comments**: No redundant comments. Only explain "why", not "what".
- **Error Handling**: No `panic`. All errors propagated or handled in UI.
- **Abstraction**: Define interfaces in `domain` only when there are multiple implementations or for testing mockability.
- **Cross-Platform**: Code must work on Windows, Linux, and macOS. See guidelines below.

### Cross-Platform Guidelines

taws must run correctly on **Windows**, **Linux**, and **macOS**. Follow these guidelines:

#### File Paths
- Always use `filepath.Join()` and `filepath.Dir()` for path construction
- Never hardcode path separators (`/` or `\`)
- Use `os.UserHomeDir()` for home directory detection (handle errors properly)

#### Environment Variables
- Use `os.Getenv()` and `os.Environ()` for environment variable access
- Environment variables work the same across all platforms

#### External Commands
- Avoid platform-specific shell commands when possible
- When calling external commands (e.g., `aws` CLI), ensure they work on all platforms
- Use `exec.Command()` without shell-specific syntax

#### Terminal Handling
- The bubbletea library handles terminal abstraction across platforms
- Avoid direct ANSI escape codes; use lipgloss for styling

#### What to Avoid
- Unix-specific paths (`/usr/bin`, `/etc`, `~`)
- Platform-specific system calls
- Shell-specific syntax in `exec.Command()`
- Hardcoded file permissions (not needed for config files)

## 5. UI/UX Guidelines

### Layout
taws uses a k9s-inspired layout:
- **Header** (2 lines): Shows Account ID, Region, Profile, User ARN, and current resource
- **Content Area**: Full-width bordered area for resource views
- **Status Bar** (1 line): Shows status and quick tips

### Keybindings

#### Global Keys
| Key | Action |
|-----|--------|
| `?` | Show help overlay |
| `:` | Open command palette |
| `ctrl+c` | Quit |

#### Command Palette
| Key | Action |
|-----|--------|
| `up` / `ctrl+p` | Previous item |
| `down` / `ctrl+n` | Next item |
| `enter` | Select |
| `esc` | Close |

#### Navigation (All Pages)
| Key | Action |
|-----|--------|
| `j` / `down` | Move down |
| `k` / `up` | Move up |
| `g` | Go to top |
| `G` | Go to bottom |
| `/` | Search / filter list |
| `space` | Toggle select row |
| `enter` | Select / Open |
| `backspace` / `esc` | Go back (and clears active filter, if any) |
| `r` | Refresh |
| `y` | Show YAML view of selected resource (at deepest level) |

#### EC2 Actions (List & Detail View)
| Key | Action |
|-----|--------|
| `S` | Start instance(s) (when stopped) |
| `s` | Stop instance(s) (when running) - requires confirmation |
| `R` | Reboot instance (when running, detail view only) |
| `T` | Terminate instance(s) - requires confirmation |

**Note**: Stop and Terminate actions require typing a confirmation keyword (`stop` or `terminate`) to prevent accidental operations. Multi-select with `space` is supported for batch operations.

#### Profiles Page
| Key | Action |
|-----|--------|
| `tab` | Switch between Profiles/Regions tabs |
| `enter` | Switch to selected profile/region |

#### S3 Page (Objects View)
| Key | Action |
|-----|--------|
| `d` | Download selected object(s) |
| `x` | Delete selected object(s)/folder(s) - requires confirmation |
| `c` | Cancel download (while downloading) |

**Note**: Delete action requires typing `delete` to confirm. Folder deletion recursively deletes all contents. Multi-select with `space` is supported for batch operations.

#### YAML View (All Detail Views)
| Key | Action |
|-----|--------|
| `j` / `k` | Scroll up/down |
| `g` / `G` | Go to top/bottom |
| `ctrl+d` / `ctrl+u` | Half page down/up |
| `f` | Toggle fullscreen (hides all UI, shows only YAML data) |
| `esc` / `backspace` / `q` | Return to list view (or exit fullscreen first) |

**Note**: Press `y` on any resource at the deepest navigation level (e.g., EC2 instances, S3 objects, ECR images, VPC subnets, EKS nodegroups, IAM users/roles, Route53 records, CloudWatch log events) to view its details in YAML format. Press `f` to toggle fullscreen mode for easier copying of data.

### Command Palette Commands
| Alias | Name |
|-------|------|
| `:ec2` | EC2 Instances |
| `:vpc` | VPC |
| `:s3` | S3 Buckets |
| `:eks` | EKS Clusters |
| `:ecr` | ECR Repositories |
| `:iam` | IAM Users |
| `:route53` | Route53 Hosted Zones |
| `:profiles` | AWS Profiles |
| `:home` | Dashboard |

### UI Features
- k9s-style header with AWS context info
- Command palette with fuzzy search and aliases
- Full-width content area with rounded border
- Loading indicators for API calls
- Error display in content area
- Status bar with quick tips
- Responsive terminal resize handling

## 6. Authentication & Credentials

### Supported Authentication Methods
taws supports all standard AWS credential methods via the AWS SDK credential chain:

1. **Environment Variables**: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
2. **Shared Credentials File**: `~/.aws/credentials`
3. **Shared Config File**: `~/.aws/config` (with profiles)
4. **AWS SSO (IAM Identity Center)**: SSO profiles configured in `~/.aws/config`
5. **IAM Instance Roles**: For EC2 instances with attached roles
6. **ECS Container Credentials**: For tasks running in ECS

### AWS SSO Support
taws fully supports AWS SSO (IAM Identity Center) profiles:

- Configure SSO profiles in `~/.aws/config`:
  ```ini
  [profile my-sso-profile]
  sso_start_url = https://my-org.awsapps.com/start
  sso_region = us-east-1
  sso_account_id = 123456789012
  sso_role_name = MyRole
  region = us-west-2
  ```

- Use the profile with taws:
  ```bash
  taws --profile my-sso-profile
  # or
  AWS_PROFILE=my-sso-profile taws
  ```

- **Auto-login**: If SSO token is expired, taws will automatically invoke `aws sso login` to refresh credentials.

### Runtime Profile/Region Switching
taws supports changing profiles and regions at runtime without restarting:

1. Open command palette with `:`
2. Type `profiles` or select "AWS Profiles"
3. Use `tab` to switch between Profiles and Regions tabs
4. Select a profile/region and press `enter` to switch
5. All resource pages will automatically refresh with new credentials

### Command-Line Options
```
taws [options]

Options:
  -p, --profile string    AWS profile name (supports SSO profiles)
  -r, --region string     AWS region (overrides profile/env settings)
```

### Priority Order
Configuration is resolved in this order (later overrides earlier):
1. User config file (saved state from previous session)
2. AWS SDK defaults (us-east-1 fallback)
3. Environment variables (`AWS_PROFILE`, `AWS_REGION`, `AWS_DEFAULT_REGION`)
4. Command-line flags (`--profile`, `--region`)
5. Runtime switching via Profiles page

### User Configuration File

taws saves user preferences to a JSON config file for session persistence:

**Config File Location:**
- **Linux/macOS**: `~/.config/taws/config.json` (or `$XDG_CONFIG_HOME/taws/config.json`)
- **Windows**: `%APPDATA%\taws\config.json`

**Saved State:**
```json
{
  "last_profile": "my-profile",
  "last_region": "us-west-2",
  "last_resource": "ec2",
  "theme": "dracula"
}
```

**Behavior:**
- On startup, taws restores the last profile, region, and resource page
- State is automatically saved when switching profiles, regions, or pages
- Environment variables and CLI flags override saved state
- If the config file doesn't exist, taws starts with defaults

### Custom Themes

taws supports custom color themes via JSON files.

**Theme File Location:**
- **Linux/macOS**: `~/.config/taws/themes/<theme-name>.json`
- **Windows**: `%APPDATA%\taws\themes\<theme-name>.json`

**Theme File Format:**
```json
{
  "name": "my-theme",
  "colors": {
    "primary": "#7D56F4",
    "secondary": "#6244C5",
    "error": "#FF0000",
    "success": "#3FB950",
    "warning": "#F4BD2D",
    "info": "#3794FF",
    "text": "#FFFFFF",
    "sub_text": "#888888",
    "border": "#888888",
    "header_bg": "#1a1a2e"
  }
}
```

**Color Properties:**
| Property | Description |
|----------|-------------|
| `primary` | Accent color for highlights, selections, titles |
| `secondary` | Status bar background, secondary elements |
| `error` | Error messages |
| `success` | Success messages, running/healthy states |
| `warning` | Pending/transition states |
| `info` | Tips, neutral notices |
| `text` | Main text color |
| `sub_text` | Dimmed text, labels, hints |
| `border` | Border color for content areas |
| `header_bg` | Header background color |

**Using a Theme:**
1. Create a theme file in the themes directory (e.g., `~/.config/taws/themes/my-theme.json`)
2. Set the theme in `config.json`: `"theme": "my-theme"`
3. Restart taws

**Built-in Themes:**
- `default` - Purple accent on dark background
- Example themes available in `examples/themes/`: dracula, nord, gruvbox, solarized-dark, catppuccin-frappe

## 7. Build & Run

### Using Makefile
```bash
# Build binary to bin/taws
make build

# Build and run
make run

# Run tests
make test

# Format code
make fmt

# Run go vet
make vet

# Install to $GOPATH/bin
make install

# Clean build artifacts
make clean
```

### Direct Go Commands
```bash
# Build
go build -o bin/taws ./cmd/taws/

# Run
./bin/taws

# With profile
./bin/taws --profile my-sso-profile

# With region override
./bin/taws --region ap-northeast-2
```
