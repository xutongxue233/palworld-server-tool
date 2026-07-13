# Palworld Server Tool v1.3.0

此版本继续面向 Palworld 1.0.0，重点补齐游戏原生备份恢复、`WorldOption.sav` 覆盖处理和完整 RCON 命令模板。所有存档写入流程都先校验、停服并创建恢复点，再进行原子替换。

## 主要更新

- 备份页可直接发现并逐文件校验当前世界 `backup/world/<时间戳>` 下的游戏原生备份；游戏自带备份作为日常恢复首选，PST 不再重复维护另一套日常快照。
- 新增原生备份安全恢复事务：停服前复核快照、完整内容摘要、恢复前 PST 安全包、同文件系统暂存、原子替换、失败回滚和可选自动重启。
- 配置页会检测活动世界中的 `WorldOption.sav` 并明确提示其优先级；可把已保存的 `PalWorldSettings.ini` 安全生成或同步为 Palworld 1.0.0 世界配置。
- `sav_cli` 新增 `sync-world-option` 模式，支持布尔、整数、浮点、字符串、枚举和枚举数组，写入后执行 GVAS 无损往返与目标字段校验。
- RCON 终端收录 Palworld 1.0.0 官方命令表全部 13 条模板，按用途与风险分组；带参数模板会自动选中首个占位符，快捷按钮只填充、不自动执行。
- 使用用户提供的 9,449 字节真实玩家存档完成技术点编辑、重压缩和校验回归，不再复现 `Player save does not contain SaveData`。

## 安全约束

- 原生备份拒绝符号链接、非普通文件、未知顶层条目和恢复过程中发生变化的快照。
- `WorldOption.sav` 生成固定校验 Pal-Conf 1.0.0 模板、109 个允许字段、已知枚举值、来源提交与 SHA-256；不下载运行远程代码，也不接受任意 Shell。
- 原生恢复和 WorldOption 同步都会先创建完整存档恢复点；现有目标使用摘要防并发覆盖，并以原子替换或不覆盖目标的同文件系统安装完成写入。
- 存档复制和安全备份拒绝符号链接及非普通文件；压缩失败时会清理半成品归档。
- 已配置受限控制驱动时，工具会恢复操作前的服务器运行状态；未配置托管启停时，必须先手动确认 PalServer 已停止。

## 下载文件

- `pst_v1.3.0_windows_x86_64.zip`
- `pst_v1.3.0_linux_x86_64.tar.gz`
- `pst_v1.3.0_linux_aarch64.tar.gz`
- 对应平台的 `pst-agent` 独立程序
- `SHA256SUMS.txt`

完整包包含主程序、对应平台的 `sav_cli`、GPL 与第三方许可证、示例配置和启动脚本。

## 升级与配置

1. 停止旧版 PST，并备份 `config.yaml`、数据库和整个世界存档目录；不要用发布包中的示例配置覆盖现有文件。
2. 将 `save.path` 设置为 PST 本机可访问的当前世界目录，才能发现游戏原生备份、创建安全恢复点和管理 `WorldOption.sav`。
3. 将 `palworld.config_path` 设置为本机可访问的 `PalWorldSettings.ini`，才能读取、写入和同步配置。
4. 若要自动停服及恢复运行状态，请配置 `palworld.control.mode` 与 `target`；支持 `process`、`docker`、`systemd`、`windows_service`。
5. `WorldOption.sav` 会优先于 `PalWorldSettings.ini`；同步后仍需重启 Palworld 服务端才能使用新配置。
6. 若希望额外保留 PST 周期备份，可将 `save.backup_interval` 从默认 `0` 改为所需秒数。

详细变更见 [`CHANGELOG.md`](https://github.com/xutongxue233/palworld-server-tool/blob/main/CHANGELOG.md)。
