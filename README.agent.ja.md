<h1 align='center'>pst-agent 配備します</h1>

<p align="center">
   <a href="/README.agent.md">简体中文</a> | <a href="/README.agent.en.md">English</a> | <strong>日本語</strong>
</p>

### Linux

これは Linux ゲームサーバーとは別の場所に PST 本体を配置する構成です。PST 本体は通常の[インストールとデプロイメント](./README.ja.md#インストールとデプロイメント)に従い、pst-agent の同期 URL だけを PST のデータベース設定へ保存します。

#### ダウンロード

pst-agent ツールをダウンロードし、名前を変更して実行可能にします。

```bash
# ダウンロードして名前を変更
mv pst-agent_v0.10.0_linux_x86_64 pst-agent
chmod +x pst-agent
```

#### 実行

```bash
# ./pst-agent --port 8081 -d {Level.sav の絶対パス}
# 例：
./pst-agent --port 8081 -d /home/lighthouse/game/Saved
```

正常に動作することを確認した後、バックグラウンドで実行します（ssh ウィンドウを閉じた後も実行を続けます）。

```bash
# バックグラウンドで実行し、ログを agent.log に保存
nohup ./pst-agent --port 8081 -f ...{省略}.../Saved > agent.log 2>&1 &
# ログを確認
tail -f agent.log
```

#### ファイアウォール/セキュリティグループを開放

pst-agent と pst 本体が同じネットワークグループ内に完全に存在しない場合、ゲームサーバーの該当する公開ポート（例えば 8081、またはカスタマイズされた他のポートも可能）を開放する必要があります。

#### 設定

**PST 本体（pst-agent ではありません）**の Web 管理モードへログインし、Swagger から `PUT /api/setup/config` で `save.path` だけを更新します。

```json
{
  "values": {
    "save.path": "http://ゲームサーバーの公開IP:ポート/sync"
  }
}
```

値は `pst.db` に保存されます。新しいアドレスを読み込むには PST 本体を再起動してください。

#### バックグラウンド実行を停止

```bash
kill $(ps aux | grep 'pst-agent' | awk '{print $2}') | head -n 1
```

### Windows

これは Windows ゲームサーバーとは別の場所に PST 本体を配置する構成です。PST 本体は通常の[インストールとデプロイメント](./README.ja.md#インストールとデプロイメント)に従い、pst-agent の同期 URL だけを PST のデータベース設定へ保存します。

#### ダウンロード

pst-agent ツールをダウンロードし、名前を変更します。例えば、`pst-agent_v0.10.0_windows_x86_64.exe`を`pst-agent.exe`にリネームします。

#### 実行

`Win + R`を押し、`powershell`を入力して Powershell を開き、`cd`コマンドでダウンロードした実行ファイルのディレクトリに移動します。

```powershell
# .\pst-agent.exe --port アクセスポート -d セーブファイル Level.sav の場所
.\pst-agent.exe --port 8081 -d C:\Users\ZaiGie\...\Pal\Saved
```

正常に実行されたら、ウィンドウを開いたままにしてください。

#### 設定

**PST 本体（pst-agent ではありません）**の Web 管理モードへログインし、Swagger から `PUT /api/setup/config` で `save.path` だけを更新します。

```json
{
  "values": {
    "save.path": "http://ゲームサーバーの公開IP:ポート/sync"
  }
}
```

値は `pst.db` に保存されます。新しいアドレスを読み込むには PST 本体を再起動してください。
