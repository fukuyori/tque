# Third-party license inventory

Collected on 2026-10-21 from the exact module versions selected by `go.mod` and the Go 1.27.3 toolchain. This directory contains upstream license and attribution files copied without changing their bytes. Copyright notices remain in the linked originals, including notices for components that use licenses other than MIT.

Scope: 29 external modules in the module graph, including test/tool dependencies, plus the Go runtime license. This is broader than the libraries linked into a particular target executable. It is not a source-code provenance audit or a statement that every file uses MIT.

- [Module version list](MODULES.txt)
- [Source paths and SHA256 checksums](SOURCES.json)
- [Go runtime license](GO-LICENSE)

`SOURCES.json` records source paths relative to each downloaded module root; no developer-specific cache paths are included. Each copied file was verified against that source. `go mod verify` also passed at collection time.

The project-level [MIT license for tque](../LICENSE) is separate from this collection and identifies `fukuyori` as the copyright holder. Third-party copyright notices and license terms remain unchanged.

## Original notices and license texts

| Module | Version | Files |
| --- | --- | --- |
| charm.land/bubbletea/v2 | v2.1.0 | [LICENSE](charm.land_bubbletea_v2@v2.1.0/LICENSE) |
| charm.land/lipgloss/v2 | v2.0.6 | [LICENSE](charm.land_lipgloss_v2@v2.0.6/LICENSE) |
| github.com/aymanbagabas/go-udiff | v0.4.1 | [LICENSE-BSD](github.com_aymanbagabas_go-udiff@v0.4.1/LICENSE-BSD), [LICENSE-MIT](github.com_aymanbagabas_go-udiff@v0.4.1/LICENSE-MIT) |
| github.com/bits-and-blooms/bitset | v1.24.6 | [LICENSE](github.com_bits-and-blooms_bitset@v1.24.6/LICENSE) |
| github.com/charmbracelet/colorprofile | v0.4.3 | [LICENSE](github.com_charmbracelet_colorprofile@v0.4.3/LICENSE) |
| github.com/charmbracelet/ultraviolet | v0.0.0-20261008173134-6b8d4baf91b4 | [LICENSE](github.com_charmbracelet_ultraviolet@v0.0.0-20261008173134-6b8d4baf91b4/LICENSE) |
| github.com/charmbracelet/x/ansi | v0.11.9 | [LICENSE](github.com_charmbracelet_x_ansi@v0.11.9/LICENSE) |
| github.com/charmbracelet/x/exp/golden | v0.0.0-20250806222409-83e3a29d542f | [LICENSE](github.com_charmbracelet_x_exp_golden@v0.0.0-20250806222409-83e3a29d542f/LICENSE) |
| github.com/charmbracelet/x/term | v0.2.2 | [LICENSE](github.com_charmbracelet_x_term@v0.2.2/LICENSE) |
| github.com/charmbracelet/x/termios | v0.1.1 | [LICENSE](github.com_charmbracelet_x_termios@v0.1.1/LICENSE) |
| github.com/charmbracelet/x/windows | v0.2.2 | [LICENSE](github.com_charmbracelet_x_windows@v0.2.2/LICENSE) |
| github.com/clipperhouse/displaywidth | v0.11.0 | [LICENSE](github.com_clipperhouse_displaywidth@v0.11.0/LICENSE) |
| github.com/clipperhouse/stringish | v0.1.1 | [LICENSE](github.com_clipperhouse_stringish@v0.1.1/LICENSE), [utf8/LICENSE](github.com_clipperhouse_stringish@v0.1.1/utf8/LICENSE) |
| github.com/clipperhouse/uax29/v2 | v2.7.0 | [LICENSE](github.com_clipperhouse_uax29_v2@v2.7.0/LICENSE) |
| github.com/cpuguy83/go-md2man/v2 | v2.0.6 | [LICENSE.md](github.com_cpuguy83_go-md2man_v2@v2.0.6/LICENSE.md) |
| github.com/inconshreveable/mousetrap | v1.1.0 | [LICENSE](github.com_inconshreveable_mousetrap@v1.1.0/LICENSE) |
| github.com/lucasb-eyer/go-colorful | v1.4.1 | [LICENSE](github.com_lucasb-eyer_go-colorful@v1.4.1/LICENSE) |
| github.com/mattn/go-runewidth | v0.0.30 | [LICENSE](github.com_mattn_go-runewidth@v0.0.30/LICENSE) |
| github.com/muesli/cancelreader | v0.2.2 | [LICENSE](github.com_muesli_cancelreader@v0.2.2/LICENSE) |
| github.com/rivo/uniseg | v0.4.7 | [LICENSE.txt](github.com_rivo_uniseg@v0.4.7/LICENSE.txt) |
| github.com/russross/blackfriday/v2 | v2.1.0 | [LICENSE.txt](github.com_russross_blackfriday_v2@v2.1.0/LICENSE.txt) |
| github.com/spf13/cobra | v1.10.2 | [LICENSE.txt](github.com_spf13_cobra@v1.10.2/LICENSE.txt) |
| github.com/spf13/pflag | v1.0.9 | [LICENSE](github.com_spf13_pflag@v1.0.9/LICENSE) |
| github.com/xo/terminfo | v1.0.0 | [LICENSE](github.com_xo_terminfo@v1.0.0/LICENSE) |
| go.yaml.in/yaml/v3 | v3.0.4 | [LICENSE](go.yaml.in_yaml_v3@v3.0.4/LICENSE), [NOTICE](go.yaml.in_yaml_v3@v3.0.4/NOTICE) |
| golang.org/x/exp | v0.0.0-20231006140011-7918f672742d | [LICENSE](golang.org_x_exp@v0.0.0-20231006140011-7918f672742d/LICENSE) |
| golang.org/x/sync | v0.22.0 | [LICENSE](golang.org_x_sync@v0.22.0/LICENSE) |
| golang.org/x/sys | v0.47.0 | [LICENSE](golang.org_x_sys@v0.47.0/LICENSE) |
| gopkg.in/check.v1 | v0.0.0-20161208181325-20d25e280405 | [LICENSE](gopkg.in_check.v1@v0.0.0-20161208181325-20d25e280405/LICENSE) |

