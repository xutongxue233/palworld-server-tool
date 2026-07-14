# Palworld Server Tool v1.9.0

此版本继续面向 Palworld 1.0.0，统一 Windows 与 Linux 的 Web 管理方案，将运行配置迁入数据库，并加入服务端自动发现流程。同时修复 PalServer 停止后玩家物品与存档编辑被错误阻止的问题。

## 主要更新

- Windows 与 Linux 使用同一套嵌入式 Web 管理界面；Windows 发布包继续提供可直接运行的 `pst.exe`，不依赖桌面 WebView 运行时。
- 运行配置存入 `pst.db`，发布包不再包含或依赖 `config.yaml`。已有配置会在数据库为空时自动导入一次，之后由数据库统一管理。
- 首次启动会自动扫描本机 PalServer：优先识别正在运行的服务端进程，并检查 Steam 注册表、Steam 库、SteamCMD 常用目录、环境路径和已有数据库路径。
- 自动推导启动器、`PalWorldSettings.ini`、活动世界、REST/RCON 端口及 SteamCMD 路径；存在多个候选时可在界面选择，完全找不到时才要求手动指定。
- 新增受认证保护的发现与数据库配置 API，并同步更新三语界面、README 与 Swagger 文档。
- 修复停服编辑判断：托管服务已停止时允许编辑；非托管服务可在明确确认停服后编辑；REST 端点仍可达时继续安全阻止写入。

## 配置迁移

1. 升级前停止旧版 PST，并备份 `config.yaml`、`pst.db` 与世界存档。
2. 将新版程序放入原工作目录后启动。仅当数据库配置为空时，PST 才会读取并导入旧的 `config.yaml` 或 `config.yml`。
3. 旧 YAML 会保留用于回退，但后续启动只读取 `pst.db`；发布包中不再附带示例 YAML。
4. 新安装首次启动时会生成随机 Web 管理密码并写入数据库，初始密码会显示在启动日志中。
5. 服务运行期间通过扫描界面或 `PUT /api/setup/config` 保存的路径会在重启 PST 后加载，以避免并发热更新全局配置。

## 自动发现与手动设置

- 单一高可信候选会在启动阶段自动应用。
- 多个接近的候选会显示在首次设置界面，由管理员选择正确的 PalServer。
- 登录后可随时从管理菜单重新扫描或切换本机服务端。
- 手动输入支持 PalServer 安装目录、启动器文件或其内部路径，PST 会向上推导实际安装根目录。

## 下载文件

- `pst_v1.9.0_windows_x86_64.zip`
- `pst_v1.9.0_linux_x86_64.tar.gz`
- `pst_v1.9.0_linux_aarch64.tar.gz`
- 对应平台的 `pst-agent` 独立程序
- `SHA256SUMS.txt`

完整包包含主程序、对应平台的 `sav_cli`、GPL 与第三方许可证以及启动脚本；运行配置由 `pst.db` 保存。

## 升级注意事项

- 不要让两个 PST 进程共享同一个 `pst.db`、备份目录或世界存档。
- 自动发现后请在界面核对活动世界、服务器配置和控制方式，再执行玩家、物品或存档写操作。
- 非托管服务器只有在确认 PalServer 已完全停止后才可使用手动停服确认；如果 REST 仍可连接，PST 会继续拒绝离线编辑。
- 数据库配置 API 不返回 Web、REST/RCON、Fleet 或自动化密钥等敏感值。

详细配置见三语 README，完整变更见 [`CHANGELOG.md`](https://github.com/xutongxue233/palworld-server-tool/blob/main/CHANGELOG.md)。
