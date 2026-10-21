# バージョン更新チェックリスト

- `VERSION`: アプリの版番号を定義する唯一のファイル。main.goがgo:embedで取り込み、インストーラースクリプトも同じファイルを読む。
- `CHANGELOG.md`: 対応する変更履歴とリリース状態を更新する。
- `README.md`、`README.ja.md`、`docs/implementation-status.md`: 掲載する例と検証結果を確認する。過去の設計・レビュー記録の日付は書き換えない。
- `docs/user-manual.md`、`docs/user-manual.ja.md`: 英語・日本語両方の対象バージョン、コマンド・キー操作、設定と保存・復旧の説明を実装と照合する。
- `.github/workflows/ci.yml`、`scripts/package-zip.sh`: ZIP名にVERSION・OS・アーキテクチャが反映され、英語・日本語のREADMEと操作マニュアルが同梱されることを確認する。
- `scripts/publish-release.sh`: タグを`v`+VERSIONと一致させる。タグpushで3 OSテスト・6対象のパッケージ作成後、ReleaseにZIP 6個・SHA256 6個が登録されたことを確認する。既存Releaseは上書きしない。
- `go.mod`、`go.sum`: Goまたは依存更新時のみ更新。`toolchain`を更新し、`.github/workflows/ci.yml`が`go-version-file: go.mod`でその版を取得することを確認する。
- `LICENSE`: 著作権者は`fukuyori`。著作権年は実際の状況に合わせて確認する。依存ライブラリの著作権表示は変更しない。
- `scripts/build-release.ps1` / `.sh`、`scripts/build-installer.ps1`、`installer/tque.iss`: 版番号を手動埋込みせずVERSIONを使用することを確認する。
- ビルド後の`--version`、Inno SetupのAppVersion、配布ファイル名を確認する。
- 対象OS/CPUのネイティブ実行、端末操作、署名、インストール・アンインストールを個別に確認する。
- パッケージのthird-party-licensesとチェックサムを更新する。ZIPの`.zip.sha256`も照合する。コード署名とmacOS公証を混同しない。

コミット・タグ作成・プッシュ・公開は明示的な依頼がある場合のみ行う。
