<h1 align='center'>幻獣パルサーバー管理ツール</h1>

<p align="center">
   <a href="/README.md">简体中文</a> | <a href="/README.en.md">English</a> | <strong>日本語</strong>
</p>

<p align="center">
  <img src="./web/src/assets/app-icon.png" width="112" alt="Palworld Server Tool アイコン" />
</p>

<p align="center">
  <a href="https://github.com/xutongxue233/palworld-server-tool/releases/latest">最新版をダウンロード</a> · <a href="./CHANGELOG.md">更新履歴</a>
</p>

<p align='center'>
  React インターフェース、公式 REST API、最新の SAV 解析で Palworld 専用サーバーを管理します。<br/>
  そして、国際化のために長くて退屈な時間を費やしました...
</p>

<p align='center'>
<img alt="GitHub Release" src="https://img.shields.io/github/v/release/xutongxue233/palworld-server-tool?style=for-the-badge">&nbsp;&nbsp;
<img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white">&nbsp;&nbsp;
<img alt="Python" src="https://img.shields.io/badge/Python-FFD43B?style=for-the-badge&logo=python&logoColor=blue">&nbsp;&nbsp;
<img alt="React" src="https://img.shields.io/badge/React-20232A?style=for-the-badge&logo=react&logoColor=61DAFB">
</p>

![PC](./docs/img/pst-ja-1.png)

> 現在、モバイル端末にも良好に対応しており、下記の [機能スクリーンショット](#機能スクリーンショット) をご覧ください。
>
> もちろん、ダークモードもバッチリです～

`Level.sav`ファイルの解析を基にした機能：

- [x] 完全なプレイヤーデータ
- [x] プレイヤーのパルデータ
- [x] ギルドデータ
- [x] プレイヤーが荷物のデータをリュックします

公式 REST API を使って実装した機能：

- [x] サーバー情報の取得
- [x] サーバ復号メトリックを取得
- [x] オンラインプレイヤーリスト
- [x] プレイヤーのキック/バン
- [x] ゲーム内ブロードキャスト
- [x] サーバーのスムーズなシャットダウンとメッセージのブロードキャスト

ツールが追加で提供する機能：

- [x] 可視化マップ管理です
- [x] ホワイトリスト管理
- [x] ゲーム標準の `backup/world` の検出、検証、安全な復元
- [x] `PalWorldSettings.ini` の直接編集と WorldOption の検出、安全な生成・同期
- [x] process、Docker、systemd、Windows サービスの制限付き起動・停止・再起動
- [x] 認証付き RCON 端末と Palworld 1.0.0 公式 13 コマンドのテンプレート
- [x] 型付きスケジュールタスク、意図的停止を尊重するウォッチドッグ、汎用/Discord Webhook 通知
- [x] 固定 App ID 2394010 の SteamCMD インストール、更新、ファイル検証、任意の再起動
- [x] Windows 公式 1.0.0 MOD ローダーの状態確認、依存関係チェック、`PalModSettings.ini` の安全な編集とロールバック
- [x] 1 サーバー 1 PST のノード集約、全管理画面の切り替え、制限付き認証プロキシ
- [x] 読み取り専用チェック、必須復元点、原子置換、ロールバックを備えた同一 OS 専用サーバーセーブ移行
- [x] 危険なセーブ操作前の PST セーフティ復元ポイント

このツールは公式 REST API と Level.sav の同期データを bbolt に保存し、管理画面から利用できます。

メンテナンスと開発のスタッフが少ないため、意欲はありますが、力不足です。フロントエンド、バックエンド、データエンジニアの皆さんからの PR を歓迎します！

## 機能スクリーンショット

https://github.com/zaigie/palworld-server-tool/assets/17232619/afdf485c-4b34-491d-9c1f-1eb82e8060a1

### デスクトップ

|                              |                              |
| :--------------------------: | :--------------------------: |
| ![](./docs/img/pst-ja-2.png) | ![](./docs/img/pst-ja-3.png) |

![](./docs/img/pst-ja-4.png)

### モバイル

<p align="center">
<img src="./docs/img/pst-ja-m-1.png" width="30%" /><img src="./docs/img/pst-ja-m-2.png" width="30%" /><img src="./docs/img/pst-ja-m-3.png" width="30%" />
</p>

## REST API を有効にする

オンラインプレイヤーの同期とサーバー管理操作には、公式 REST API を有効にする必要があります。

設定ページから `PalWorldSettings.ini` を直接読み込み、検証して書き戻せます。`WorldOption.sav` がある場合は上書き関係を表示し、サーバー停止、完全バックアップ、無損失検証の後に保存済みの 1.0.0 設定を安全に同期できます。ファイルがない場合は、チェックサム固定の 1.0.0 ベースから生成できます。

最初に**管理者パスワード**を設定します

![ADMIN](./docs/img/admin-ja.png)

次に **REST API** を有効にします

## 自動化、ウォッチドッグ、通知

Web 管理モードの **運用 → 自動化** から、Cron を書かずに一定間隔・毎日・毎週のタスクを作成できます。操作はワールド保存、アナウンス、起動、安全停止、保存して再起動、セーブ解析同期、追加 PST バックアップに限定され、Shell や任意 RCON コマンドには変換されません。タスク、最新 500 件の結果、設定は `pst.db` に保存されます。

ウォッチドッグは制限付き制御ドライバーのプロセス状態と Palworld REST `/info` の両方を確認します。連続失敗しきい値、起動猶予、クールダウン、最大復旧回数を備えています。Web または型付きタスクで手動停止した場合は「停止を許可」として永続化され、意図に反して再起動しません。

オフラインセーブ編集、設定書き込み、RCON、従来の定期同期/バックアップ、自動メンテナンスは同じ操作ロックを共有します。RCON の `Shutdown` と `DoExit` も意図的停止として記録され、ウォッチドッグが管理者コマンドと競合しません。

通知は汎用 JSON Webhook と Discord Webhook に対応し、タスク、手動起動停止、異常、復旧イベントを選択できます。汎用 Webhook は `X-PST-Signature: sha256=<HMAC>` を検証できます。既定では公開 HTTPS のみを許可し、リダイレクト、localhost、プライベートネットワークを拒否します。Webhook トークンと署名シークレットは読み取り API に表示されません。

初回値は `config.yaml` の `automation.watchdog` と `automation.notification` から読み込み、その後 Web UI で保存できます。ウォッチドッグには `palworld.control` が必要です。設計では [palworld-server-docker](https://github.com/thijsvanloef/palworld-server-docker) と [TRRabbit Palworld Server Manager](https://github.com/TRRabbit/palworld-server-manager) の運用 UX を参考にしつつ、PST 独自の許可リスト操作、排他制御、送信専用通知を維持しています。

```yaml
automation:
  watchdog:
    enabled: false
    desired_running: true
    check_interval_seconds: 30
    failure_threshold: 3
    restart_cooldown_seconds: 120
    max_recovery_attempts: 3
    startup_grace_seconds: 90
  notification:
    enabled: false
    provider: "generic" # generic または discord
    webhook_url: ""
    secret: "" # 汎用 Webhook の任意 HMAC-SHA256 キー
    events: ["task.failed", "watchdog.unhealthy", "watchdog.recovered"]
    timeout_seconds: 10
    allow_private_network: false
```

## マルチサーバーノード

PST 1.8.0 は、複数の Palworld 1.0.0 サーバーを **1 サーバー 1 PST ノード + 中央コントローラー**として管理します。各 PalServer は独立した PST プロセス、`config.yaml`、`pst.db`、セーブ、バックアップ、スケジューラー、ウォッチドッグ、メンテナンスロックを持ちます。中央 PST は状態を集約し、固定許可リストの既存 API だけを中継します。単一プロセス内でグローバル設定を切り替えないため、別ワールドの処理が混ざらず、危険操作は対象ノード自身のロックと復旧トランザクションで保護されます。

同じホストで複数ノードを実行する場合、ノードごとに**異なる PST 作業ディレクトリ**と `web.port` を使用してください。たとえば `pst-primary/` と `pst-second/` に実行ファイル、設定、DB を個別配置し、REST、セーブ、制御、SteamCMD、MOD の各パスを対応する PalServer だけに向けます。2 つの PST プロセスで `pst.db`、バックアップディレクトリ、同じワールドセーブを共有しないでください。

管理されるノードに識別子と 32 文字以上のランダムな受信用トークンを設定します。

```yaml
web:
  port: 8081

fleet:
  node_id: "second"
  node_name: "第 2 ワールド"
  node_token: "replace-with-at-least-32-random-characters"
  nodes: []
```

中央 PST の `fleet` にノードを追加します。`id` はリモート `node_id`、`token` はリモート `node_token` と一致させます。

```yaml
fleet:
  node_id: "primary"
  node_name: "メインワールド"
  node_token: "optional-token-if-this-node-is-also-controlled-remotely"
  timeout_seconds: 15
  nodes:
    - id: "second"
      name: "第 2 ワールド"
      base_url: "https://palworld-second.example.com:8081"
      token: "replace-with-at-least-32-random-characters"
      allow_private_network: false
      timeout_seconds: 15
```

中央 Web 管理モードへログインすると、ヘッダーの選択メニューと概要ノードレールに最大 32 ノードの接続状態、プレイヤー数、FPS、遅延、制御状態、設定問題が表示されます。ノードを選ぶと、概要、プレイヤー、ギルド、マップ、設定、RCON、バックアップ、自動化、導入更新、MOD、移行の各画面が独立キャッシュで対象サーバーへ切り替わります。書き込み処理中は切り替えを無効化し、完了処理が別ワールドへ適用されることを防ぎます。

公開ノードには有効な HTTPS が必要です。`allow_private_network: true` は信頼できるループバック、LAN、プライベートオーバーレイだけに設定し、通常 HTTP の危険も明示的に受け入れる設定です。ノードクライアントはシステムプロキシとリダイレクトを使わず、接続時にすべての DNS/IP を再検証し、検証済みアドレスを順番に試し、リクエスト全体の時間を制限します。中央プロキシはログイン、Fleet 再帰、未知のメソッド/パス、トラバーサル、8 MiB を超える本文を拒否します。ブラウザーが保持するのは中央 JWT だけで、リモートトークンは前端へ返らず、中央 JWT もリモートへ転送されません。

## SteamCMD インストール・更新

Palworld 1.0.0 専用サーバーの App ID は `2394010` です。Web 管理モードの **運用 → 導入・更新** から、公式の `app_update 2394010` 手順を実行でき、既定でファイル検証も行います。PST は第三者の「最新バージョン」API を使わず、Shell、任意の App ID、自由な SteamCMD 引数を受け付けません。

```yaml
steamcmd:
  # Windows は steamcmd.exe、Linux は steamcmd.sh または steamcmd。絶対パス必須。
  executable: "C:/steamcmd/steamcmd.exe"
  # Palworld Dedicated Server の導入先。ファイルシステムのルートは指定不可。
  install_dir: "D:/PalworldServer"
  # 1 回の導入・更新の上限時間。60～7200 秒。
  timeout: 1800
```

実行前に SteamCMD のハッシュ、導入先、`appmanifest_2394010.acf`、プラットフォーム用起動ファイル、プランダイジェストを再検証します。導入先にワールドがある場合、ローカル `save.path` はその導入先内のワールドを指す必要があります。PST はサーバーを停止して完全な復元点を必ず作成し、バックアップに失敗した場合は SteamCMD を起動しません。`palworld.control` があれば安全な停止と任意の再起動が可能です。未設定の場合はホスト側で PalServer を完全停止し、UI で明示確認してください。

[Palworld 1.0.0 公式 SteamCMD 導入ガイド](https://docs.palworldgame.com/getting-started/deploy-dedicated-server)も参照してください。

## 公式 MOD 管理（Windows）

Palworld 1.0.0 の公式サーバー MOD は、現在 **Windows Dedicated Server** のみ対応しています。Web 管理モードで **運用 → MOD 管理** を開くと、PST は `<PalServer>/Mods/Workshop/<任意のフォルダー>/Info.json` を走査し、パッケージ名、バージョン、作者、依存関係、導入種類、サーバー互換性を表示します。`<PalServer>/Mods/ManagedMods/<PackageName>/InstallManifest.json` から、導入済み、再起動待ち、削除待ちも判定します。

```yaml
mods:
  # 任意の Palworld Dedicated Server 絶対導入先。
  # 空の場合は steamcmd.install_dir を使用します。
  install_dir: "D:/PalworldServer"
```

PST は 1.0.0 公式形式の `WorkshopRootDir`、`bGlobalEnableMod`、複数の `ActiveModList` を扱い、起動引数の `-NoMods` と `-workshopdir` 上書きも検出します。`IsServer: true`、パッケージ内に収まる 1 件以上の導入先、公式 5 種類（UE4SS、Lua、PalSchema、LogicMods、Paks）のいずれかを持つパッケージだけをサーバー一覧に追加できます。依存 MOD の欠落、重複、無効化は適用を停止します。

PST は MOD 内容を**ダウンロード、展開、実行しません**。信頼できる公式形式パッケージを Workshop ディレクトリへ手動で配置してください。確定時はプランダイジェストを再検証してサーバーを停止します。既存ワールドがある場合、ローカル `save.path` は同じ導入先内のワールドを指す必要があります。完全な安全復元点を先に作成し、`PalModSettings.ini` をバックアップして原子的に置換・再検証し、必要なら再起動します。管理下の再起動に失敗した場合は失敗したプロセスを強制停止し、旧設定へ戻して旧構成での起動を試みます。サーバー MOD はクラッシュやセーブ破損を起こす可能性があるため、各パッケージを確認し、ゲーム標準バックアップを保持してください。

[Palworld 1.0.0 公式 MOD ガイド](https://docs.palworldgame.com/settings-and-operation/mod)と [PalworldModUploader 形式ドキュメント](https://github.com/pocketpairjp/PalworldModUploader)も参照してください。

## 安全なセーブ移行

Web 管理モードで **運用 → セーブ移行** を開き、旧サーバーのローカル絶対パスを入力します。`Level.sav`、特定ワールド、`Saved`、PalServer 導入ルートを指定できます。複数ワールドが見つかった場合は、ワールドディレクトリを直接選択してください。事前確認は読み取り専用で、重要ファイルをハッシュ化し、同梱 `sav_cli` で `Level.sav`、`LevelMeta.sav`、全プレイヤーセーブ、任意の `WorldOption.sav` が Palworld 1.0.0 の正しいセーブ種類か検証します。

自動移行は **同一 OS の専用サーバー → 現在の専用サーバー** に限定されます。次の場合は実験的変換を行わず停止します。

- 協力ホスト/シングルのセーブ、または `00000000000000000000000000000001.sav` のホストプレイヤーを検出した場合；
- プレイヤー ID/GUID の変更が必要な Windows と Linux 間の移行；
- リモート URL、Docker/Kubernetes ソース、シンボリックリンク、同一の移行元・移行先、無効な構造やセーブ種類。

実行時はメンテナンスロックを取得し、ウォッチドッグを一時停止し、サーバーを保存・停止して、現在のワールドの必須 PST 復元点を作成します。移行元は移行先と同じファイルシステムのトランザクション領域へコピーされます。移行元・移行先のダイジェストが変わらず、一時配置ファイルが再検証された場合だけ、`Level.sav`、`LevelMeta.sav`、`Players/`、任意の `WorldOption.sav` を原子的に置換します。現在のゲーム管理 `backup/` と未知のトップレベル項目は保持し、移行元の `backup/` は導入しません。移行元に `WorldOption.sav` がなければ、移行先の既存ファイルは削除されます。導入後の検証失敗時は自動ロールバックし、ロールバック自体が失敗した場合は `.pst-save-migration-*` に復旧ファイルを残してパスをエラーに表示します。`palworld.control` 設定時は任意に再起動でき、未設定時は手動停止を確認し、完了後に手動起動してください。


## インストールとデプロイメント

- [ファイルデプロイメント](#ファイルデプロイメント)
  - [Linux](#linux)
  - [Windows](#windows)
- [Docker デプロイメント](#docker-デプロイメント)
  - [単体デプロイメント](#単体デプロイメント)
  - [Agent デプロイメント](#agent-デプロイメント)
  - [k8s-pod からの存档同期](#k8s-pod-からの存档同期)
- [docker コンテナからの存档同期](#docker-コンテナからの存档同期)

> `Level.sav`ファイルの解析タスクは短時間（<20s）で一定量のシステムメモリ（1GB~3GB）を消費します。このメモリは解析タスク完了後に解放されるため、サーバーに十分なメモリがあることを確認してください。

ここでは、**pst ツールとゲームサーバーを同一物理マシン上に配置することをデフォルトとしています**。一部の状況では、それらを同一マシン上に配置したくない場合があります：

- 別のサーバーに単独でデプロイする必要がある
- 個人の PC にのみデプロイする必要がある
- ゲームサーバーの性能が不足しているため、上記のいずれかの方案を採用する

**[pst-agent デプロイメントガイド](./README.agent.ja.md) または [k8s-pod からの存档同期](#k8s-pod-からの存档同期) を参照してください**

### ファイルデプロイメント

以下のアドレスから最新版の実行可能ファイルをダウンロードしてください。

- [Github Releases](https://github.com/zaigie/palworld-server-tool/releases)

#### Linux

##### ダウンロードと解凍

```bash
# pst_{version}_{platform}_{arch}.tar.gz ファイルをダウンロードしてpstディレクトリに解凍します
mkdir -p pst && tar -xzf pst_v1.8.0_linux_x86_64.tar.gz -C pst
```

##### 設定

1. ディレクトリを開いて実行可能にします

   ```bash
   cd pst
   chmod +x pst sav_cli
   ```

2. `config.yaml`ファイルを見つけて、指示に従って変更します。

   `decode_path`については、通常は pst ディレクトリに`sav_cli`を追加するだけです。空にすることができ、デフォルトで現在のディレクトリを取得します。

   ```yaml
   # Palworld の設定ファイルと起動制御（ゲームバージョン 1.0.0）
   palworld:
     # Web UI から直接編集する PalWorldSettings.ini のローカルパス
     config_path: "/path/to/PalServer/Pal/Saved/Config/LinuxServer/PalWorldSettings.ini"
     control:
       # disabled / process / docker / systemd / windows_service
       mode: "systemd"
       target: "palworld.service"
       arguments: []
       working_directory: ""
       timeout: 120

   # WebUI設定
   web:
     # WebUI管理者パスワード
     password: ""
     # WebUIアクセスポート
     port: 8080
     # HTTPS TLSアクセスを有効にするかどうか
     tls: false
     # TLS証明書のパス HTTPSを使用する場合は証明書ファイルのパスを入力してください
     cert_path: ""
     # TLSキーのパス HTTPSを使用する場合は証明書キーファイルのパスを入力してください
     key_path: ""
     # HTTPSアクセスを有効にする場合は、HTTPS証明書にバインドされたドメイン名を入力してください 例：https://yourdomain.com
     public_url: ""

   # タスク関連設定です
   task:
     # タイミングゲームサービスにプレーヤーのオンライン状況を取得する間隔、単位秒です
     sync_interval: 60
     # プレイヤーのサーバーへの入/出通知です
     player_logging: true
     # プレイヤーはサーバーメッセージにアクセスします
     player_login_message: "Player {username} has joined the server! Current online player count: {online_num}."
     # プレイヤーはサーバーメッセージから離脱します
     player_logout_message: "Player {username} has left the server! Current online player count: {online_num}."


   # REST API 関連構成です
   rest:
     # RESTのアドレスです
     address: "http://127.0.0.1:8212"
     # Base Authのユーザー名,adminに固定します
     username: "admin"
     password: ""
     # 通信のタイムアウト時間、<= 5を推奨
     timeout: 5

   # sav_cli Config 存档ファイル解析関連設定
   save:
     # 存档ファイルパス
     path: "/path/to/your/Pal/Saved"
     # Sav_cli Path 存档解析ツールのパス、通常はpstと同一ディレクトリ、空にすることができます
     decode_path: ""
     # Sav Decode Interval Sec 存档からデータを取得する間隔、秒単位、>= 120を推奨
     sync_interval: 120
     # Sav Backup Interval Sec アーカイブ自動バックアップ間隔です、秒単位
     backup_interval: 0
     # Sav Backup Keep Days アーカイブ自動バックアップを保持する日数です、日単位
     backup_keep_days: 7

   # Manage Config ホワイトリスト管理関連
   manage:
     # プレイヤーがホワイトリストにない場合に自動的にキックするかどうか
     kick_non_whitelist: false
   ```

> [!NOTE]
> Palworld 1.0.0 には標準のワールドバックアップがあるため、通常の復元にはゲーム側のバックアップを使い、PST の `save.backup_interval` は既定で `0` になりました。プレイヤーまたはパルのセーブ編集前に作成する必須の安全バックアップは引き続き有効です。
>
> `palworld.config_path` は PST からローカルに参照できる `PalWorldSettings.ini` を指定します。Web 書き込みではダイジェスト確認、旧ファイルのバックアップ、原子的置換を行います。`palworld.control` は任意のシェルを実行せず、`process`、`docker`、`systemd`、`windows_service` の制限されたドライバーだけをサポートします。
>
> バックアップ画面は現在のワールドの `backup/world/<timestamp>` を直接読み取ります。復元時は全ファイルを再ハッシュし、サーバーを停止し、現在のワールドを含む PST 復元ポイントを作成してから、同一ファイルシステム上でロールバック可能な置換を行います。ローカルの `save.path` が必要です。
>
> WorldOption の生成・同期はサーバー INI に保存済みの設定だけを受け付け、固定された Palworld 1.0.0 型メタデータを使用します。停止、完全復元ポイント、無損失検証、原子的インストールが必須です。

##### 実行

```bash
./pst
```

```log
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:75 | Starting PalWorld Server Tool...
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:76 | Version: Develop
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:77 | Listening on http://127.0.0.1:8080 or http://192.168.1.66:8080
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:78 | Swagger on http://127.0.0.1:8080/swagger/index.html
```

SSH ウィンドウを閉じても実行を続ける場合は以下のようにします。

```bash
# バックグラウンドで実行し、ログをserver.logに保存します
nohup ./pst > server.log 2>&1 &
# ログを確認する
tail -f server.log
```

##### バックグラウンド実行を停止する

```bash
kill $(ps aux | grep 'pst' | awk '{print $2}') | head -n 1
```

##### アクセス

ブラウザを通じて http://127.0.0.1:8080 または http://{ローカルネットワーク IP}:8080 にアクセスしてください。

クラウドサーバーのファイアウォールとセキュリティグループを開放した後、http://{サーバー IP}:8080 にアクセスすることもできます。

> [!WARNING]
> 最初に開いたときには内容が表示されずに空白になる場合があります。**最初の sav ファイル同期が完了するまでお待ちください**。

> サーバーの設定が十分で、パフォーマンスが良い場合は、`save.sync_interval`を短くしてみることができます。

#### Windows

##### ダウンロードと解凍

`pst_v1.8.0_windows_x86_64.zip`を任意のディレクトリに解凍します（`pst`というディレクトリ名を推奨）。

##### 設定

解凍ディレクトリ内の`config.yaml`ファイルを見つけ、指示に従って変更します。

`decode_path`に関しては、解凍後の pst ディレクトリに`sav_cli.exe`を追加するだけです。空にすることができ、デフォルトで現在のディレクトリを取得します。

マウスの右クリックから「プロパティ」を選択し、パスとファイル名を確認してから、それらを結合してください。（存档ファイルのパスとツールのパスも同様）

> [!WARNING]
> コピーしたパスを`config.yaml`に直接貼り付けるのではなく、すべての'\\'の前にもう一つ'\\'を追加する必要があります。以下に示すように
>
> また重要なのは、`config.yaml`ファイルが**ANSI エンコーディング**であることを確認してください。他のエンコーディング形式はパスエラーなどの問題を引き起こす可能性があります！！

```yaml
# Palworld の設定ファイルと起動制御（ゲームバージョン 1.0.0）
palworld:
  config_path: "C:\\path\\to\\PalServer\\Pal\\Saved\\Config\\WindowsServer\\PalWorldSettings.ini"
  control:
    mode: "process"
    target: "C:\\path\\to\\PalServer.exe"
    arguments: []
    working_directory: "C:\\path\\to"
    timeout: 120

# WebUI設定
web:
  # WebUI管理者パスワード
  password: ""
  # WebUIアクセスポート
  port: 8080
  # HTTPS TLSアクセスを有効にするかどうか
  tls: false
  # TLS証明書のパス HTTPSを使用する場合は証明書ファイルのパスを入力してください
  cert_path: ""
  # TLSキーのパス HTTPSを使用する場合は証明書キーファイルのパスを入力してください
  key_path: ""
  # HTTPSアクセスを有効にする場合は、HTTPS証明書にバインドされたドメイン名を入力してください 例：https://yourdomain.com
  public_url: ""

# タスク関連設定です
task:
  # タイミングゲームサービスにプレーヤーのオンライン状況を取得する間隔、単位秒です
  sync_interval: 60
  # プレイヤーのサーバーへの入/出通知です
  player_logging: true
  # プレイヤーはサーバーメッセージにアクセスします
  player_login_message: "Player {username} has joined the server! Current online player count: {online_num}."
  # プレイヤーはサーバーメッセージから離脱します
  player_logout_message: "Player {username} has left the server! Current online player count: {online_num}."


# REST API 関連構成です
rest:
  # RESTのアドレスです
  address: "http://127.0.0.1:8212"
  # Base Authのユーザー名,adminに固定します
  username: "admin"
  password: ""
  # 通信のタイムアウト時間、<= 5を推奨
  timeout: 5

# sav_cli Config 存档ファイル解析関連設定
save:
  # 存档ファイルパス
  path: "/path/to/your/Pal/Saved"
  # Sav_cli Path 存档解析ツールのパス、通常はpstと同一ディレクトリ、空にすることができます
  decode_path: ""
  # Sav Decode Interval Sec 存档からデータを取得する間隔、秒単位、>= 120を推奨
  sync_interval: 120
  # Sav Backup Interval Sec アーカイブ自動バックアップ間隔です、秒単位
  backup_interval: 0
  # Sav Backup Keep Days アーカイブ自動バックアップを保持する日数です、日単位
  backup_keep_days: 7

# Manage Config ホワイトリスト管理関連
manage:
  # プレイヤーがホワイトリストにない場合に自動的にキックするかどうか
  kick_non_whitelist: false
```

##### 実行

Windows で実行するには 2 つの

方法があります。

1. start.bat（推奨）

   解凍ディレクトリ内の`start.bat`ファイルをダブルクリックして実行します。

2. `Win + R`を押して`powershell`を入力し、Powershell を開きます。`cd`コマンドでダウンロードした実行ファイルのディレクトリに移動します。

   ```powershell
   .\pst.exe
   ```

```log
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:75 | Starting PalWorld Server Tool...
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:76 | Version: Develop
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:77 | Listening on http://127.0.0.1:8080 or http://192.168.31.214:8080
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:78 | Swagger on http://127.0.0.1:8080/swagger/index.html
```

上記の画面が表示されたら、正常に実行されています。ウィンドウを開いたままにしてください。

##### アクセス

ブラウザを通じて http://127.0.0.1:8080 または http://{ローカルネットワーク IP}:8080 にアクセスしてください。

クラウドサーバーのファイアウォールとセキュリティグループを開放した後、http://{サーバー IP}:8080 にアクセスすることもできます。

> [!WARNING]
> 最初に開いたときには内容が表示されずに空白になる場合があります。**最初の sav ファイル同期が完了するまでお待ちください**。
>
> サーバーの設定が十分で、パフォーマンスが良い場合は、`save.sync_interval`を短くしてみることができます。

### Docker デプロイメント

#### 単体デプロイメント

単一のコンテナが必要で、ゲームの存档ディレクトリをコンテナ内にマッピングし、ゲームサーバーと同じ物理マシン上で実行します。

> 注意:スワップ領域を使用すると、プログラムのパフォーマンスが低下する可能性があります。メモリが不足している場合のみ使用してください

```bash
docker run -d --name pst \
-p 8080:8080 \
-v /path/to/your/Pal/Saved:/game \
-v ./backups:/app/backups \
-e WEB__PASSWORD="your web password" \
-e REST__ADDRESS="http://127.0.0.1:8212" \
-e REST__PASSWORD="your admin password" \
-e SAVE__PATH="/game" \
-e SAVE__SYNC_INTERVAL=120 \
jokerwho/palworld-server-tool:latest
```

最も重要なのは、ゲームの存档ファイル（Level.sav）があるディレクトリを-v オプションでコンテナ内の/game ディレクトリにマッピングすることです。

##### 永続化

`pst.db`ファイルを永続化する必要がある場合：

```bash
# ファイルをディレクトリとして認識されないようにするために、先にファイルを作成します
touch pst.db
```

その後、`docker run -v`に`-v ./pst.db:/app/pst.db`を追加します。

##### 環境変数

各環境変

数を設定します。[`config.yaml`](#設定)と基本的に似ていますが、以下の表のようになります：

> [!WARNING]
> 単一と複数のアンダースコアを区別してください。変更が必要な場合は、下表の変数名をコピーして使用してください！

|            変数名             |      デフォルト値       |    タイプ    |                                          説明                                          |
| :---------------------------: | :---------------------: | :----------: | :------------------------------------------------------------------------------------: |
|        WEB\_\_PASSWORD        |           ""            |    文字列    |                         Web インターフェースの管理者パスワード                         |
|          WEB\_\_PORT          |          8080           |     数値     | **特に必要がない限り、変更するのではなくコンテナのマッピングポートを変更してください** |
|                               |                         |              |                                                                                        |
|                               |                         |              |                                                                                        |
|     TASK\_\_SYNC_INTERVAL     |           60            |     数値     |                サーバーにプレイヤーのオンラインデータの同期を要求します                |
|    TASK\_\_PLAYER_LOGGING     |          false          | ブール値です |                      プレイヤー登録・アナウンスメッセージ掲載です                      |
| TASK\_\_PLAYER_LOGIN_MESSAGE  |           ""            |    文字列    |                     プレーヤー登録メッセージコンテンツを放送します                     |
| TASK\_\_PLAYER_LOGOUT_MESSAGE |           ""            |    文字列    |                         プレイヤーが放送メッセージを掲載します                         |
|                               |                         |              |                                                                                        |
|        REST\_\_ADDRESS        | "http://127.0.0.1:8212" |    文字列    |                         API に対応したアドレスを提供しています                         |
|       REST\_\_PASSWORD        |           ""            |    文字列    |                          サーバー設定ファイルの AdminPassword                          |
|        REST\_\_TIMEOUT        |            5            |     数値     |                               タイムアウトをお願いします                               |
|                               |                         |              |                                                                                        |
|         SAVE\_\_PATH          |           ""            |    文字列    |       ゲームの存档ファイルのパス **コンテナ内のパスとして必ず記入してください**        |
|      SAVE\_\_DECODE_PATH      |     "/app/sav_cli"      |    文字列    |              ⚠️ コンテナ内蔵、変更禁止、存档解析ツールのエラーになります               |
|     SAVE\_\_SYNC_INTERVAL     |           600           |     数値     |                          プレイヤーの存档データを同期する間隔                          |
|    SAVE\_\_BACKUP_INTERVAL    |            0            |     数値     | ゲーム標準バックアップを優先し、0 より大きい場合のみ PST 周期バックアップを追加します |
|   SAVE\_\_BACKUP_KEEP_DAYS    |            7            |     数値     |                      アーカイブ自動バックアップを保持する日数です                      |
| MANAGE\_\_KICK_NON_WHITELIST  |          false          | ブール値です |            プレイヤーがホワイトリストにない場合に自動的にキックするかどうか            |

#### Agent デプロイメント

`palworld-server-tool`と`palworld-server-tool-agent`の 2 つのコンテナが必要です。

適用可能なシナリオ：

- 他のサーバーに単独でデプロイする必要がある
- 個人の PC にのみデプロイする必要がある
- ゲームサーバーの性能が不足しているため、上記のいずれかの方案を採用する

##### 最初に agent コンテナを実行する

> 注意:スワップ領域を使用すると、プログラムのパフォーマンスが低下する可能性があります。メモリが不足している場合のみ使用してください

```bash
docker run -d --name pst-agent \
-p 8081:8081 \
-v /path/to/your/Pal/Saved:/game \
-e SAVED_DIR="/game" \
jokerwho/palworld-server-tool-agent:latest
```

ゲームの存档ファイル（Level.sav）があるディレクトリを-v オプションでコンテナ内の/game ディレクトリにマッピングする必要があります。

|  変数名   | デフォルト値 | タイプ |                                   説明                                    |
| :-------: | :----------: | :----: | :-----------------------------------------------------------------------: |
| SAVED_DIR |      ""      | 文字列 | ゲームの存档ファイルのパス **コンテナ内のパスとして必ず記入してください** |

##### 次に pst コンテナを実行する

```bash
docker run -d --name pst \
-p 8080:8080 \
-v ./backups:/app/backups \
-e WEB__PASSWORD="your password" \
-e REST__ADDRESS="http://{GameServerIP}:{RestAPIPort}" \
-e REST__PASSWORD="your admin password" \
-e SAVE__PATH="http://{GameServerIP}:{AgentPort}/sync" \
-e SAVE__SYNC_INTERVAL=120 \
jokerwho/palworld-server-tool:latest
```

##### 永続化

`pst.db`ファイルを永続化する必要がある場合：

```bash
# ファイルをディレクトリとして認識されないようにするために、先にファイルを作成します
touch pst.db
```

その後、`docker run -v`に`-v ./pst.db:/app/pst.db`を追加します。

##### 環境変数

> [!WARNING]
> 単一と複数のアンダースコアを区別してください。変更が必要な場合は、下表の変数名をコピーして使用してください！

|            変数名             |      デフォルト値       |    タイプ    |                                             説明                                              |
| :---------------------------: | :---------------------: | :----------: | :-------------------------------------------------------------------------------------------: |
|        WEB\_\_PASSWORD        |           ""            |    文字列    |                            Web インターフェースの管理者パスワード                             |
|          WEB\_\_PORT          |          8080           |     数値     |    **特に必要がない限り、変更するのではなくコンテナのマッピングポートを変更してください**     |
|                               |                         |              |                                                                                               |
|                               |                         |              |                                                                                               |
|     TASK\_\_SYNC_INTERVAL     |           60            |     数値     |                   サーバーにプレイヤーのオンラインデータの同期を要求します                    |
|    TASK\_\_PLAYER_LOGGING     |          false          | ブール値です |                         プレイヤー登録・アナウンスメッセージ掲載です                          |
| TASK\_\_PLAYER_LOGIN_MESSAGE  |           ""            |    文字列    |                        プレーヤー登録メッセージコンテンツを放送します                         |
| TASK\_\_PLAYER_LOGOUT_MESSAGE |           ""            |    文字列    |                            プレイヤーが放送メッセージを掲載します                             |
|                               |                         |              |                                                                                               |
|        REST\_\_ADDRESS        | "http://127.0.0.1:8212" |    文字列    |                            API に対応したアドレスを提供しています                             |
|       REST\_\_PASSWORD        |           ""            |    文字列    |                             サーバー設定ファイルの AdminPassword                              |
|        REST\_\_TIMEOUT        |            5            |     数値     |                                  タイムアウトをお願いします                                   |
|                               |                         |              |                                                                                               |
|         SAVE\_\_PATH          |           ""            |    文字列    | pst-agent があるサービスのアドレス、形式は<br> http://{ゲームサーバー IP}:{Agent ポート}/sync |
|      SAVE\_\_DECODE_PATH      |     "/app/sav_cli"      |    文字列    |                  ⚠️ コンテナ内蔵、変更禁止、存档解析ツールのエラーになります                  |
|     SAVE\_\_SYNC_INTERVAL     |           600           |     数値     |                             プレイヤーの存档データを同期する間隔                              |
|    SAVE\_\_BACKUP_INTERVAL    |            0            |     数値     | ゲーム標準バックアップを優先し、0 より大きい場合のみ PST 周期バックアップを追加します |
|   SAVE\_\_BACKUP_KEEP_DAYS    |            7            |     数値     |                         アーカイブ自動バックアップを保持する日数です                          |
| MANAGE\_\_KICK_NON_WHITELIST  |          false          | ブール値です |               プレイヤーがホワイトリストにない場合に自動的にキックするかどうか                |

#### k8s-pod からの存档同期

v0.5.3 から、agent なしでクラスタ内のゲームサーバーの存档を同期することがサポートされています。

> v0.5.8 の後で、プレーヤーのバックパックのデータを増加して見るため、復制するのは全体 Sav ファイルのディレクトリで、パルのサービスの端の容器の中に tar 工具があることを確保しなければ圧縮して伸張します

> pst が使用する serviceaccount には"pods/exec"権限が必要

です！

`SAVE__PATH`環境変数を変更するだけでよく、形式は以下の通りです：

```bash
SAVE__PATH="k8s://<namespace>/<podname>/<container>:<ゲームの存档ディレクトリ>"
```

例えば：

```bash
SAVE__PATH="k8s://default/palworld-server-0/palworld-server:/palworld/Pal/Saved"
```

> ゲームサーバーが Level.sav ファイルを作成する時間と位置（HASH を含む）は初回には不確定なため、Saved ディレクトリレベルを指定してください。プログラムが自動的にスキャンします

pst とゲームサーバーが同一の namespace にある場合、namespace を省略できます：

```bash
SAVE__PATH="k8s://palworld-server-0/palworld-server:/palworld/Pal/Saved"
```

### docker コンテナからの存档同期

v0.5.3 から、agent なしでコンテナ内のゲームサーバーの存档を同期することがサポートされています。

#### ファイルデプロイメント使用時

pst 本体がバイナリファイルとしてデプロイされている場合、`config.yaml`内の`save.path`を変更するだけです：

```yaml
save:
  path: "docker://<container_name_or_id>:<ゲームの存档ディレクトリ>"
```

例えば：

```yaml
save:
  path: docker://palworld-server:/palworld/Pal/Saved
# または
save:
  path: docker://04b0a9af4288:/palworld/Pal/Saved
```

#### Docker デプロイメント使用時

pst 本体が Docker 単体デプロイメントである場合、`SAVE__PATH`環境変数を変更し、Docker デーモンを pst コンテナ内にマウントする必要があります

1. デーモンをマウントする

元の`docker run`コマンドに`-v /var/run/docker.sock:/var/run/docker.sock`を追加します

2. 環境変数を変更する

`SAVE__PATH`環境変数を以下の形式で変更します：

```bash
SAVE__PATH="docker://<container_name_or_id>:<ゲームの存档ディレクトリ>"
```

例えば：

```bash
SAVE__PATH="docker://palworld-server:/palworld/Pal/Saved"
# または
SAVE__PATH="docker://04b0a9af4288:/palworld/Pal/Saved"
```

> [!WARNING]
> 実行後に` Error response from daemon: client version 1.44 is too new. Maximum supported API version is 1.43`のようなエラーが表示された場合は、現在の docker engine が使用している Docker API のバージョンが低いことを意味します。その場合は、別の環境変数を追加してください：
>
> -e DOCKER_API_VERSION="1.43" (あなたの API バージョン)

> ゲームサーバーが Level.sav ファイルを作成する時間と位置（HASH を含む）は初回には不確定なため、Saved ディレクトリレベルを指定してください。プログラムが自動的にスキャンします

## プロジェクトの統計

![Stats](https://repobeats.axiom.co/api/embed/8724e69c284e0645f764a4a1cd525477be13cbe8.svg "Repobeats analytics image")

## API ドキュメント

[APIFox オンライン API ドキュメント](https://q4ly3bfcop.apifox.cn/)

## 謝辞

- [PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) の `palsav-flex` は、現在のセーブ解析、Oodle 圧縮、再構築機能を提供します
- [palworld-server-toolkit](https://github.com/magicbear/palworld-server-toolkit) は存档の高性能解析の一部を提供しました
- [pal-conf](https://github.com/Bluefissure/pal-conf) は最新のサーバー設定一覧と翻訳の参照元です
- [PalEdit](https://github.com/EternalWraith/PalEdit) は最初のデータ化思考とロジックを提供しました

## ライセンス

メインアプリケーションは [Apache 2.0](LICENSE) で提供されます。別プロセスの `sav_cli` には GPL-3.0-or-later コンポーネントが含まれ、配布物に `sav_cli-GPL-3.0.txt` を同梱します。
