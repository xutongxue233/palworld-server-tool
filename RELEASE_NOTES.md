# Palworld Server Tool v1.5.0

此版本继续严格面向 Palworld 1.0.0，新增新手可直接使用的 SteamCMD 安装与更新工作区。应用 ID 固定为官方 Palworld Dedicated Server `2394010`，整个流程与服务器启停、看门狗和存档安全恢复点共享受限维护边界。

## 主要更新

- 在“运维 → 部署更新”中完成 SteamCMD 首次安装、现有安装更新与文件校验，不需要输入命令。
- 预检会显示 SteamCMD 路径、安装目录、App manifest、当前构建 ID、平台启动文件、世界数量、恢复点状态和服务控制状态。
- 执行过程固定为复核计划、保存并停服、创建恢复点、安装/更新 App 2394010、校验并按需重启五个步骤。
- 读取 `steamapps/appmanifest_2394010.acf` 展示更新前后构建 ID，并在 SteamCMD 成功退出后再次确认 manifest 与 `PalServer.exe`/`PalServer.sh` 有效。
- 已配置 `palworld.control` 时可以安全停止正在运行的服务器，并选择完成后自动启动；没有控制驱动时必须先手动完全停服并明确确认。
- 新增中、英、日三语部署界面、JWT 保护的预检/执行 API 和对应 Swagger 文档。

## 存档保护

- 安装目录中只要存在世界数据，`save.path` 就必须指向该安装目录内的本地世界。
- SteamCMD 启动前强制创建完整 PST 恢复点；备份失败、路径不匹配或服务器仍在运行时会直接中止。
- 停服确认会在备份后、真正运行 SteamCMD 前再次执行，缩小外部进程意外重新启动造成的竞态窗口。
- 更新失败时会使用独立恢复上下文尝试重新启动更新前正在运行的托管服务器，并恢复看门狗目标状态。
- Palworld 1.0.0 的日常恢复仍优先使用游戏自带 `backup/world`；PST 压缩备份继续用于危险维护前的强制恢复点。

## 安全边界

- App ID 永远固定为 `2394010`，不接受自定义 Depot、测试分支、Shell 文本或任意 SteamCMD 参数。
- SteamCMD 可执行文件和安装目录必须是绝对路径；拒绝符号链接、文件系统根目录、Windows 非 `.exe` 文件以及 Linux 缺少执行权限的程序/启动脚本。
- 执行前再次校验计划摘要、SteamCMD SHA-256、manifest 摘要、启动文件大小/时间和备份条件，计划变化会安全中止。
- 更新决策不依赖第三方“最新版本”服务，只信任本地 Steam manifest 与 SteamCMD 执行结果。
- SteamCMD 使用直接参数启动而不是 Shell，运行最长 60-7200 秒，输出只保留有界末尾。
- SteamCMD 维护持有全局服务器操作锁并暂停看门狗，避免与存档编辑、原生备份恢复、配置写入、RCON 或定时任务重叠。

## 下载文件

- `pst_v1.5.0_windows_x86_64.zip`
- `pst_v1.5.0_linux_x86_64.tar.gz`
- `pst_v1.5.0_linux_aarch64.tar.gz`
- 对应平台的 `pst-agent` 独立程序
- `SHA256SUMS.txt`

完整包包含主程序、对应平台的 `sav_cli`、GPL 与第三方许可证、示例配置和启动脚本。

## 升级与配置

1. 停止旧版 PST，并备份现有 `config.yaml`、`pst.db` 和世界存档目录；不要用发布包中的示例配置覆盖现有文件。
2. 旧配置可直接升级。只有需要使用 SteamCMD 时，才新增 `steamcmd.executable`、`steamcmd.install_dir` 和可选 `steamcmd.timeout`。
3. `steamcmd.executable` 与 `steamcmd.install_dir` 必须是 PST 主机或容器内可访问的绝对路径；远程 agent 路径不能用于本机 SteamCMD。
4. 若安装目录中已有世界，请确认 `save.path` 指向其中的活动世界；页面必须显示“存档恢复点”已就绪。
5. 推荐配置 `palworld.control`，这样更新时可以保存、停服并选择自动启动。否则先在宿主系统中完全停止 PalServer。
6. 默认开启 `validate`。大型安装或较慢磁盘可在 `steamcmd.timeout` 中把上限调整到 7200 秒以内。

详细变更见 [`CHANGELOG.md`](https://github.com/xutongxue233/palworld-server-tool/blob/main/CHANGELOG.md)。
