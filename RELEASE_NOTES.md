# Palworld Server Tool v1.2.0

此版本面向 Palworld 1.0.0，新增安全的游戏配置直写与服务器生命周期控制，同时修复真实玩家存档同步、可选 GameData 404 和 RCON 易用性问题。

## 主要更新

- 配置页可直接读取、可视化编辑并写回 `PalWorldSettings.ini`；写入前会校验文件摘要、备份旧配置并原子替换，避免旧页面误覆盖新文件。
- 新增游戏服务器启动、保存并重启和状态显示，支持本机进程、Docker 容器、systemd unit 与 Windows 服务四种受限驱动。
- Palworld 1.0.0 已自带世界备份，因此 PST 周期存档备份默认关闭；玩家与帕鲁存档编辑前的强制安全备份继续保留。
- 修复真实 1.0.0 玩家存档同步误报 `Player save does not contain SaveData` 并中断整个同步的问题。
- 未启用 `PalGameDataBridge` 时，GameData 404 现在显示为可选功能未开启，不再作为系统错误。
- RCON 终端增加服务器信息、在线玩家和保存世界快捷命令，并明确官方已弃用 RCON，常用管理优先使用 REST API。

## 下载文件

- `pst_v1.2.0_windows_x86_64.zip`
- `pst_v1.2.0_linux_x86_64.tar.gz`
- `pst_v1.2.0_linux_aarch64.tar.gz`
- 对应平台的 `pst-agent` 独立程序
- `SHA256SUMS.txt`

完整包包含主程序、对应平台的 `sav_cli`、GPL 许可证、示例配置和启动脚本。

## 升级与配置

1. 停止旧版 PST，并备份 `config.yaml`、数据库和世界存档目录；不要用发布包中的示例配置覆盖现有文件。
2. 若要直接编辑游戏配置，请设置 `palworld.config_path` 为 PST 本机可访问的 `PalWorldSettings.ini`。
3. 若要使用启动和重启，请设置 `palworld.control.mode` 与 `target`；支持值为 `process`、`docker`、`systemd`、`windows_service`。
4. `process` 模式的 `target` 必须是绝对可执行文件路径；管理接口不会执行任意 Shell 文本。
5. 游戏配置写入后需要重启 Palworld 服务端才会生效。
6. 若希望额外保留 PST 周期备份，可将 `save.backup_interval` 从默认 `0` 改为所需秒数。

详细变更见 [`CHANGELOG.md`](https://github.com/xutongxue233/palworld-server-tool/blob/main/CHANGELOG.md)。
