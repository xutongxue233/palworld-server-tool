<h1 align='center'>幻兽帕鲁服务器管理工具</h1>

<p align="center">
   <strong>简体中文</strong> | <a href="/README.en.md">English</a> | <a href="/README.ja.md">日本語</a>
</p>

<p align="center">
  <img src="./web/src/assets/app-icon.png" width="112" alt="Palworld Server Tool 图标" />
</p>

<p align="center">
  <a href="https://github.com/xutongxue233/palworld-server-tool/releases/latest">下载最新版本</a> · <a href="./CHANGELOG.md">更新记录</a>
</p>

<p align='center'>
  通过 React 可视化界面、官方 REST API 与最新 SAV 存档解析管理幻兽帕鲁专用服务器<br/>
  并且花了很漫长且枯燥的时间去做了国际化...
</p>

<p align='center'>
<img alt="GitHub Release" src="https://img.shields.io/github/v/release/xutongxue233/palworld-server-tool?style=for-the-badge">&nbsp;&nbsp;
<img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white">&nbsp;&nbsp;
<img alt="Python" src="https://img.shields.io/badge/Python-FFD43B?style=for-the-badge&logo=python&logoColor=blue">&nbsp;&nbsp;
<img alt="React" src="https://img.shields.io/badge/React-20232A?style=for-the-badge&logo=react&logoColor=61DAFB">
</p>

![PC](./docs/img/pst-zh-1.png)

> 目前移动端适配良好，可查看下面 [功能截图](#功能截图)
>
> 当然深色模式也安排得妥妥的～

基于 `Level.sav` 存档文件解析实现的功能：

- [x] 完整玩家数据
- [x] 玩家帕鲁数据
- [x] 公会数据
- [x] 玩家背包物品数据

基于官方提供的 REST API 实现功能：

- [x] 获取服务器信息
- [x] 获取服务器指标数据
- [x] 获取服务器当前设置
- [x] 在线玩家列表
- [x] 踢出/封禁/解封玩家（支持跨平台 User ID 与操作提示消息）
- [x] 游戏内广播
- [x] 立即保存世界
- [x] 平滑关闭服务器并广播消息
- [x] 强制停止服务器
- [x] 获取世界 Actor 快照数据

工具额外提供的功能：

- [x] 可视化地图管理
- [x] 白名单管理
- [x] 发现、校验并安全恢复游戏自带的 `backup/world` 世界备份
- [x] 直接编辑 `PalWorldSettings.ini`，检测并安全生成/同步 `WorldOption.sav`
- [x] 进程、Docker、systemd 与 Windows 服务的受限启停/重启
- [x] 受 Web 管理认证保护的 RCON 命令终端和全部 13 条 1.0.0 官方命令模板
- [x] 类型化定时任务、不会干扰人工停服的服务器看门狗，以及通用/Discord Webhook 通知
- [x] 固定 App ID 2394010 的 SteamCMD 安装、更新、文件校验和可选自动重启
- [x] 危险存档操作前自动创建 PST 安全恢复点

### 存档校验与离线编辑

发行包内置的新版本 `sav_cli` 支持当前 Palworld 存档的 Oodle 压缩格式，可校验、导出 JSON、从 JSON 重建并执行无损往返测试。编辑前必须停止服务器并备份整个世界存档目录，禁止直接覆盖正在运行的 `Level.sav`。

```powershell
# 校验存档
.\sav_cli.exe --mode validate --file .\Level.sav

# 导出可编辑 JSON
.\sav_cli.exe --mode export --file .\Level.sav --output .\Level.editable.json

# 编辑 JSON 后重建为新文件；不会覆盖原始存档
.\sav_cli.exe --mode rebuild --file .\Level.editable.json --output .\Level.edited.sav
```

详细参数和许可证说明见 [`sav_cli/README.md`](./sav_cli/README.md)。

本工具使用 bbolt 单文件存储，通过定时任务同步官方 REST API 与 Level.sav 数据，并提供可视化管理界面。

由于维护开发人员较少，虽有心但力不足，欢迎各前端和后端甚至数据工程师来提交 PR！

> [!NOTE]
> 如果您需要幻兽帕鲁服务器&工具搭建交流，或者**需要闭源付费定制功能开发**，如：多服务器管理、服务器注入反作弊、可视化修改存档等，请加群或 QQ 交流

幻兽帕鲁服务器管理交流：<a target="_blank" href="https://qm.qq.com/cgi-bin/qm/qr?k=RkItz42aIvppN716Tdlpni_gSpnYasxF&jump_from=webapi&authKey=PLbIHENUObGLnW4s5476OnenRVcUNBV79g9zd0CEi5kpddfdooAsoU/SeoEdfGWq"><img border="0" src="https://pub.idqqimg.com/wpa/images/group.png" alt="幻兽帕鲁服务器管理" title="幻兽帕鲁服务器管理"></a>

![加QQ群](./docs/img/add_group.jpg)

## 功能截图

https://github.com/zaigie/palworld-server-tool/assets/17232619/afdf485c-4b34-491d-9c1f-1eb82e8060a1

### 桌面端

|                              |                              |
| :--------------------------: | :--------------------------: |
| ![](./docs/img/pst-zh-2.png) | ![](./docs/img/pst-zh-3.png) |

![](./docs/img/pst-zh-4.png)

### 移动端

<p align="center">
<img src="./docs/img/pst-zh-m-1.png" width="30%" /><img src="./docs/img/pst-zh-m-2.png" width="30%" /><img src="./docs/img/pst-zh-m-3.png" width="30%" />
</p>

## 开启 REST API

本项目必须启用服务器的官方 REST API 才能同步在线玩家并执行服务器管理操作。

服务端 REST API 的字段与行为以 [Pocketpair 官方服务端文档](https://docs.palworldgame.com/category/rest-api) 为准。

配置页可以直接读取、校验并写回 `PalWorldSettings.ini`。检测到 `WorldOption.sav` 时会明确提示覆盖关系，并可在停服、完整备份和无损校验后，把已经保存的 1.0.0 配置安全同步到现有文件；没有文件时也可从固定校验的 1.0.0 基准生成。

先设置 **管理员密码**

![ADMIN](./docs/img/admin-zh.png)

再启用 **REST API**

## 可选：启用 RCON

Palworld 1.0.0 仍可使用 RCON，但官方已将其标记为弃用，常用管理建议优先使用 REST API。若需兼容旧服务器或插件命令，请在 `PalWorldSettings.ini` 中设置 `RCONEnabled=True`、确认 `RCONPort`，并配置非空 `AdminPassword`。随后在 PST 的 `config.yaml` 中填写相同密码：

```yaml
rcon:
  address: "127.0.0.1:25575"
  password: "你的 AdminPassword"
  use_base64: false
  timeout: 5
```

RCON 端口只应向 PST 所在主机或受信任网络开放。`use_base64` 仅用于兼容明确支持 Base64 命令的代理，直连官方服务端时保持 `false`。

## 自动化、看门狗与通知

登录 Web 管理模式后，在“运维 → 自动化”中可以创建每隔一段时间、每天或每周执行的任务，不需要填写 Cron。支持的动作固定为保存世界、发送公告、启动、安全停止、保存并重启、同步解析存档和额外 PST 安全备份；后端不会把用户输入拼接成 Shell 或任意 RCON 命令。任务、最近 500 次运行结果和自动化设置保存在 `pst.db`，PST 重启后会重新注册任务。

服务器看门狗同时检查受限控制驱动的进程状态和 Palworld REST `/info` 响应。只有连续失败达到阈值后才尝试恢复，并带启动宽限、冷却和最大恢复次数。通过 Web 或类型化任务主动停止服务器时，PST 会持久记录“允许停机”，看门狗不会把它重新拉起；重新启动服务器后会恢复“保持运行”目标。

离线存档编辑、配置写入、RCON、旧版周期同步/备份和自动化维护共享同一个操作锁。通过 RCON 执行 `Shutdown` 或 `DoExit` 同样会记录有意停机，避免看门狗与管理员命令互相对抗。

通知支持通用 JSON Webhook 和 Discord Webhook，可选择任务、手动启停、服务器异常与恢复事件。通用 Webhook 可使用 `X-PST-Signature: sha256=<HMAC>` 验证消息。默认仅允许公网 HTTPS 目标，并拒绝跳转、localhost 和私有网络地址。Webhook 地址和签名密钥不会通过读取 API 回显。

首次启动时可由 `config.yaml` 的 `automation.watchdog` 与 `automation.notification` 提供默认值，之后可在 Web UI 中保存。看门狗要求先配置 `palworld.control`。设计参考了 [palworld-server-docker 的定时备份/Discord 通知](https://github.com/thijsvanloef/palworld-server-docker) 和 [TRRabbit Palworld Server Manager 的 Scheduler/Guardian 交互](https://github.com/TRRabbit/palworld-server-manager)，但 PST 保留自己的白名单动作、安全互斥和出站通知边界。

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
    provider: "generic" # generic 或 discord
    webhook_url: ""
    secret: "" # 通用 Webhook 可选 HMAC-SHA256 密钥
    events: ["task.failed", "watchdog.unhealthy", "watchdog.recovered"]
    timeout_seconds: 10
    allow_private_network: false
```

## SteamCMD 安装与更新

Palworld 1.0.0 官方专用服务器应用 ID 为 `2394010`。在 Web 管理模式的“运维 → 部署更新”中，PST 可以执行官方 `app_update 2394010` 安装/更新流程，并默认启用文件校验。PST 不查询第三方“最新版本”接口，也不接受 Shell、自定义应用 ID 或任意 SteamCMD 参数。

```yaml
steamcmd:
  # Windows 使用 steamcmd.exe；Linux 使用 steamcmd.sh 或 steamcmd。必须是绝对路径。
  executable: "C:/steamcmd/steamcmd.exe"
  # Palworld Dedicated Server 安装目录，不能是磁盘根目录。
  install_dir: "D:/PalworldServer"
  # 单次安装/更新最大用时，允许 60-7200 秒。
  timeout: 1800
```

每次执行都会重新校验 SteamCMD 文件哈希、安装目录、`appmanifest_2394010.acf`、平台启动文件和计划摘要。若安装目录中已有世界数据，`save.path` 必须指向该安装目录内的本地世界，PST 会先停止服务器并强制创建完整恢复点，备份失败则不会运行 SteamCMD。配置了 `palworld.control` 时可安全停服并按需重新启动；否则必须先在系统中完全停止 PalServer，再在页面明确确认。

参考：[Palworld 1.0.0 官方 SteamCMD 部署说明](https://docs.palworldgame.com/getting-started/deploy-dedicated-server)。


## 安装部署

- [Sealos 一键部署](#sealos-一键部署)
- [文件部署](#文件部署)
  - [Linux](#linux)
  - [Windows](#windows)
- [Docker 部署](#docker-部署)
  - [单体部署](#单体部署)
  - [Agent 部署](#agent-部署)
  - [从 k8s-pod 同步存档](#从-k8s-pod-同步存档)
- [从 docker 容器同步存档](#从-docker-容器同步存档)

> 解析 `Level.sav` 存档的任务需要在短时间（<20s）耗费一定的系统内存（1GB~3GB），这部分内存会在执行完解析任务后释放，因此你至少需要确保你的服务器有充足的内存。

这里**默认为将 pst 工具和游戏服务器放在同一台物理机上**，在一些情况下你可能不想要它们部署在同一机器上：

- 需要单独部署在其它服务器
- 只需要部署在本地个人电脑
- 游戏服务器性能较弱不满足，采用上述两种方案之一

**请参考 [pst-agent 部署教程](./README.agent.md) 或 [从 k8s-pod 同步存档](#从-k8s-pod-同步存档)**

### Sealos 一键部署

**30s 部署私服 + 管理工具，拒绝复杂步骤**

首先点击以下按钮一键部署帕鲁私服：


然后点击以下按钮一键部署 palworld-server-tool：


### 文件部署

请在以下地址下载最新版可执行文件

- [Github Releases](https://github.com/zaigie/palworld-server-tool/releases)

#### Linux

##### 下载解压

```bash
# 下载 pst_{version}_{platform}_{arch}.tar.gz 文件并解压到 pst 目录
mkdir -p pst && tar -xzf pst_v1.5.0_linux_x86_64.tar.gz -C pst
```

##### 配置

1. 打开目录并允许可执行

   ```bash
   cd pst
   chmod +x pst sav_cli
   ```

2. 找到其中的 `config.yaml` 文件并按照说明修改。

   关于其中的 `decode_path`，一般就是解压后的 pst 目录加上 `sav_cli` ，可以为空，默认会获取当前目录

   ```yaml
   # Palworld 游戏文件与进程控制（游戏版本 1.0.0）
   palworld:
     # PalWorldSettings.ini 的本地路径；配置后可在 Web UI 直接读写
     config_path: "/path/to/PalServer/Pal/Saved/Config/LinuxServer/PalWorldSettings.ini"
     control:
       # disabled / process / docker / systemd / windows_service
       mode: "systemd"
       # 进程可执行文件、容器名、systemd unit 或 Windows 服务名
       target: "palworld.service"
       arguments: []
       working_directory: ""
       timeout: 120

   # WebUI 设置
   web:
     # WebUI 管理员密码
     password: ""
     # WebUI 访问端口
     port: 8080
     # 是否开启使用 HTTPS TLS 访问
     tls: false
     # TLS Cert 如果开启使用 HTTPS 请输入证书文件路径
     cert_path: ""
     # TLS Key 如果开启使用 HTTPS 请输入证书密钥文件路径
     key_path: ""
     # 若开启 HTTPS 访问请填写你的 HTTPS 证书绑定的域名 eg. https://yourdomain.com
     public_url: ""

   # 任务相关设置
   task:
     # 定时向游戏服务获取玩家在线情况的间隔，单位秒
     sync_interval: 60
     # 玩家进入/离开服务器通知
     player_logging: true
     # 玩家进入服务器消息
     player_login_message: "玩家 {username} 加入服务器!\n当前在线人数: {online_num}"
     # 玩家离开服务器消息
     player_logout_message: "玩家 {username} 离开服务器!\n当前在线人数: {online_num}"


   # REST API 相关配置
   rest:
     # REST 的地址
     address: "http://127.0.0.1:8212"
     # Base Auth 的用户名，固定为 admin
     username: "admin"
     # 服务端设置的 AdminPassword
     password: ""
     # 通信超时时间，推荐 <= 5
     timeout: 5

   # sav_cli Config 存档文件解析相关配置
   save:
     # 存档文件路径
     path: "/path/to/your/Pal/Saved"
     # 存档解析工具路径，一般和 pst 在同一目录，可以为空
     decode_path: ""
     # 定时从存档获取数据的间隔，单位秒，推荐 >= 120
     sync_interval: 120
     # 存档定时备份间隔，单位秒，设置为0时禁用
     backup_interval: 0
     # 存档定时备份保留天数，默认为7天
     backup_keep_days: 7

   # Manage Config 白名单管理相关
   manage:
     # 玩家不在白名单是否自动踢出
     kick_non_whitelist: false
   ```

> [!NOTE]
> Palworld 1.0.0 已自带世界存档备份，日常恢复建议直接使用游戏备份，因此 PST 的 `save.backup_interval` 默认改为 `0`。PST 仍会在修改玩家/帕鲁存档前强制创建安全备份，不受此设置影响。
>
> `palworld.config_path` 仅支持 PST 本机可访问的 `PalWorldSettings.ini`。Web UI 写入时会校验文件摘要、备份旧文件并原子替换。`palworld.control` 不执行任意 Shell，只支持 `process`、`docker`、`systemd`、`windows_service` 四种受限驱动。
>
> 备份页会直接读取当前世界的 `backup/world/<时间戳>`。恢复会先复核每个文件的 SHA-256、停止服务器、创建包含当前世界的 PST 恢复点，再通过同文件系统事务替换并在失败时回滚；只支持本地 `save.path`。若服务器原本正在运行且已配置受限控制驱动，恢复完成后可自动启动。
>
> `WorldOption.sav` 生成/同步只接受已经写入服务器 INI 的配置，固定使用 Palworld 1.0.0 类型元数据；操作同样要求停服、完整恢复点、重建后无损校验和原子安装。世界文件不支持的服务器专用字段会保留在 INI，并在结果中列出。

##### 运行

```bash
./pst
```

```log
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:75 | Starting PalWorld Server Tool...
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:76 | Version: Develop
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:77 | Listening on http://127.0.0.1:8080 or http://192.168.1.66:8080
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:78 | Swagger on http://127.0.0.1:8080/swagger/index.html
```

若需要后台运行（关闭 ssh 窗口后仍运行）

```bash
# 后台运行并将日志保存在 server.log
nohup ./pst > server.log 2>&1 &
# 查看日志
tail -f server.log
```

##### 关闭后台运行

```bash
kill $(ps aux | grep 'pst' | awk '{print $2}') | head -n 1
```

##### 访问

请通过浏览器访问 http://127.0.0.1:8080 或 http://{局域网 IP}:8080

云服务器开放防火墙及安全组后也可以访问 http://{服务器 IP}:8080

> [!WARNING]
> 初次打开会显示空白没有内容，请**等待第一次 sav 存档同步完成**再访问
>
> 如果你的服务器配置足够且性能良好，你可以试着将 `save.sync_interval` 改短一点

#### Windows

##### 下载解压

解压 `pst_v1.5.0_windows_x86_64.zip` 到任意目录（推荐命名文件夹目录名称为 `pst`）

##### 配置

找到解压目录中的 `config.yaml` 文件并按照说明修改。

关于其中的 `decode_path`，一般就是解压后的 pst 目录加上 `sav_cli.exe`，可以为空，默认会获取当前目录

你也可以直接鼠标右键——“属性”，查看路径和文件名，再将它们拼接起来。（存档文件路径和工具路径同理）

> [!WARNING]
> 请不要直接将复制的路径粘贴到 `config.yaml` 中，而是需要在所有的 '\\' 前面再加一个 '\\'，像下面展示的一样
>
> 还有比较重要的是，请确保 `config.yaml` 文件为 **ANSI 编码**，其它编码格式将会导致路径错误等问题！！

````yaml
# Palworld 游戏文件与进程控制（游戏版本 1.0.0）
palworld:
  config_path: "C:\\path\\to\\PalServer\\Pal\\Saved\\Config\\WindowsServer\\PalWorldSettings.ini"
  control:
    # disabled / process / docker / systemd / windows_service
    mode: "process"
    target: "C:\\path\\to\\PalServer.exe"
    arguments: []
    working_directory: "C:\\path\\to"
    timeout: 120

# WebUI 设置
web:
  # WebUI 管理员密码
  password: ""
  # WebUI 访问端口
  port: 8080
  # 是否开启使用 HTTPS TLS 访问
  tls: false
  # TLS Cert 如果开启使用 HTTPS 请输入证书文件路径
  cert_path: ""
  # TLS Key 如果开启使用 HTTPS 请输入证书密钥文件路径
  key_path: ""
  # 若开启 HTTPS 访问请填写你的 HTTPS 证书绑定的域名 eg. https://yourdomain.com
  public_url: ""

# 任务相关设置
task:
  # 定时向游戏服务获取玩家在线情况的间隔，单位秒
  sync_interval: 60
  # 玩家进入/离开服务器通知
  player_logging: true
  # 玩家进入服务器消息
  player_login_message: "玩家 {username} 加入服务器!\n当前在线人数: {online_num}"
  # 玩家离开服务器消息
  player_logout_message: "玩家 {username} 离开服务器!\n当前在线人数: {online_num}"


# REST API 相关配置
rest:
  # REST 的地址
  address: "http://127.0.0.1:8212"
  # Base Auth 的用户名，固定为 admin
  username: "admin"
  # 服务端设置的 AdminPassword
  password: ""
  # 通信超时时间，推荐 <= 5
  timeout: 5

# sav_cli Config 存档文件解析相关配置
save:
  # 存档文件路径
  path: "C:\\path\\to\\your\\Pal\\Saved"
  # 存档解析工具路径，一般和 pst 在同一目录，可以为空
  decode_path: ""
  # 定时从存档获取数据的间隔，单位秒，推荐 >= 120
  sync_interval: 120
  # 存档定时备份间隔，单位秒，设置为0时禁用
  backup_interval: 0
  # 存档定时备份保留天数，默认为7天
  backup_keep_days: 7

# Manage Config 白名单管理相关
manage:
  # 玩家不在白名单是否自动踢出
  kick_non_whitelist: false

##### 运行

这里有两种方式可以在 Windows 下运行

1. start.bat（推荐）

   找到解压目录下的 `start.bat` 文件，双击运行

2. 按下 `Win + R`，输入 `powershell` 打开 Powershell，通过 `cd` 命令到下载的可执行文件目录

   ```powershell
   .\pst.exe
````

```log
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:75 | Starting PalWorld Server Tool...
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:76 | Version: Develop
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:77 | Listening on http://127.0.0.1:8080 or http://192.168.31.214:8080
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:78 | Swagger on http://127.0.0.1:8080/swagger/index.html
```

看到上述界面表示成功运行，请保持窗口打开

##### 访问

请通过浏览器访问 http://127.0.0.1:8080 或 http://{局域网 IP}:8080

云服务器开放防火墙及安全组后也可以访问 http://{服务器 IP}:8080

> [!WARNING]
> 初次打开会显示空白没有内容，请**等待第一次 sav 存档同步完成**再访问
>
> 如果你的服务器配置足够且性能良好，你可以试着将 `save.sync_interval` 改短一点

### Docker 部署

#### 单体部署

只需要一个容器，将游戏存档目录映射至容器内，与游戏服务器在同一物理主机上运行。

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

最重要的是需要 -v 到游戏存档文件（Level.sav）所在目录，将其映射到容器内的 /game 目录

##### 持久化

如果需要持久化 `pst.db` 文件：

```bash
# 先创建文件，避免被识别为目录
touch pst.db
```

然后在 `docker run -v` 中增加 `-v ./pst.db:/app/pst.db`

##### 环境变量

设置各环境变量，与 [`config.yaml`](#配置) 基本相似，表格如下：

> [!WARNING]
> 注意区分单个和多个下划线，若需修改最好请复制下表变量名！

|            变量名             |         默认值          | 类型 |                         说明                         |
| :---------------------------: | :---------------------: | :--: | :--------------------------------------------------: |
|        WEB\_\_PASSWORD        |           ""            | 文本 |               Web 界面的管理员模式密码               |
|          WEB\_\_PORT          |          8080           | 数字 |     **若非必要不建议修改，而是更改容器映射端口**     |
|                               |                         |      |                                                      |
|                               |                         |      |                                                      |
|     TASK\_\_SYNC_INTERVAL     |           60            | 数字 |           请求服务器同步玩家在线数据的间隔           |
|    TASK\_\_PLAYER_LOGGING     |          false          | 布尔 |                玩家登录/登出广播消息                 |
| TASK\_\_PLAYER_LOGIN_MESSAGE  |           ""            | 文本 |                 玩家登录广播消息内容                 |
| TASK\_\_PLAYER_LOGOUT_MESSAGE |           ""            | 文本 |                 玩家登出广播消息内容                 |
|                               |                         |      |                                                      |
|        RCON\_\_ADDRESS        |   "127.0.0.1:25575"   | 文本 |                  官方 RCON 服务地址                  |
|       RCON\_\_PASSWORD       |           ""            | 文本 |           服务器配置文件中的 AdminPassword           |
|      RCON\_\_USE_BASE64      |          false          | 布尔 |         仅用于兼容支持 Base64 命令的 RCON 代理         |
|        RCON\_\_TIMEOUT        |            5            | 数字 |                  单个命令的超时时间                  |
|                               |                         |      |                                                      |
|        REST\_\_ADDRESS        | "http://127.0.0.1:8212" | 文本 |       服务 REST API 对应的地址，可以用容器网络       |
|       REST\_\_PASSWORD        |           ""            | 文本 |           服务器配置文件中的 AdminPassword           |
|        REST\_\_TIMEOUT        |            5            | 数字 |                  单个请求的超时时间                  |
|                               |                         |      |                                                      |
|         SAVE\_\_PATH          |           ""            | 文本 |    游戏存档所在路径 **请务必填写为容器内的路径**     |
|      SAVE\_\_DECODE_PATH      |     "/app/sav_cli"      | 文本 |    ⚠️ 容器内置，禁止修改，会导致存档解析工具错误     |
|     SAVE\_\_SYNC_INTERVAL     |           600           | 数字 |                同步玩家存档数据的间隔                |
|    SAVE\_\_BACKUP_INTERVAL    |            0            | 数字 | 游戏已有日常备份；设置大于 0 才额外启用 PST 周期备份 |
|   SAVE\_\_BACKUP_KEEP_DAYS    |            7            | 数字 |            自动备份玩家存档数据的保留天数            |
| MANAGE\_\_KICK_NON_WHITELIST  |          false          | 布尔 |        当检测到玩家不在白名单却在线时自动踢出        |

#### Agent 部署

需要两个容器，分别是 `palworld-server-tool` 和 `palworld-server-tool-agent`

适用于：

- 需要单独部署在其它服务器
- 只需要部署在本地个人电脑
- 游戏服务器性能较弱不满足，采用上述两种方案之一

##### 先运行 agent 容器

> 注意:使用交换分区,可能导致程序性能下降,建议仅在内存不足时使用

```bash
docker run -d --name pst-agent \
-p 8081:8081 \
-v /path/to/your/Pal/Saved:/game \
-e SAVED_DIR="/game" \
jokerwho/palworld-server-tool-agent:latest
```

需要 -v 到游戏存档文件（Level.sav）所在目录，将其映射到容器内的 /game 目录

|  变量名   | 默认值 | 类型 |                           说明                           |
| :-------: | :----: | :--: | :------------------------------------------------------: |
| SAVED_DIR |   ""   | 文本 | 游戏存档 Saved 目录所在路径 **请务必填写为容器内的路径** |

##### 再运行 pst 容器

```bash
docker run -d --name pst \
-p 8080:8080 \
-v ./backups:/app/backups \
-e WEB__PASSWORD="your password" \
-e REST__ADDRESS="http://游戏服务器IP:8212" \
-e REST__PASSWORD="your admin password" \
-e SAVE__PATH="http://游戏服务器IP:Agent端口/sync" \
-e SAVE__SYNC_INTERVAL=120 \
jokerwho/palworld-server-tool:latest
```

##### 持久化

如果需要持久化 `pst.db` 文件：

```bash
# 先创建文件，避免被识别为目录
touch pst.db
```

然后在 `docker run -v` 中增加 `-v ./pst.db:/app/pst.db`

##### 环境变量

> [!WARNING]
> 注意区分单个和多个下划线，若需修改最好请复制下表变量名！

|            变量名             |         默认值          | 类型 |                                    说明                                     |
| :---------------------------: | :---------------------: | :--: | :-------------------------------------------------------------------------: |
|        WEB\_\_PASSWORD        |           ""            | 文本 |                          Web 界面的管理员模式密码                           |
|          WEB\_\_PORT          |          8080           | 数字 |                **若非必要不建议修改，而是更改容器映射端口**                 |
|                               |                         |      |                                                                             |
|                               |                         |      |                                                                             |
|     TASK\_\_SYNC_INTERVAL     |           60            | 数字 |                      请求服务器同步玩家在线数据的间隔                       |
|    TASK\_\_PLAYER_LOGGING     |          false          | 布尔 |                            玩家登录/登出广播消息                            |
| TASK\_\_PLAYER_LOGIN_MESSAGE  |           ""            | 文本 |                            玩家登录广播消息内容                             |
| TASK\_\_PLAYER_LOGOUT_MESSAGE |           ""            | 文本 |                            玩家登出广播消息内容                             |
|                               |                         |      |                                                                             |
|        RCON\_\_ADDRESS        |   "127.0.0.1:25575"   | 文本 |                            官方 RCON 服务地址                              |
|       RCON\_\_PASSWORD       |           ""            | 文本 |                      服务器配置文件中的 AdminPassword                       |
|      RCON\_\_USE_BASE64      |          false          | 布尔 |                   仅用于兼容支持 Base64 命令的 RCON 代理                    |
|        RCON\_\_TIMEOUT        |            5            | 数字 |                            单个命令的超时时间                               |
|                               |                         |      |                                                                             |
|        REST\_\_ADDRESS        | "http://127.0.0.1:8212" | 文本 |                  服务 REST API 对应的地址，可以用容器网络                   |
|       REST\_\_PASSWORD        |           ""            | 文本 |                      服务器配置文件中的 AdminPassword                       |
|        REST\_\_TIMEOUT        |            5            | 数字 |                             单个请求的超时时间                              |
|                               |                         |      |                                                                             |
|         SAVE\_\_PATH          |           ""            | 文本 | pst-agent 所在服务地址，格式为<br> http://{游戏服务器 IP}:{Agent 端口}/sync |
|      SAVE\_\_DECODE_PATH      |     "/app/sav_cli"      | 文本 |                ⚠️ 容器内置，禁止修改，会导致存档解析工具错误                |
|     SAVE\_\_SYNC_INTERVAL     |           600           | 数字 |                           同步玩家存档数据的间隔                            |
|    SAVE\_\_BACKUP_INTERVAL    |            0            | 数字 |             游戏已有日常备份；设置大于 0 才额外启用 PST 周期备份             |
|   SAVE\_\_BACKUP_KEEP_DAYS    |            7            | 数字 |                       自动备份玩家存档数据的保留天数                        |
|                               |                         |      |                                                                             |
| MANAGE\_\_KICK_NON_WHITELIST  |          false          | 布尔 |                   当检测到玩家不在白名单却在线时自动踢出                    |

#### 从 k8s-pod 同步存档

从 v0.5.3 开始，支持无需 agent 同步集群内游戏服务器存档。

> v0.5.8 之后，由于增加了玩家背包数据查看，复制的是整个 Sav 文件的目录，须确保帕鲁服务端容器内具有 tar 工具才能压缩和解压

> 请确保 pst 所使用的 serviceaccount 具有 "pods/exec" 权限！

只需要更改 `SAVE__PATH` 环境变量即可，格式如下：

```bash
SAVE__PATH="k8s://<namespace>/<podname>/<container>:<游戏存档目录>"
```

比如：

```bash
SAVE__PATH="k8s://default/palworld-server-0/palworld-server:/palworld/Pal/Saved
```

> 由于游戏服务器创建 Level.sav 文件的时间、位置（包含 HASH）在初次都不确定，您只需要指向 Saved 目录级别即可，程序会自动扫描

当 pst 与游戏服务器在同一 namespace 下时，您可以省略它：

```bash
SAVE__PATH="k8s://palworld-server-0/palworld-server:/palworld/Pal/Saved
```

### 从 docker 容器同步存档

从 v0.5.3 开始，支持无需 agent 同步容器内游戏服务器存档

#### 文件部署使用

当你的 pst 本体是通过运行二进制文件部署时，只需要修改 `config.yaml` 中的 `save.path` 即可：

```yaml
save:
  path: "docker://<container_name_or_id>:<游戏存档目录>"
```

比如：

```yaml
save:
  path: docker://palworld-server:/palworld/Pal/Saved
# or
save:
  path: docker://04b0a9af4288:/palworld/Pal/Saved
```

#### Docker 部署使用

如果 pst 本体是通过 Docker 单体部署的，那么你需要修改 `SAVE__PATH` 环境变量，并且将 Docker 守护进程挂载至 pst 容器内

1. 挂载守护进程

在原来的 `docker run` 命令中，增加一行 `-v /var/run/docker.sock:/var/run/docker.sock`

2. 修改环境变量

更改 `SAVE__PATH` 环境变量，格式如下：

```bash
SAVE__PATH="docker://<container_name_or_id>:<游戏存档目录>"
```

比如：

```bash
SAVE__PATH="docker://palworld-server:/palworld/Pal/Saved"
#or
SAVE__PATH="docker://04b0a9af4288:/palworld/Pal/Saved"
```

> [!WARNING]
> 如果在运行后看到如 ` Error response from daemon: client version 1.44 is too new. Maximum supported API version is 1.43` 的报错，是因为你当前 docker engine 使用的 Docker API 版本较低，这时候请再增加一个环境变量：
>
> -e DOCKER_API_VERSION="1.43" (你的 API 版本)

> 由于游戏服务器创建 Level.sav 文件的时间、位置（包含 HASH）在初次都不确定，您只需要指向 Saved 目录级别即可，程序会自动扫描

## 项目状态

![Stats](https://repobeats.axiom.co/api/embed/8724e69c284e0645f764a4a1cd525477be13cbe8.svg "Repobeats analytics image")

## 接口文档

[APIFox 在线接口文档](https://q4ly3bfcop.apifox.cn/)

## 感谢

- [PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) 的 `palsav-flex` 提供当前存档解析、Oodle 压缩与重建能力
- [palworld-server-toolkit](https://github.com/magicbear/palworld-server-toolkit) 提供了存档高性能解析部份实现
- [pal-conf](https://github.com/Bluefissure/pal-conf) 提供最新服务端配置项与中文名称参考
- [PalEdit](https://github.com/EternalWraith/PalEdit) 提供了最初的数据化思路及逻辑

## 许可证

主程序根据 [Apache2.0 许可证](LICENSE) 授权。独立进程 `sav_cli` 包含 GPL-3.0-or-later 组件，其许可证随发行包以 `sav_cli-GPL-3.0.txt` 提供。
