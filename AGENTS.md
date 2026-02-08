# zDNS: Zero-Trust Discovery Service

zDNS is a high-performance, privacy-focused local network discovery protocol designed to replace mDNS/Bonjour. It operates as a "Dark" protocol, ensuring that device presence and metadata are only visible to cryptographically paired peers.

## Project Overview

### Architecture & Security
- **Stealth Transport:** Uses UDP Multicast (port 5354) but avoids plaintext broadcasts.
- **Rolling ServiceIDs:** Employs an HMAC-SHA256 based rolling identifier that rotates every 5 minutes (configurable), preventing long-term tracking by unauthorized sniffers.
- **Encrypted Payloads:** State blobs (containing Device ID, Battery, State, etc.) are encrypted and authenticated using **ChaCha20-Poly1305**.
- **Zero-Trust Model:** No "Trust on First Use" (TOFU). Trust is established via Out-of-Band (OOB) exchange of public keys and shared secrets.
- **Replay Protection:** Includes Unix timestamps in encrypted payloads with clock-skew validation.

### Core Technologies
- **Language:** Go 1.21+
- **Network:** `golang.org/x/net/ipv4` for robust multicast handling.
- **Cryptography:** `golang.org/x/crypto/chacha20poly1305` for authenticated encryption.

## Building and Running

### Prerequisites
- Go 1.21 or higher.

### Compilation
Use the provided Makefile for standard operations:

```bash
# Build the binary
make

# Run tests
make test

# Install to system (default /usr/local/bin)
sudo make install
```

### Execution
Run the unified `zdns` tool:

```bash
# Pair devices
zdns invite
zdns join --invite <CODE>

# Run the background daemon
zdns daemon

# Or use systemd (User Service)
systemctl --user enable --now zdns
```

## Data Persistence
zDNS adheres to modern OS conventions for data storage:
- **Linux:** `~/.config/zdns/`
- **macOS:** `~/Library/Application Support/zdns/`
- **Windows:** `%AppData%\zdns\`

### Configuration Files
- `identity.json`: Local X25519 identity keys.
- `peers.json`: Trusted peer metadata.
- `secrets.json`: Encrypted shared secrets.
- `triggers.json`: (Optional) Automation triggers for peer state changes.
- `anyisland.json`: Anyisland distribution manifest.

## Anyisland Integration
zDNS is **Anyisland Aware**, supporting easy installation and OTA updates:
- **Auto-Registration**: The daemon automatically registers with the Anyisland host on startup.
- **Pulse Aware**: Supports OTA update notifications via Anyisland Pulse.
- **Status Command**: Use `zdns managed` to check if your installation is being managed by Anyisland.

## Automation (Triggers)
You can automate actions based on peer state changes by creating `triggers.json` in your config directory:

```json
{
  "triggers": [
    {
      "peer_name": "MyPhone",
      "event": "state_change",
      "value": "LOCKED",
      "command": "notify-send 'Phone Locked' 'Securing workstation...'"
    }
  ]
}
```
Available environment variables in commands: `$ZDNS_PEER_NAME`, `$ZDNS_PEER_BAT`, `$ZDNS_PEER_TAGS`.

## Development Conventions
- **Minimal Dependencies:** Prefer Go standard library or `golang.org/x` packages. Avoid heavy frameworks.
- **Security First:** Never log or broadcast plaintext identifiers.
- **Concurrency:** Use goroutines for non-blocking I/O in listeners and broadcasters.
- **Error Handling:** Fail silently for unauthorized packets to avoid leaking information to potential attackers.
- **Build Hygiene:** NEVER create build executables or binaries directly in the project root or source directories. All builds must be directed to the `build/` directory, which is git-ignored.

## Project Structure
- `pkg/zdns/`: Core protocol library.
    - `types.go`: Protocol schemas and state definitions.
    - `crypto.go`: Encryption, decryption, and rolling hash logic.
    - `peers.go`: Secure management of trusted peer identities.
    - `network.go`: UDP Multicast listener and broadcaster implementation.
    - `storage.go`: JSON persistence for the PeerStore using OS-standard config directories.
    - `vault.go`: Encryption at rest logic.
    - `pairing.go`: X25519 pairing and identity logic.
    - `history.go`: Encrypted event logging.
    - `drop.go`: Encrypted file transfer logic.
    - `relay.go`: Remote signaling relay logic.
    - `anyisland.go`: Anyisland auto-registration logic.
    - `pulse.go`: Anyisland Pulse IPC logic.
- `pkg/sysinfo/`: Platform-specific system state providers (Battery, Lock status, Health probing).
- `pkg/ipc/`: Inter-Process Communication (Unix Domain Sockets).
- `pkg/triggers/`: Automation engine for state-based actions.
- `pkg/commands/`: Safe command executor for remote actions.
- `pkg/tui/`: Bubble Tea stealth dashboard.
- `pkg/dnsbridge/`: Local DNS resolver for .zdns domains.
- `cmd/zdns/`: Main CLI entry point and modular subcommands.