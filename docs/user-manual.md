# tque User Manual

[English](user-manual.md) | [日本語](user-manual.ja.md)

Applies to tque 0.2.4.

tque is a terminal task manager. Use commands through its CLI or operate a task list with the keyboard through its TUI. Both interfaces use the same saved data. This manual is in English; it does not change the language of messages displayed by the application.

## Contents

- [Starting tque](#starting-tque)
- [Your first tasks](#your-first-tasks)
- [Worked example: preparing a document](#worked-example-preparing-a-document)
- [Using the TUI](#using-the-tui)
- [Using the CLI](#using-the-cli)
- [Hierarchy, dependencies, and progress](#hierarchy-dependencies-and-progress)
- [Storage configuration](#storage-configuration)
- [Backup and recovery](#backup-and-recovery)
- [Troubleshooting](#troubleshooting)
- [Shell completion](#shell-completion)

## Starting tque

Use an executable built for your operating system and CPU. `amd64` means x64; `arm64` means ARM64. macOS builds use the OS name `darwin`. Windows targets Windows 11; the installer requires build 22000 or later and does not install on Windows 10. Running tque does not require Go or an external database. See the [README](../README.md) for builds and ZIP artifacts, and the [verification record (Japanese)](https://github.com/fukuyori/tque/blob/HEAD/docs/implementation-status.md) for platform coverage.

After extracting a ZIP, open a terminal in the directory containing the executable.

```powershell
# Windows / PowerShell
.\tque.exe
```

```sh
# macOS / Linux
./tque
```

If a Unix extraction tool did not preserve execute permission, set it with `chmod +x ./tque` before running. The ZIPs are unsigned; actual macOS execution and signing/notarization remain unverified.

The examples in this manual assume the executable is named `tque` and available on PATH. Otherwise, replace the first `tque` with its actual path, such as `.\tque.exe` in PowerShell or `./tque` on Unix.

```text
tque
tque --tui
tque --help
tque --version
tque add --help
```

No arguments, or `--tui`, opens the TUI. An interactive terminal is required. Do not combine `--tui` with a subcommand such as `ls`.

## Your first tasks

### In the TUI

1. Start `tque`.
2. Press `a`, type `Prepare a document`, and press Enter.
3. Select the new task, press `d`, enter `+7d`, and press Enter to save its due date.
4. Press uppercase `A`, type `Plan the outline`, and press Enter to create a child task.
5. Select the child and press Space to complete it.
6. Press Enter to open full details, Esc to return, and `q` to quit.

Each confirmed change is saved immediately. There is no separate save-on-exit action. Text being edited is not saved until you confirm it with Enter.

### In the CLI

IDs 1, 2, and 3 below assume an empty data store and commands executed in order. If you already have tasks, use the actual IDs reported by `add`.

```text
tque add "Prepare a document" --due +7d
tque add "Plan the outline" --parent 1
tque add "Write the text" --parent 1 --dep 2
tque ls --all --tree
tque done 2
tque ls --ready
```

“Write the text” is blocked while “Plan the outline” is incomplete. Completing task 2 clears that dependency warning. The parent may also appear in `--ready` when it meets the ready conditions.

## Worked example: preparing a document

This example starts with an empty store and follows three tasks through to completion. `>` marks a command to type; omit that marker when running it. Lines below each command are actual output. With existing data, replace IDs `1`, `2`, and `3` with the IDs reported by `add`. Running this after the previous example adds another set of tasks; you do not need to erase existing data.

### 1. Create a parent, children, and a dependency

```text
> tque add "Prepare a document"
[+] Created task #1: Prepare a document
> tque add "Plan the outline" --parent 1
[+] Created task #2: Plan the outline
> tque add "Write the text" --parent 1 --dep 2
[+] Created task #3: Write the text
> tque ls --all --tree
1 [ ] Prepare a document [.......... 0%]
├─ 2 [ ] Plan the outline [.......... 0%]
└─ 3 [ ] Write the text [.......... 0%] (blocked by #2)
```

`blocked by #2` means writing the text is waiting for task 2 to finish.

### 2. Update progress

```text
> tque edit 2 --progress 50
[*] Updated task #2
> tque ls --all --tree
1 [ ] Prepare a document [##........ 25%]
├─ 2 [ ] Plan the outline [manual] [#####..... 50%]
└─ 3 [ ] Write the text [.......... 0%] (blocked by #2)
```

The children are at 50% and 0%, so the parent automatically shows 25%. `[manual]` identifies explicitly set progress.

### 3. Clear the dependency

```text
> tque done 2
[x] Completed task #2
> tque ls --ready
1 [ ] Prepare a document [#####..... 50%]
3 [ ] Write the text [.......... 0%]
```

Task 3 is now ready and no longer carries a dependency warning. Parent task 1 also meets the ready conditions and appears in this list.

### 4. Complete the children and the parent

```text
> tque done 3
[x] Completed task #3
> tque ls --all --tree
1 [ ] Prepare a document [########## 100%]
├─ 2 [x] Plan the outline [manual] [########## 100%]
└─ 3 [x] Write the text [########## 100%]
> tque done 1
[x] Completed task #1
> tque ls --all --tree
1 [x] Prepare a document [########## 100%]
├─ 2 [x] Plan the outline [manual] [########## 100%]
└─ 3 [x] Write the text [########## 100%]
```

Completing all children brings the parent to 100%, but does not automatically mark it completed. `done 1` completes the parent. Use `--all` to include completed tasks.

### 5. Inspect the result in the TUI

Start `tque` to see the same three tasks in the tree. If it is already open, press `r` to reload and use `f` to select all tasks. Select the parent and press Left to collapse it or Right to expand it. Enter opens details; Esc returns to the list.

To remove these three practice tasks afterward, first check the parent ID and its children with `tque ls --all --tree`, then run `tque rm 1 --cascade`, replacing `1` with the actual parent ID. This command deletes the parent and its children without a confirmation prompt.

## Using the TUI

In the task list, uppercase `J` / `K` scroll down / up while keeping the selected screen line fixed. The task arriving on that line becomes selected. Blank separators are skipped; scrolling stops at the viewport boundaries. In input fields, these keys enter text.

### Reading the screen

Top-level titles and branch guides are white. Child levels cycle through cyan, teal, blue, and purple by depth. All titles use normal font weight, including parents. Completed task rows use light gray and `[x]` to indicate completion. For incomplete tasks, overdue dates are red and today's dates are yellow; the dependency marker `!dep` is also yellow. Selection uses reverse video instead of these colors, and completed rows do not use warning colors. The exact colors depend on your terminal palette. Set `NO_COLOR` to a nonempty value to disable colors and reverse video.

The TUI and CLI `tque ls --tree` use `├─`, `└─`, and `│` to mark child tasks. One blank line separates a visible group of children from the next top-level task. Consecutive top-level tasks have no blank line between them. In the TUI, separators cannot be selected, and collapsing a parent's children also removes the separator after that group.

The initial view is a tree of all tasks. The header shows the data location and search/filter information. The list occupies the center; selected-task information, the latest notification, and key hints appear below it. Press `?` to read the full data path and notification if they do not fit.

| Marker | Meaning |
| --- | --- |
| `>` at the start of a row | Selected task |
| `[ ]` / `[x]` | Incomplete / completed |
| `v` / `>` on a parent | Expanded / collapsed |
| `[manual]` | Progress is set manually |
| `[context]` | Ancestor included to show the location of a matching child |
| `!dep` | At least one dependency is incomplete |
| `!` after a due date | Incomplete and overdue |
| Bar made of `#` and `.`, or a percentage | Effective displayed progress |

The layout is designed around 80 columns by 24 rows. Narrower or shorter terminals omit the progress bar, due date, or lower detail area as needed. Use Enter for full details. Below 40 columns by 10 rows, normal operations are disabled: enlarge the terminal or quit with `q` / Ctrl+C.

### Keys in the task list

| Key | Action |
| --- | --- |
| Up / Down, k / j | Select previous / next task |
| PageUp / PageDown | Move by a page |
| Left | Collapse an expanded parent; otherwise move to its parent |
| Right | Expand a collapsed parent; otherwise move to a displayed child |
| Enter | Open full details of the selected task |
| a | Add a task without a parent |
| A | Add a child of the selected task |
| e | Edit title |
| i | Renumber the selected task |
| s / d | Edit start / due date |
| p | Edit progress: 0–100 or auto |
| l (lowercase L) | Edit dependencies, for example 2,3 |
| Space | Toggle completed/incomplete |
| + / - | Adjust progress by 10 and switch to manual mode |
| x / Delete | Open deletion confirmation |
| / | Start a title search |
| f | Cycle all → incomplete → due through today → overdue → ready → all |
| o | Cycle ID → due date → name → ID |
| Esc | Clear search; keep the filter selected with f |
| r | Reload saved task data |
| ? | Open help |
| q / Ctrl+C | Quit |

To change or remove a parent, use CLI `edit --parent`. On a completed task, `+` and `-` do not change progress; reopen it with Space first.

### Editing, details, and deletion

In an attribute input, Enter saves and Esc cancels. An empty start date, due date, or dependency input clears that attribute. Progress cannot be empty; enter `auto` to restore automatic calculation.

Left/Right move the text cursor. Home/End or Ctrl+A/Ctrl+E move to the beginning/end. Backspace/Delete remove text. Characters such as `q`, `j`, `k`, and `?` are ordinary input while editing. Ctrl+C quits without saving unconfirmed input.

Full details are read-only. Scroll with Up/Down, j/k, or PageUp/PageDown; Esc returns to the list. Pressing `q` in details quits the application. Close help with `?` or Esc.

Deletion confirmation shows the selected task ID and total number of tasks to remove, including hidden descendants; it does not list every descendant. Inspect the expanded tree before deleting. Press `y` to delete or `n`, `q`, Esc, or Enter to cancel. **There is no undo.** Deletion is refused if a task outside the deletion set depends on one of the tasks being removed.

### Search and filters

Press `/` and type to search titles immediately using a case-insensitive substring match. Enter keeps the query and returns to list navigation; Esc while editing restores the query that was active before editing. Regular expressions and fuzzy matching are not supported.

Search and the `f` filter are combined with AND. Ancestors of matching tasks remain visible as `[context]` but do not count as matches. While searching or filtering, matching paths are expanded and Left/Right navigate between parents and children. Clearing the conditions restores the previous collapsed state.

“Due through today” includes incomplete tasks due today or earlier. “Overdue” includes incomplete tasks due before today. “Ready” means incomplete, with no incomplete dependencies, and with a start date that is absent or no later than today.

### Concurrent use

Press `r` to load changes made through another instance or the CLI. There is no automatic file watcher. `r` reloads tasks, not configuration.

During a save, Ctrl+C requests exit after success; other keys, including `q`, are ignored. On failure, the application stays open and displays a notification.

If another process changed the data, the TUI reloads current data and preserves the draft for reconfirmation. Review the latest values before pressing Enter again, or cancel with Esc. If the edited task no longer exists, editing stops and the draft is retained in the notification.

## Using the CLI

### Commands

| Command | Purpose |
| --- | --- |
| tque add "Title" | Create a task |
| tque edit ID attribute-options | Change specified attributes |
| tque done ID | Complete a task |
| tque undone ID | Reopen a task |
| tque rm ID | Delete a task |
| tque rm ID --cascade | Delete the task and all descendants |
| tque ls | List incomplete tasks |

CLI deletion has no interactive confirmation. Check `ls --all --tree` before deleting. A task with children requires `--cascade`. Dependencies from outside the deletion set prevent deletion even with `--cascade`.

### Task IDs

A new task gets the highest existing ID (including completed tasks) plus one, or 1 if there are no tasks.

Screen IDs, CLI arguments, and `ls --json` use display IDs. Stored tasks also have fixed internal IDs, used for parent and dependency references. There is no user command requiring internal IDs.

Example: after deleting 3 from 1, 2, 3, the next new task gets 3.

### Renumbering tasks

Run `tque renumber 8 6` to move display ID 8 to 6. If 6 is occupied, the former 6 and 7 become 7 and 8. Moving 6 to 8 shifts the former 7 and 8 to 6 and 7. If the destination is unused, only the selected task changes. Using the same ID is a no-op; zero, negative, and out-of-range numbers are rejected.

In the TUI, select a task and press `i`, enter the new number, then press `Enter` to save or `Esc` to cancel. Selection follows the same task, and the notification lists the changes. Name and due-date sorting remain active.

Renumbering includes completed, filtered-out, and collapsed tasks. Moving a parent's number does not move its descendants as a block. Parent and dependency relationships continue to refer to the same tasks. If another operation changes numbers while you are editing, the TUI reports a conflict and cancels the edit. Check the current numbers before retrying. Subsequent CLI commands also interpret numbers as current display IDs.

Storage uses schema 3. Schema 2 data is migrated in memory on read and written as schema 3 on the first successful content change. Reads and no-op commands do not rewrite it. The original is retained as `tasks.schema2.backup`, separately from the rolling `tasks.json.backup`. An existing migration backup with different contents is not overwritten; saving stops instead. Older versions cannot read schema 3. Schema 1 remains unsupported. `ls --json` is a display-ID listing, not a replacement for the storage file.


### Add and edit options

| Option | Value |
| --- | --- |
| -t, --title | Title; edit only |
| -s, --start | Start date |
| -d, --due | Due date |
| -p, --progress | 0–100 or auto |
| --parent | Parent task ID |
| --dep | Dependency IDs; comma-separated and repeatable |

```text
tque add "Check behavior" --start today --due +3d -p 20
tque edit 3 --title "Write the first draft" --start today --due +5d
tque edit 3 --dep 2,4 --dep 5
tque edit 3 --parent 1
tque edit 3 --parent none --dep none --start none --due none
```

Omitted attributes remain unchanged during `edit`. Editing dependencies replaces the entire list rather than appending to it. The dependency example sets IDs 2, 4, and 5; they must exist.

Use `none` to clear parent, dependencies, start date, or due date with `edit` only. When adding a task, omit unwanted attributes. If one attribute in a multi-attribute edit is invalid, none of the changes are saved.

Titles must contain 1–200 Unicode code points after trimming surrounding whitespace. Control characters, including tabs and newlines, are rejected. Quote titles that contain spaces.

### Dates

| Format | Example | Meaning |
| --- | --- | --- |
| YYYY-MM-DD | 2026-11-01 | An explicit date |
| today | today | The local date of the operation |
| +Nd | +7d | N days after the local date of the operation |

N must be between 0 and 36500. Dates do not include a time. If both dates exist, start must be on or before due. Change both in the same `edit` if necessary to keep the pair valid.

### Listing, searching, and sorting

| Option | Effect |
| --- | --- |
| --all | Include completed tasks |
| --tree | Display the hierarchy |
| --search "text" | Match a substring in titles |
| --blocked | Tasks with incomplete dependencies |
| --due-today | Incomplete tasks due today or earlier |
| --overdue | Incomplete tasks due before today |
| --ready | Incomplete tasks ready to start |
| --sort name | Title in Unicode code-point order; ID breaks ties. No phonetic sorting of Japanese names |
| --sort id | ID order; default |
| --sort due | Due-date order; undated last, ID breaks ties |
| --json | Output a JSON array |

```text
tque ls --all --tree
tque ls --search "document" --sort due
tque ls --due-today
tque ls --ready --json
tque ls --all --blocked
```

Conditions combine with AND. `--due-today`, `--overdue`, and `--ready` still select only incomplete tasks when combined with `--all`. A tree includes ancestors of matching children, so a completed parent can appear without `--all`. Tree sorting applies within sibling groups.

Do not combine `--tree` with `--json`, or `--ready` with `--blocked`. JSON is a flat array containing task attributes plus `effective_progress`, `blocked`, and `blocked_by` (IDs of incomplete dependencies). `ls --json` includes only incomplete tasks by default. Use the actual storage document for a complete backup, not listing output.

### Results and exit codes

`Created task #number` identifies the newly assigned ID. `Unchanged task` means there was no change. An empty text list displays `対象なし` (no matches); JSON returns `[]`. Errors go to standard error.

| Exit code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Task, configuration, storage, or other operation error |
| 2 | Invalid command arguments or options |
| 3 | Save result uncertain; reload and inspect |

Immediately after a command, inspect `$LASTEXITCODE` in PowerShell, `$?` in Bash/Zsh, or `$status` in Fish.

If saving succeeds but lock cleanup fails, the CLI prints the saved result (including a newly assigned ID), reports `saved; unlock failed` on standard error, and exits with code 1. This does not mean the change was rolled back. Inspect the data before any retry; do not automatically repeat `add` on every nonzero exit.

## Hierarchy, dependencies, and progress

A parent/child relationship describes the breakdown of work. A dependency identifies another task to finish first. Making a task a child does not automatically make it depend on the parent.

`--dep 2` means “this task depends on task 2.” An incomplete dependency makes the task blocked. This is a warning; editing and completion are still allowed. Even a completed task may display a dependency warning. Self-references and cycles within either relationship graph are rejected.

There are two progress modes:

- **Automatic (auto):** the default. An incomplete parent displays the average of its immediate children's effective progress, rounded down. A task without children displays its stored value, initially zero.
- **Manual (manual):** setting a number switches to this mode. The value does not follow children. Use `edit ID -p auto`, or `p` then `auto` in the TUI, to return to automatic mode.

For example, children at 20% and 80% give an automatic parent 50%. In deeper hierarchies, each parent averages its immediate children, not all leaf tasks together.

**100% progress and completion are separate.** Parents can have manual progress of 0% or 100%; the chosen value stays in effect until explicitly switching to `auto` or completing the task. Completing a task sets its progress to 100% and its progress mode to `auto`. `-p 100` does not complete a task. `done` or Space can complete a parent only when all its descendants have completed status. An incomplete child at 100% still blocks completion. Children are not completed in bulk, and parents do not complete automatically.

The CLI rejects a numeric progress below 100 for a completed task. Use `undone ID` first, then `edit ID -p NUMBER`.

Reopening a completed task with `undone` or Space in the TUI resets its stored progress to 0% without changing its progress mode. A task without children displays 0%; an automatic parent recalculates from its children. Reopening only the parent does not change its children's progress. Running `undone` on an already incomplete task leaves its progress unchanged.

## Storage configuration

### Configuration file

On all operating systems the configuration path is `~/.tque/config.json`. `~` denotes the user's home directory, typically `C:\Users\username` on Windows.

The application does not create the configuration file. To choose a data location, create the `.tque` directory and a UTF-8 `config.json`. Inspect an existing file before editing it.

```json
{
  "data_dir": "~/tque-data"
}
```

A Windows D-drive example:

```json
{
  "data_dir": "D:/tque-data"
}
```

Backslashes in JSON must be escaped, as in `"D:\\tque-data"`. Forward slashes avoid that escaping on Windows.

| Setting | Meaning |
| --- | --- |
| No configuration file, or {} | Use ~/.tque |
| "data_dir": "~/tque-data" | tque-data in the user's home directory |
| "data_dir": "data" | ~/.tque/data, relative to the configuration file |
| Absolute path | Use that location; the path must suit the operating system |

`data_dir` is the only supported key. Comments, unknown or duplicate keys, `null`, and empty paths are invalid. Environment variables such as `%USERPROFILE%` and `$HOME` are not expanded. Invalid configuration causes an error instead of falling back to another directory.

Nonempty paths are used without trimming whitespace. For example, on Unix, `" /data"` is a relative path containing a leading space, not `/data`. Check for accidental whitespace when copying a path. This preserves valid directory names containing spaces.

Restart tque after editing configuration. TUI `r` does not reload it. There is no command-line option to relocate the configuration file.

### Moving existing data

Changing the configured directory does not move existing data.

1. Stop all tque instances.
2. Copy the current `tasks.json` and, if present, `tasks.json.backup` to a separate safe location. Record the current configuration too.
3. Prepare a new directory that does not contain other task data.
4. Copy the complete `tasks.json` into it, without extracting only part of the document. Skip this step if no task data has been created.
5. Update `data_dir` in `config.json`.
6. Check the contents with `tque ls --all --tree`, and confirm the data path in TUI help.

Keep the original data until verification is complete. Do not copy the lock or temporary files to the new location.

## Backup and recovery

| File | Purpose |
| --- | --- |
| tasks.json | Tasks and revision information |
| tasks.json.backup | One generation of the document before a real update |
| lock | OS lock coordination between processes |
| tasks.json.tmp, tasks.json.backup.tmp | Temporary save files |

The first save, an unchanged operation, and an input error do not create a new backup. Subsequent changes replace the backup, so it is not an edit/deletion history. For periodic backups, stop all instances and copy complete storage files elsewhere. A backup on the same storage device does not protect against failure of that device.

A remaining `lock` file is normal. Do not delete it to release a lock. Concurrent changes via network shares or synchronization tools are outside the supported guarantees.

Reading an existing data directory also opens or creates `lock` for read/write access. `ls` therefore needs permission to use that lock and, if it is absent, to create it. A directory or backup mounted read-only may not be readable through tque even when `tasks.json` itself is readable. Do not bypass the lock; copy a backup into a writable test directory to inspect it.

### Corrupt data

There is no automatic recovery or dedicated restore command. First stop every tque instance and preserve the current document, backup, and remaining temporary files elsewhere. Do not delete a corrupt document and continue with an empty store, or blindly replace it with a temporary file.

Simply overwriting the document with an older backup rolls `revision` backward. When both values are known, set the recovered `revision` to the larger value plus one. This is not possible if the integer limit would be exceeded.

Consult the [storage and recovery design (Japanese)](https://github.com/fukuyori/tque/blob/HEAD/docs/detailed-design.md) before manual restoration. Verify that the recovered document loads and contains the expected tasks before resuming normal updates.

## Troubleshooting

| Symptom or error | What to check |
| --- | --- |
| Command not found | Use the executable's path; for example .\tque.exe in PowerShell |
| TUI does not open | Use an interactive terminal. For pipes or redirection, use ls |
| A task is missing | CLI: ls --all. TUI: Esc to clear search and f to return to all. Check the data location |
| Parent progress does not change | Check manual mode and completion. Use -p auto to enable automatic calculation |
| ConfigError | Check the reported configuration path, JSON, key, and data path |
| InvalidInput | Check title, dates, progress range, and start/due order |
| NotFound | Check IDs with ls --all |
| CycleDetected | Check hierarchy or dependencies for cycles |
| ReferencedTask | Inspect dependencies outside the deletion set; edit them appropriately before retrying |
| RevisionConflict | Review current data and explicitly reconfirm the retained draft |
| LockTimeout | Retry after another instance finishes saving. Lock wait is approximately five seconds maximum |
| CorruptData | Preserve the document and follow the recovery guidance |
| UnsupportedSchema | Check application/data compatibility; do not manually change the schema number |
| StorageFailure | Check the reported path, write permissions, free space, and other applications holding files open |
| CommitUncertain / exit code 3 | The change may already be saved. Reload and inspect before retrying |

When the save result is uncertain, TUI updates are disabled. Press `r`, then inspect the list or details. Reloading does not automatically resend a draft. In the CLI use `ls --all` to check the result; take particular care not to duplicate an addition. If reloading also fails, preserve the error and storage files for diagnosis.

## Shell completion

Generate and load completion into the current shell using the matching example below. Do not run examples for other shells.

```powershell
# PowerShell
tque completion powershell | Out-String | Invoke-Expression
```

```sh
# Bash
source <(tque completion bash)
```

```sh
# Zsh
autoload -Uz compinit
compinit
source <(tque completion zsh)
```

```fish
# Fish
tque completion fish | source
```

Dynamic task-ID completion is not supported. Profiles are not modified automatically. To enable completion permanently, load it through your shell's startup configuration.
