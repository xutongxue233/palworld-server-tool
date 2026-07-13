# Palworld Server Tool v1.4.0

此版本继续严格面向 Palworld 1.0.0，新增面向新手的类型化定时任务、服务器看门狗和结构化通知。所有自动化动作都经过白名单校验，并与已有的启停、游戏原生备份恢复和 `WorldOption.sav` 安全写入共享互斥边界。

## 主要更新

- 在“运维 → 自动化”中直接创建间隔、每天或每周任务，不需要学习 Cron；支持保存世界、发送公告、启动、安全停止、保存并重启、同步存档解析和额外 PST 安全备份。
- 提供每小时保存、每天维护重启、每周安全备份和定时存档同步模板；任务与设置保存在 `pst.db`，PST 重启后会自动恢复。
- 服务器看门狗同时检查受限控制驱动的进程状态和 Palworld REST `/info`，并通过失败阈值、启动宽限、冷却与最大恢复次数限制自动恢复。
- 人工或类型化任务主动停止服务器时会持久记录“允许停机”，看门狗不会把管理员明确停止的服务器重新拉起。
- 支持通用 JSON 和 Discord Webhook，可选择任务、人工启停、服务器异常、恢复成功和恢复失败事件；通用 Webhook 可使用 HMAC-SHA256 签名。
- 新增运行记录和维护状态轨道，最多保留最近 500 次结果；服务控制状态会显示独占维护操作是否正在执行。
- 新增完整自动化 REST API 与 Swagger 文档，并为中、英、日界面补齐错误提示和操作说明。

## 安全边界

- 定时任务只接受七种固定动作及强类型参数，不接受任意 Shell、任意 Cron 文本或任意 RCON 命令。
- 人工控制、RCON、离线存档编辑、配置写入、旧版周期任务、自动化任务、原生备份恢复和 WorldOption 同步共享服务器操作锁，避免保存、停止、恢复和配置写入互相覆盖。
- RCON 的 `Shutdown` 与 `DoExit` 会记录有意停机；Discord 通知禁用所有用户、角色和 `@everyone` 提及。
- Webhook 默认仅允许公网 HTTPS，拒绝重定向、系统代理、localhost、私网、CGNAT、基准测试/文档网段、链路本地、多播、IPv6 过渡网段和 DNS 解析后的受限地址。
- 私网 Webhook 只能由本机管理员在 `config.yaml` 中显式开启；Web UI 不能放宽该边界。
- Webhook 完整路径、令牌和签名密钥不会通过读取 API 回显；通知只出站发送状态事件，不接收远程控制命令。
- 看门狗必须使用 `palworld.control` 的受限 `process`、`docker`、`systemd` 或 `windows_service` 驱动，不会执行用户提供的 Shell 文本。

## 下载文件

- `pst_v1.4.0_windows_x86_64.zip`
- `pst_v1.4.0_linux_x86_64.tar.gz`
- `pst_v1.4.0_linux_aarch64.tar.gz`
- 对应平台的 `pst-agent` 独立程序
- `SHA256SUMS.txt`

完整包包含主程序、对应平台的 `sav_cli`、GPL 与第三方许可证、示例配置和启动脚本。

## 升级与配置

1. 停止旧版 PST，并备份现有 `config.yaml`、`pst.db` 和世界存档目录；不要用发布包中的示例配置覆盖现有文件。
2. 旧配置可以直接升级。自动化首次使用默认关闭；进入 Web 管理模式后可在“运维 → 自动化”中创建任务和配置通知。
3. 若要使用启动、重启或看门狗，请先配置 `palworld.control.mode` 与 `target`。看门狗在未配置受限控制驱动时会拒绝启用。
4. 看门狗需要可用的 Palworld REST API；请确认 `rest.address`、管理员密码和游戏端 `RESTAPIEnabled=True`。
5. 通用/Discord Webhook 默认必须是公网 HTTPS。只有明确受信任的内网接收端才应在本地配置中设置 `automation.notification.allow_private_network: true`。
6. Palworld 1.0.0 的日常存档恢复仍优先使用游戏自带 `backup/world`；PST 额外备份用于危险写入前恢复点或明确创建的计划任务。

详细变更见 [`CHANGELOG.md`](https://github.com/xutongxue233/palworld-server-tool/blob/main/CHANGELOG.md)。
