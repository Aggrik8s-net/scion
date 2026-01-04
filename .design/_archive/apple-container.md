# Apple Native Container Support for Sandbox

## Objective

Enable the use of Apple's native `container` CLI as a backend for the Gemini CLI
sandbox, providing a lightweight, secure, and native virtualization alternative
to Docker/Podman on macOS.

## Background

Apple's `container` CLI allows running Linux containers on macOS using the
native Virtualization framework. It offers better performance and security
characteristics compared to running a full Linux VM for Docker Desktop, as it
uses lightweight VMs for each container.

The `container` CLI is largely compatible with Docker's CLI arguments, making it
a strong candidate for integration into the existing `gemini-cli` sandbox
architecture.

## Design

### 1. Configuration

- **`GEMINI_SANDBOX` Environment Variable**:
  - Support `GEMINI_SANDBOX=container` to explicitly select the Apple
    `container` runtime.
- **`settings.json`**:
  - Allow `"tools": { "sandbox": "container" }`.
- **`sandboxConfig.ts`**:
  - Add `'container'` to `VALID_SANDBOX_COMMANDS`.
  - Update `getSandboxCommand` to return `'container'` when requested or auto-detected.
  - **Auto-detection**: On macOS, if `container` is present, it is prioritized over
    other sandbox methods unless explicitly configured otherwise.

### 2. Implementation Details

#### `packages/cli/src/utils/sandbox.ts`

The `start_sandbox` function currently handles `docker` and `podman`. We will
extend this to handle `container` with specific adaptations:

- **Flag Compatibility**:
  - **`--init`**: The `container` CLI manages its own init process (`vminitd`)
    and does not support the `--init` flag used by Docker. This flag must be
    omitted when `config.command === 'container'`.
  - **`--platform`**: Supported.
  - **`--network`**: Supported (see Network Limitations below).
  - **`--add-host`**: `host.docker.internal` mapping is skipped for `container`.

- **Mount Constraints**:
  - The `container` runtime on macOS has limitations with virtiofs mounts.
    Specifically, mounting the same source directory to multiple destinations
    (e.g., user settings) causes errors. The implementation excludes the
    secondary mount for `container`.

- **Network Limitations (macOS 15 vs 26+)**:
  - The `gemini-cli` sandbox architecture uses a dedicated network
    (`gemini-cli-sandbox`) to facilitate communication between the sandbox
    container and a proxy container.
  - **macOS 26+**: Supports `container network create`. The existing logic to
    create and inspect networks should work, provided we check for command
    availability or version.
  - **macOS 15**: Does not support `container network create` and lacks
    container-to-container communication.
    - _Implication_: The proxy feature (if used) might not work as expected on
      older macOS versions with `container`.
    - _Mitigation_: We will attempt to run the network commands. If they fail
      (or if we detect the capability is missing), we might need to fallback or
      warn the user that proxying is disabled. For the initial implementation,
      we will assume the user has a capable version or accept the failure for
      the proxy setup.

- **Image Management**:
  - `container` supports `pull`, `build`, and `inspect` similar to Docker. The
    existing `ensureSandboxImageIsPresent` logic should remain largely
    compatible.

#### `scripts/build_sandbox.js`

- Ensure the build script respects `container` as a command for building
  the sandbox image if `GEMINI_SANDBOX=container`.

### 3. Prerequisites

- The user must have the `container` CLI installed and configured.
- The `container` system services must be running (`container system start`).

### 4. Proposed Changes

#### `packages/cli/src/config/sandboxConfig.ts`

```typescript
const VALID_SANDBOX_COMMANDS = [
  'docker',
  'podman',
  'sandbox-exec',
  'container',
] as const;
// ...
```

#### `packages/cli/src/utils/sandbox.ts`

Modify the arguments construction to conditionally exclude `--init`:

```typescript
// In start_sandbox function
const useInit = config.command !== 'container'; // 'container' manages its own init

// ...
if (useInit) {
  args.push('--init');
}
```

#### `scripts/sandbox_command.js`

Update to recognize `container` as a valid command to prevent "missing sandbox
command" errors during scripts execution.

## Verification

1.  **Setup**: Install `container` CLI. Run `container system start`.
2.  **Config**: Set `export GEMINI_SANDBOX=container`.
3.  **Run**: Execute `gemini --sandbox` and verify it launches inside the
    container.
4.  **Test**: Run `gemini -s -p "run shell command: uname -a"` to verify Linux
    environment.
5.  **Proxy Test** (if applicable): Verify network connectivity through the
    proxy.

## Risks

- **macOS Version Compatibility**: Users on older macOS versions might face
  networking issues.
- **Feature Parity**: While `container` is "Docker-like", subtle differences in
  volume mounting or networking might surface during deep testing.
