# Palworld Server Tool v1.1.0

此版本面向 Palworld 1.0.0，补齐官方 RCON 管理能力，更新当前游戏数据，并修复存档个体值兼容问题。

## 主要更新

- 恢复受 Web 管理 JWT 认证保护的 RCON 命令接口与管理台终端。
- 配置编辑器加入官方 `RCONEnabled`、`RCONPort`，并覆盖 1.0.0 官方文档全部 93 个可用参数。
- 同步 Palworld 1.0.0 中文帕鲁、物品、被动技能名称与对应图标。
- 修复 1.0.0 存档 `Talent_HP` 未映射导致生命个体值显示错误的问题，同时兼容旧版 `Talent_Melee`。
- 将生命、攻击和防御字段按个体值语义显示，并修正牧场生产速度配置范围。
- 扩充 Python 存档编辑测试，更新 Swagger、配置示例和中英文使用说明。

## 下载文件

- `pst_v1.1.0_windows_x86_64.zip`
- `pst_v1.1.0_linux_x86_64.tar.gz`
- `pst_v1.1.0_linux_aarch64.tar.gz`
- 对应平台的 `pst-agent` 独立程序
- `SHA256SUMS.txt`

完整包包含主程序、对应平台的 `sav_cli`、GPL 许可证、示例配置和启动脚本。

## 升级前注意

1. 停止旧版 PST 和 Palworld 服务端，并备份 `config.yaml`、数据库及整个世界存档目录。
2. 不要用发布包中的示例 `config.yaml` 覆盖自己的配置。
3. REST API 与 RCON 共用 Palworld 的 `AdminPassword`，请设置强密码并只向受信任网络开放管理端口。
4. 使用 RCON 终端前，在 `PalWorldSettings.ini` 中启用 `RCONEnabled=True` 并确认 `RCONPort`。
5. Linux 用户需要为 `pst`、`sav_cli` 和 `start.sh` 保留可执行权限。

详细变更见 [`CHANGELOG.md`](https://github.com/xutongxue233/palworld-server-tool/blob/main/CHANGELOG.md)。
