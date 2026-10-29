# tque

[English](README.md) | [日本語](README.ja.md)

tque is a terminal task manager with a CLI and an interactive TUI in one executable. Organize tasks into a hierarchy, track dependencies, set start and due dates, and manage manual or automatic progress.

For everyday use, see the [User Manual](docs/user-manual.md). A [Japanese manual](docs/user-manual.ja.md) is also available.

This is an initial implementation. See the [implementation and verification record (Japanese)](docs/implementation-status.md) for tested platforms and outstanding distribution work. A successful cross-build does not establish that the binary works on its target platform.

Windows targets Windows 11; the installer requires build 22000 or later and does not install on Windows 10.

## Screenshots

Interactive task tree with progress, due dates, and dependency indicators (Japanese UI).

![tque TUI showing hierarchical tasks and progress](images/screenshot1.png)

Adding tasks and listing the task tree from the CLI.

![tque CLI showing task creation and tree output](images/screenshot2.png)

## Build and run

```powershell
# Windows / PowerShell
go test ./...
go vet ./...
./scripts/build-release.ps1 -OutputDirectory ./.work/build
./.work/build/tque-windows-amd64.exe
```

```sh
# macOS / Linux, with Go installed
go test ./...
go vet ./...
bash scripts/build-release.sh "$(go env GOOS)" "$(go env GOARCH)" ./.work/build
./.work/build/tque-$(go env GOOS)-$(go env GOARCH)
```

The toolchain specified in `go.mod` is Go 1.27.3. Building may download that toolchain and dependencies. Running the application does not require Go, PowerShell, or an external database.

The examples below assume the executable is named `tque` and is on PATH. Otherwise, replace `tque` with its path, such as `.\tque.exe` in PowerShell.

## CLI

```text
tque add "Prepare release" --due +7d
tque add "Implement storage" --parent 1 -p 50
tque add "Integration tests" --parent 1 --dep 2
tque ls --all --tree
tque ls --search storage --sort due
tque ls --due-today
tque ls --ready --json
tque edit 2 -p auto
tque edit 2 --due none
tque done 2
tque undone 2
# Destructive: deletes task 1 and all descendants without a prompt
tque rm 1 --cascade
```

Replace example IDs with the IDs returned when you create tasks.

Use `tque renumber 8 6` (or `i` in the TUI) to move display ID 8 to 6; occupied numbers in between shift by one. Parent and dependency references retain their fixed internal identities. Schema 2 data is read without rewriting and migrated to schema 3 on the first content change, with the original retained in `tasks.schema2.backup`. Older versions cannot read schema 3.

- `ls` shows incomplete tasks by default; `--all` includes completed tasks. `--due-today` includes overdue tasks.
- `--dep 2,3 --dep 4` accepts multiple dependencies. Editing replaces the entire dependency list.
- Use `none` with `edit --parent`, `--dep`, `--start`, or `--due` to clear an attribute.
- `--tree --json` and `--ready --blocked` are invalid combinations.
- Progress of 100% does not automatically complete a task. Completing a parent does not complete its children; a parent can be completed only when all of its descendants are completed.
- Automatic progress averages the displayed progress of immediate children. `[manual]` indicates a fixed manual value.
- Incomplete dependencies produce a warning; they do not prevent editing or completion. Tasks referenced by dependencies outside the deletion set cannot be deleted.
- `rm ID` deletes without a confirmation prompt. `rm ID --cascade` also deletes all descendants. There is no undo.
- Exit codes: 0 success, 1 task/configuration/storage error, 2 usage error, 3 uncertain save result. For code 3, reload and inspect the result before retrying a change.

A confirmed save followed by a lock cleanup error prints the saved result and `saved; unlock failed`, then exits with code 1. Inspect the data before retrying; do not automatically repeat an addition.

## TUI

Run `tque` without arguments, or use `tque --tui`, in an interactive terminal.

| Key | Action |
| --- | --- |
| Up/Down or k/j | Move selection |
| PageUp / PageDown | Move by a page |
| J / K | Scroll down / up while keeping the selected screen line fixed |
| Left/Right | Collapse, expand, or navigate between parent and child |
| Enter / Esc | Open full details / return |
| a / A | Add a root task / child of the selected task |
| e / s / d / p / l | Edit title / start / due / progress / dependencies |
| i | Renumber the selected task |
| Space | Toggle completed/incomplete |
| + / - | Change progress by 10 and switch to manual mode |
| x / Delete | Confirm deletion: y deletes; n, Esc, or Enter cancels |
| / | Search titles; Left/Right move the text cursor while typing |
| f | Cycle all → incomplete → due through today → overdue → ready |
| o | Cycle ID/due-date/name sort |
| r | Reload task data |
| ? | Show help, including the full data path and latest notification |
| q / Ctrl+C | Quit; q is ordinary text in an input field |

Enter saves an attribute; Esc cancels. Empty date or dependency input clears it. In search input, Enter keeps the query and Esc restores the previous query. Esc in the task list clears only the search, keeping the filter.

To manage several projects, put each project's tasks under a top-level task and start with `tque --root NAME_OR_ID` to show only that task and its descendants. Any task can be the root. If several titles match, the TUI asks which one to open. `tque ls --root NAME_OR_ID` limits a listing the same way.

The TUI starts with every parent collapsed, showing only top-level tasks. Expanding a parent shows its immediate children; deeper parents stay collapsed. Reloading with `r` keeps your expanded/collapsed state.

Filtered trees retain ancestors as `[context]`; they are excluded from the match count. `!dep` means an incomplete dependency; `!` after a due date marks an overdue incomplete task.

Each change is saved individually. Update keys pressed during a save are ignored. During a save, only Ctrl+C requests exit after success; q is ignored. Conflicts reload the latest data and retain the draft for explicit reconfirmation.

## Storage

On all three operating systems, configuration is read from `~/.tque/config.json`, and the default data directory is `~/.tque`. The application does not create a configuration file automatically.

```json
{
  "data_dir": "~/tque-data"
}
```

A Windows absolute path can be written as `"D:/tque-data"`. Relative paths are resolved against the configuration directory. Environment variables are not expanded. Invalid configuration stops the operation instead of silently selecting another location. Restart after changing configuration.

The data directory contains `tasks.json`, an OS locking file named `lock`, and a one-generation backup named `tasks.json.backup`. Reads do not create `tasks.json`, but do open or create `lock` for read/write access; reading an existing store requires the corresponding permissions. Do not delete `lock` to release a lock. Concurrent changes through network shares or synchronization software are outside the supported guarantees.

Changing `data_dir` does not move data automatically. Stop all instances before migrating or restoring data. Preserve the complete document, including its revision. See the [manual](docs/user-manual.md#storage-configuration) for migration and recovery guidance.

## GitHub Actions ZIP builds

The `test-and-package` workflow runs on branch pushes, `v*` tag pushes, pull requests, and manual dispatch. It validates `VERSION`, tests on Windows, macOS, and Linux, then builds ZIPs for six OS/architecture combinations on Ubuntu.

ZIP names use the version in `VERSION`:

```text
tque-0.4.0-windows-amd64.zip
tque-0.4.0-windows-arm64.zip
tque-0.4.0-darwin-amd64.zip
tque-0.4.0-darwin-arm64.zip
tque-0.4.0-linux-amd64.zip
tque-0.4.0-linux-arm64.zip
```

`darwin` means macOS. Each ZIP contains a directory with the same base name, containing `tque.exe` on Windows or `tque` on macOS/Linux, both README languages, CHANGELOG, the complete docs directory (both manuals, design/review records, verification notes, checklists, and configuration examples), the application license, and dependency licenses. Unix executables are archived with execute permissions.

In GitHub, open **Actions → test-and-package → a successful run → Artifacts**, then download the desired target. Each artifact includes the ZIP and its `.zip.sha256` checksum. The artifact download is a wrapper archive containing the named ZIP; extract that ZIP as well. Builds use the runner's temporary directory rather than placing bare executables in the repository's `dist` directory. A version-tag push also publishes a GitHub Release as described below. The workflow does not create tags or signed installers.

### Publish a version-tag release

Push a tag named exactly as the contents of `VERSION`, such as `0.4.0`. Commit and push the intended source and workflow changes before tagging that commit. A mismatched tag is rejected before testing or packaging. The format is `MAJOR.MINOR.PATCH` without a `v` prefix; prerelease suffixes are not accepted.

After all three OS test jobs and all six packaging jobs succeed, the release job downloads the artifacts from that same run, checks each ZIP and SHA256, and publishes a release titled `tque 0.4.0` with automatically generated notes. The assets are six ZIP files and six `.zip.sha256` files; download them directly from the repository's Releases page. These are application binaries, not signed installers.

Only a **tag push** publishes a release. Branch pushes, pull requests, and manual workflow dispatch only build artifacts. The release job alone receives `contents: write` through the built-in `GITHUB_TOKEN`; no personal access token is needed. An existing release is not overwritten on reruns. If uploading fails, inspect the existing release/draft and assets before retrying. Tag creation and pushing remain explicit maintainer actions. See [GitHub CLI release creation](https://cli.github.com/manual/gh_release_create).

The project's [MIT license](LICENSE) identifies the copyright holder as `fukuyori`. The license prerequisite for ZIP creation is satisfied.

Workflow reference: [setup-go version-file support](https://github.com/actions/setup-go/blob/main/docs/advanced-usage.md) and [upload-artifact documentation](https://github.com/actions/upload-artifact).

The packaging script can also be tested in a Unix environment with Go and `zip` installed:

```sh
bash scripts/package-zip.sh linux amd64 ./.work/packages
```

It refuses to overwrite an existing archive. Build and staging directories are temporary; the output directory is explicitly specified. Explicit relative output paths in the build and collection scripts are resolved against the caller's working directory. Verify a downloaded ZIP with `sha256sum -c NAME.zip.sha256` on Linux or `shasum -a 256 -c NAME.zip.sha256` on macOS.

## Shell completion

Generate completion for `powershell`, `bash`, `zsh`, or `fish`. Profiles are not modified automatically.

```powershell
tque completion powershell | Out-String | Invoke-Expression
```

```sh
# Bash
source <(tque completion bash)
# Zsh
autoload -Uz compinit; compinit
source <(tque completion zsh)
# Fish
tque completion fish | source
```

## Other distribution scripts

`scripts/build-release.ps1` accepts `-OS windows|darwin|linux`, `-Arch amd64|arm64`, and `-OutputDirectory`. The Bash equivalent accepts OS, architecture, and output directory as positional arguments. Existing local build defaults still use `dist`; CI specifies a temporary output directory explicitly.

Distribution includes the project's [LICENSE](LICENSE), with `Copyright (c) 2026 fukuyori`. Packaging scripts reject missing or unresolved licensing information.

The [third-party module list](third-party-licenses/MODULES.txt) identifies the collected dependency versions. Original copyright notices and license texts are retained in the corresponding subdirectories. Package scripts collect these files again from the full module graph, including test/tool dependencies and licenses in subdirectories; this is broader than the modules linked into a particular executable.

On Windows, `scripts/build-installer.ps1` uses Inno Setup. With `-Sign`, it signs the executable, installer, and uninstaller. Set `CODESIGN_CERT` to the certificate subject name in your certificate store, matching wtmux's `signtool /n` convention. SignTool is discovered in the Windows SDK (native architecture first, then x64) or on PATH. The default RFC3161 timestamp URL is `http://timestamp.sectigo.com`, also matching wtmux.

```powershell
./scripts/build-installer.ps1 -Sign
```

To select by a 40-digit SHA1 thumbprint instead, set `CODESIGN_CERT` to that thumbprint and add `-CertificateSelector Thumbprint`. The default selector is `SubjectName`; certificate files/PFX are not supported. Override SDK discovery or the timestamp service with `-SignToolPath` / `-TimestampUrl`. The default certificate store is CurrentUser; use `-MachineCertificateStore` for LocalMachine. Argument handling has been checked on PowerShell 5.1 and 7. Configuration and task data are retained on uninstall. See [Microsoft's SignTool options](https://learn.microsoft.com/en-us/windows/win32/seccrypto/signtool) for `/n` and `/sha1` selection.

For macOS/Linux tar.gz packaging, use `bash scripts/package-release.sh OS ARCH`. It includes licenses and creates a SHA256 checksum. macOS signing and notarization require separate work.

To install an extracted Unix executable into a user-owned directory:

```sh
mkdir -p "$HOME/.local/bin"
install -m 755 tque "$HOME/.local/bin/tque"
```

Add `~/.local/bin` to PATH in your shell configuration if needed. tque does not change shell configuration.
