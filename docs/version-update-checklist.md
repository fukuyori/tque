# バージョン更新チェックリスト

- `VERSION`: アプリの版番号を定義する唯一のファイル。main.goがgo:embedで取り込み、インストーラースクリプトも同じファイルを読む。
- `CHANGELOG.md`: 対応する変更履歴を追加する。リリース済みか否かなど、後で古くなる状態は書かない。
- `README.md`、`README.ja.md`、`docs/implementation-status.md`: 掲載する例と検証結果を確認する。過去の設計・レビュー記録の日付は書き換えない。
- `docs/user-manual.md`、`docs/user-manual.ja.md`: 英語・日本語両方の対象バージョン、コマンド・キー操作、設定と保存・復旧の説明を実装と照合する。
- `.github/workflows/ci.yml`、`scripts/package-zip.sh`: ZIP名にVERSION・OS・アーキテクチャが反映され、英語・日本語のREADMEと操作マニュアルが同梱されることを確認する。
- `scripts/publish-release.sh`: タグ名はVERSIONと同じ（`v`を付けない）。タグpushで3 OSテスト・6対象のパッケージ作成後、ReleaseにZIP 6個・SHA256 6個が登録されたことを確認する。既存Releaseは上書きしない。
- `go.mod`、`go.sum`: Goまたは依存更新時のみ更新。`toolchain`を更新し、`.github/workflows/ci.yml`が`go-version-file: go.mod`でその版を取得することを確認する。
- `LICENSE`: 著作権者は`fukuyori`。著作権年は実際の状況に合わせて確認する。依存ライブラリの著作権表示は変更しない。
- `scripts/build-release.ps1` / `.sh`、`scripts/build-installer.ps1`、`installer/tque.iss`: 版番号を手動埋込みせずVERSIONを使用することを確認する。
- ビルド後の`--version`、Inno SetupのAppVersion、配布ファイル名を確認する。
- 対象OS/CPUのネイティブ実行、端末操作、署名、インストール・アンインストールを個別に確認する。
- パッケージのthird-party-licensesとチェックサムを更新する。ZIPの`.zip.sha256`も照合する。コード署名とmacOS公証を混同しない。

## 配布ライセンスの確認

- ZIP・tar.gz・Inno Setupの各作成スクリプトは、配布する実行ファイルとOS・CPUを `scripts/collect-licenses.sh` / `.ps1` に渡す。共通処理は `scripts/licenses` にある。
- ビルドと同じOS・CPU、`CGO_ENABLED=0` で取得した依存モジュールと、実行ファイル内のモジュール名・バージョンが一致することを確認する。実行ファイルのOS・CPU・Goバージョンも照合し、不一致はエラーにする。
- 対象モジュールの `LICENSE*`・`COPYING*`・`NOTICE*`・`COPYRIGHT*` は、サブフォルダーを含め原文のまま同梱する。ライセンスが見つからない場合、空の場合、読み取りに失敗した場合は作成を停止する。
- `GO-LICENSE` はビルドと同じバージョンのGoから取得する。`MODULES.txt` のモジュール名・バージョンと同梱フォルダーを照合する。
- 収集先は毎回新規に作成する。既存ディレクトリは拒否し、古いライセンスが混入しないようにする。リポジトリ内の `third-party-licenses/` は開発用の記録であり、配布物へ丸ごとコピーしない。
- Windows・macOS・Linuxのamd64／arm64の6対象を確認する。除外するモジュール名や個数は固定せず、依存更新のたびに実際のビルド対象から決める。
- カスタムビルドタグやモジュール置換は現状では拒否する。採用する際には依存抽出条件とライセンス収集元を見直す。
- 収集・照合に失敗した場合はパッケージ作成を停止し、公開しない。実際のアーカイブやインストーラーの同梱内容も確認する。

コミット・タグ作成・プッシュ・公開は明示的な依頼がある場合のみ行う。
