# Bash sweep除外設定をconfigで管理する

## 目的

現在 `--include-bash-tool` がopt-inなのは、Bashエントリに
まだ存在しないパスへの意図的な許可設定が含まれうるため
（例: `mkdir`, `touch` 等のcreate系コマンドの出力先パス）。

configで除外パターンを指定できるようにすれば、
`--include-bash-tool` をより安全に利用でき、
将来的にデフォルト有効化への道も開ける。

## 変更内容

- cctidy用の設定ファイルを導入する
  （例: `.cctidy.toml` またはCLIフラグ経由）
- Bash sweep時に除外するエントリのパターンを設定可能にする
  - 完全一致（例: `Bash(mkdir -p /path/to/dir)`）
  - コマンド名ベース（例: `mkdir`, `touch`）
  - パスパターン（例: `/path/to/output/**`）
- BashToolSweeperに除外ロジックを追加する
- configが存在しない場合は現状と同じ挙動を維持する

## 対象ファイル

- @sweep.go（BashToolSweeper に除外ロジック追加）
- @cmd/cctidy/main.go（config読み込み・CLIフラグ追加）
- 新規: config定義ファイル（形式は要検討）

## 完了条件

- [ ] 除外パターンの設定形式が決定されている
- [ ] BashToolSweeperが除外パターンに基づきエントリを保持する
- [ ] configが存在しない場合は既存の挙動と互換性がある
- [ ] テストで除外パターンの動作が検証されている
- [ ] docs/reference/permission-sweeping.md に除外設定の
      説明が追加されている
