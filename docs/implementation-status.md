# 実装・検証記録

更新日: 2026-10-21。初回バージョン0.1.0。0.2.0で採番方法を最大ID+1へ変更し、保存形式をschema_version 2とした（0.1.0のデータとの互換性なし）。

## 実装した範囲

- model/engine: ID非再利用、revision、親子と依存の別々のDAG検査、深い階層の明示スタック処理、手動／自動進捗、完了状態、参照付き削除の保護。
- config/storage: 厳密JSON、必須・未知・重複・null検査、保存先解決、読取時の非作成、OSロック、更新の一括検証、1世代バックアップ、同期・置換・保存結果不明の区別。
- CLI: add/edit/done/undone/rm/ls、全属性フラグ、検索・期日・着手可能フィルタ、兄弟単位のソート、ツリー・JSON出力、4シェルの補完生成。
- TUI: 一覧＋下部詳細、独立した通知欄、全文詳細、検索・折り畳み、属性入力、削除確認、ヘルプ、非同期保存、競合時の入力保持、保存結果不明時の再送抑止。
- ビルド: 3 OS×2 CPU用ビルド、Windows Inno Setup定義、署名呼出し、Unixアーカイブ作成、依存ライセンス収集、CI定義。

アプリ実装後のコード・英日README・英日操作マニュアルはClaude Codeのレビューを実施済み（2026-10-21）。[回答原文](claude-implementation-review.md)と[依頼・完了記録](implementation-review-request.md)を参照。C01〜C12は中2件・低10件、文書関連D1〜D13は13件。[対応記録](implementation-review-response.md)に採否・修正・検証結果を記録。修正後の再レビューは未実施。Antigravityの既存レビューは設計・画面案に対するもので、実装後レビューは未依頼。

## 今回の検証

| 項目 | 結果と範囲 |
| --- | --- |
| Windows amd64 / Go 1.27.3 | PowerShellからgo test ./...、go vet ./...成功 |
| Linux amd64 / Go 1.27.3 | WSL2 Ubuntu（kernel 6.18.33.2-microsoft-standard-WSL2）でgo test ./...、go vet ./...、go test -race ./...成功 |
| ビルド | windows/darwin/linux × amd64/arm64の6種類すべて成功。macOS・ARM64バイナリは実行未確認 |
| CLI統合 | 日本語、全操作、JSON、検索ツリー、none、使用方法エラー、壊れた設定でもhelp/version/completionが動作することを確認 |
| 保存障害 | 書込・Sync・Close・置換前・置換後の障害注入。本体保護、バックアップ、CommitUncertainを確認 |
| 並行更新 | 4子プロセス×8追加で32件・revision32・next_id33を確認。ロック待機中断、所有プロセス強制終了後の再取得を確認 |
| Windows固有 | 削除共有を許可しないハンドルで本体を開いた状態の置換失敗と、本体バイト保持を確認 |
| JSONファジング | Windowsで約6秒、348,847入力、失敗なし。網羅性や安全性の証明ではない |
| TUI状態遷移 | 検索の文字入力、取消、全文詳細、追加、削除Enter取消、保存中連打、終了保留、競合時の入力案保持、結果不明後の再読込を自動試験 |
| 描画 | 120×30、80×24、60×14、40×10、20×5で列幅・行数を検査。長い日本語タイトル・絵文字の全文閲覧と入力欄を確認 |
| Windows端末セッション | ConPTYで日本語文字列の追加、Enter詳細、Esc復帰、Space完了、保存成功後の終了、JSONとの一致、代替画面からの復帰を確認 |
| Linux端末セッション | WSLのPTYで日本語文字列の追加、ヘルプ表示、終了・代替画面からの復帰を確認 |
| スクリプト | PowerShell構文解析、Windows/Unixビルド実行、依存ライセンス収集を確認。Inno Setupは/O-でコンパイル成功（検証用ファイル、配布パッケージ出力なし） |

端末への日本語文字列送信はIMEでの変換操作の検証ではない。ネイティブ端末アプリ・フォント・モニター上の視認性、実際のウィンドウリサイズは別途確認が必要。

## APIと依存の採用根拠

- [Go公式ダウンロード情報](https://go.dev/dl/?mode=json)で安定版Go 1.27.3を確認。toolchainに固定。Linuxツールチェーンの取得時に公開SHA256を照合した。システムのGoインストールは更新していない。
- [Bubble Tea v2.1.0](https://github.com/charmbracelet/bubbletea/tree/v2.1.0)（MIT）、[Lip Gloss v2.0.6](https://github.com/charmbracelet/lipgloss/tree/v2.0.6)（MIT）、[Cobra v1.10.2](https://github.com/spf13/cobra/tree/v1.10.2)（Apache-2.0）。ソースとgo.modでAPI・Go要件を確認しビルドした。
- Windowsは[LockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex)の排他・非ブロッキング取得、UnlockFileExを使用。Linux/macOSはgolang.org/x/sys/unix.Flockを使用。
- Windows置換は[MoveFileExW](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-movefileexw)のREPLACE_EXISTINGとWRITE_THROUGH。Unixはos.Renameと親ディレクトリSync。全OSで本体を先に削除しない。置換APIがエラーを返した場合は保守的にCommitUncertainとし、再試行しない。
- GoのSync、WRITE_THROUGH、ディレクトリSyncは電源断耐性の実測ではない。特にWindowsのすべてのファイルシステム上の原子的可視性・永続性を保証したとは扱わない。全プロセスが本アプリのロック規約に従うローカル保存が前提。
- [Inno Setup SignTool](https://jrsoftware.org/ishelp/topic_setup_signtool.htm)と[SignedUninstaller](https://jrsoftware.org/ishelp/topic_setup_signeduninstaller.htm)に従い、-Sign時に実行ファイルを先に署名し、Innoからインストーラー・アンインストーラー署名を呼び出す。
- 推移依存のライセンスはMIT、BSD系、Apache-2.0等の原文を収集した。GoランタイムのLICENSEと、モジュールのLICENSE/COPYING/NOTICEを配布時に同梱する。収集対象はテスト用等を含むモジュールグラフ全体であり、本体にリンクされる依存だけの一覧ではない。GPLアプリからのコード移植は行っていない。

## 配布前の残項目

- 著作権者名は`fukuyori`に確定し、正式なMITのLICENSEを作成済み。LICENSE未確定によるパッケージ作成の停止条件は解消。
- CODESIGN_CERTはwtmuxの署名スクリプトを確認し、証明書サブジェクト名（signtool /n）を既定に修正。拇印は-CertificateSelector Thumbprintで明示する。SDKのSignTool検出と既定タイムスタンプURLもwtmuxの方式に合わせた。PowerShell 5.1／7でSDK検出、模擬署名ツールへの引数・署名後のverify呼出し、空値／不正な拇印の拒否を確認。実証明書による3成果物の署名・検証は未実施。
- インストール・更新・アンインストール、PATH追加実績の継承・除去は実環境で未実施。コンパイル成功と導入試験を区別する。
- macOSでの実行、macOS署名・公証、ARM64実機、OS最低版の具体的な保証は未検証。
- Git初期化・初回コミット0e438eb・mainへのpushは完了。既存CIは[実行39524940074](https://github.com/fukuyori/tque/actions/runs/39524940074)で3 OSテストと6対象のpackageジョブが成功。
- 10,000件・深さ1,000・依存20,000辺の各OSにおけるp95性能受入は未実施。深さ1,000の機能試験と性能受入を区別する。
- 4シェルの補完生成は確認済み。各シェルでの実際の補完候補表示は未確認。
- Unicodeの追加制限・具体配色・通知の高度な履歴機能等の再検討は保留を維持。今回確認した操作の範囲は詳細設計第7版を参照。

## 成果物

### タグpushによるRelease登録（2026-10-21追記）

- VERSIONと同じ名前のタグ（`v`なし）をpushすると、3 OSテストと6対象のパッケージ成功後にRelease公開ジョブを実行。通常のブランチpush／PR／手動実行はArtifacts作成のみ。
- Releaseジョブだけにcontents: writeを付与。同じ実行の6対象のZIPとSHA256を取得し、版・ファイルの存在・SHA256・ZIP整合性を検査してから、gh release create --verify-tag --generate-notesで公開。既存Releaseは上書きしない。
- actionlint v1.7.12とBash構文検査成功。検証用ZIPと模擬ghで12添付の引数を確認。タグ不一致・添付不足・チェックサム不一致・壊れたZIPでghを呼ばず停止することを確認。ネットワークへの公開操作は行っていない。
- 署名修正を含む作業ツリーの変更は未コミット・未push。以前のCI成功は初回コミットに対する結果で、今回の変更の成功を示すものではない。

### 本体LICENSEの確定（2026-10-21追記）

利用者の指定に従い、`Copyright (c) 2026 fukuyori`を記載したMITの`LICENSE`を作成。MIT本文は既存の雛形を維持し、依存ライブラリの原文・著作権表示は変更していない。以下の取得・ZIP検証記録にある「未確定」は各作業時点の状態を示す。この変更ではパッケージ作成・署名・公開は行っていない。

### 依存ライセンス原文の取得（2026-10-21追記）

- `third-party-licenses/`に29外部モジュールとGo 1.27.3ランタイムのLICENSE・NOTICE等33ファイルを保存。テスト用等を含むモジュールグラフ全体が対象で、本体にリンクされる依存だけの一覧ではない。
- 各原文に著作権表示を保持。README.mdにファイル一覧、SOURCES.jsonに元モジュール・版・相対パス・SHA256を記録。`go mod verify`成功、全33ファイルが取得元とバイト一致。
- 既存収集処理の探索範囲をサブディレクトリへ拡張。従来の直下探索で漏れていた`github.com/clipperhouse/stringish/utf8/LICENSE`を追加し、配置を保持する。PowerShell／Bash両方の実行結果33ファイルのSHA256一致を確認。
- tque本体のLICENSE名義確定、参照コードの全面的な出自監査とは別の作業。本体LICENSEは作成していない。

### GitHub ActionsのZIP作成・英日ドキュメント（2026-10-21追記）

- `.github/workflows/ci.yml`を3 OSテスト後の6対象ZIP作成・Artifact保存に変更。バージョンはVERSION、Go版はgo.modから取得。CIの本体ビルド先はランナーの一時ディレクトリで、リポジトリのdistには出力しない。
- `scripts/package-zip.sh`は`tque-バージョン-OS-アーキテクチャ.zip`を作成。実行ファイル、英日README、英日操作マニュアルを含むdocs、正式LICENSE、依存ライセンスを同梱する。
- README.mdとdocs/user-manual.mdは英語、README.ja.mdとdocs/user-manual.ja.mdは日本語。言語間リンクとローカル参照・見出し・コードフェンスを検査済み。既存のInno／tar.gz同梱処理にも両言語の文書を追加。
- WSL上の検証用コピーで6種類のZIP作成、ZIP整合性、必要ファイル、実行ファイル形式、Unix実行権限を確認。展開したLinux amd64版の`--version`成功。正式LICENSEは未確定のため、検証コピーだけに配布禁止を明記したテスト用LICENSEを使用。生成した検証ZIPは配布用ではない。
- LICENSEなしでの停止と既存ZIPの上書き拒否を確認。actionlint v1.7.12（shellcheck呼出しなし）でワークフロー静的検査成功。Bash／PowerShell構文検査、Innoのamd64／arm64定義の出力なしコンパイル成功。
- GitHub上での実行・Artifact取得は未確認。正式LICENSEを用意するまでCIのZIP作成は停止する。macOS・ARM64の実機動作、署名・公開を検証したものではない。

### 初回実装時の成果物

`dist/tque-windows-amd64.exe`などに6対象の開発確認用実行ファイルを作成した。未署名・未公開。`.work`は一時データ・ツールチェーン・描画記録・コンパイル検証用ファイルで、配布対象ではない。実ユーザーのタスクデータは使用していない。
