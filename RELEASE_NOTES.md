# Palworld Server Tool v1.7.0

此版本继续严格面向 Palworld 1.0.0，新增 Windows Dedicated Server 官方 MOD 加载器的安全状态管理。PST 只读取本机官方格式元数据并修改 `PalModSettings.ini`，不会替用户下载、解压或执行任何 MOD 内容。

## 主要更新

- 在“运维 → MOD 管理”中查看 `<PalServer>/Mods/Workshop/<任意目录>/Info.json` 包目录，无需手工整理 `ActiveModList`。
- 展示 PackageName、版本、作者、标签、依赖、安装类型、服务器兼容性、已部署、等待重启与等待移除状态。
- 以 `Info.json → ActiveModList → InstallManifest` 流程说明官方加载器的实际生效阶段。
- 选择服务端 MOD 时自动加入已发现的依赖；缺失、重复、无效或未启用的依赖会在预检中阻止执行。
- 检测 `-NoMods` 与 `-workshopdir` 启动覆盖，避免页面设置与实际启动行为不一致。
- 新增中、英、日三语界面、JWT 保护的状态/预检/执行 API 和 Swagger 文档。

## 官方 1.0.0 兼容范围

- 仅在 Windows Dedicated Server 上允许写入；Linux 会明确显示官方服务端加载器不受支持。
- 支持官方五种安装类型：UE4SS、Lua、PalSchema、LogicMods、Paks。
- 服务端包必须包含 `IsServer: true` 和至少一个留在包目录内的安装目标。
- 默认扫描 `<PalServer>/Mods/Workshop`，可通过 `WorkshopRootDir` 使用另一个本机绝对目录。
- 根据 `<PalServer>/Mods/ManagedMods/<PackageName>/InstallManifest.json` 判断重启后是否已由游戏部署。

## 安全应用与失败恢复

- 执行前重新扫描全部包并核对设置、`Info.json` 哈希、部署状态与计划摘要；目录或元数据改变后必须重新预检。
- 变更会独占服务器维护锁、暂停看门狗并确认 PalServer 完全停止。
- 已有世界必须让本机 `save.path` 指向同一安装目录内的世界，并先创建完整 PST 安全恢复点；备份失败不会修改设置。
- 每次写入前单独保存 `PalModSettings.ini` 恢复点；同目录暂存、原子替换并回读验证，失败自动恢复旧文件。
- 选择自动重启后若新 MOD 配置无法启动，PST 会停下失败实例、回滚旧设置，再尝试启动原配置。
- MOD 仍可能导致服务器崩溃或损坏存档；确认框要求显式接受风险，并应继续保留游戏自带 `backup/world`。

## 明确不会执行的操作

- 不访问 MOD 下载 URL，不调用 Steam Workshop 下载器，不接受任意 Shell 或安装命令。
- 不解压 ZIP，不复制 DLL/Pak/Lua 内容，不运行 MOD 自带程序。
- 不绕过 `-NoMods`，也不会静默修改 `palworld.control.arguments`。
- 不把未知安装类型、客户端专用包、目录穿越目标或符号链接包当作可用服务端 MOD。

## 下载文件

- `pst_v1.7.0_windows_x86_64.zip`
- `pst_v1.7.0_linux_x86_64.tar.gz`
- `pst_v1.7.0_linux_aarch64.tar.gz`
- 对应平台的 `pst-agent` 独立程序
- `SHA256SUMS.txt`

完整包包含主程序、对应平台的 `sav_cli`、GPL 与第三方许可证、示例配置和启动脚本。

## 升级与使用

1. 停止旧版 PST，并备份现有 `config.yaml`、`pst.db` 和世界存档目录；不要用发布包中的示例配置覆盖现有文件。
2. 旧配置可直接升级。若已配置 `steamcmd.install_dir`，MOD 管理会自动复用；否则添加可选的 `mods.install_dir`，指向包含 `PalServer.exe` 的 Windows 专用服务器绝对目录。
3. 把经过审核、符合官方格式的包放入 `Mods/Workshop/<任意目录>/Info.json`；PST 不负责获取 MOD 文件。
4. 已有世界请确认 `save.path` 指向该安装内的本机世界，并先检查游戏自带备份可用。
5. 在“MOD 管理”中先运行只读预检，核对依赖、安装类型、变更项和恢复点条件，再显式确认 MOD 风险。
6. 若启动参数包含 `-NoMods`，页面选择不会实际生效；若包含 `-workshopdir`，应先确认该覆盖目录正是预期目录。
7. 推荐配置 `palworld.control` 以自动停服、重启和失败恢复；未配置时必须在主机系统中完全停止 PalServer，并在页面明确确认。

官方格式参考：[Palworld 1.0.0 MOD 说明](https://docs.palworldgame.com/settings-and-operation/mod) 与 [PalworldModUploader](https://github.com/pocketpairjp/PalworldModUploader)。详细变更见 [`CHANGELOG.md`](https://github.com/xutongxue233/palworld-server-tool/blob/main/CHANGELOG.md)。
