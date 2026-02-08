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

## Project Structure
- `pkg/zdns/`: Core protocol library.
    - `types.go`: Protocol schemas and state definitions.
    - `crypto.go`: Encryption, decryption, and rolling hash logic.
    - `peers.go`: Secure management of trusted peer identities.
    - `network.go`: UDP Multicast listener and broadcaster implementation.
    - `storage.go`: JSON persistence for the PeerStore using OS-standard config directories.
- `cmd/`: Application entry points.

## Data Persistence
zDNS adheres to modern OS conventions for data storage:
- **Linux:** `~/.config/zdns/`
- **macOS:** `~/Library/Application Support/zdns/`
- **Windows:** `%AppData%\zdns\`
- **Permissions:** All configuration files are created with `0600` (user-only) permissions to protect cryptographic secrets.
    - `advertiser/`: PoC for a device announcing its state.
    - `listener/`: PoC for a device discovering trusted peers.

## Development Conventions
- **Minimal Dependencies:** Prefer Go standard library or `golang.org/x` packages. Avoid heavy frameworks.
- **Security First:** Never log or broadcast plaintext identifiers.
- **Concurrency:** Use goroutines for non-blocking I/O in listeners and broadcasters.
- **Error Handling:** Fail silently for unauthorized packets to avoid leaking information to potential attackers.
