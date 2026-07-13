# 更新记录

本项目采用语义化版本号。重大不兼容调整会提升主版本号。

## [Unreleased]

## [1.1.0] - 2026-07-13

### 变更

- 配置编辑器恢复 Palworld 1.0.0 官方 `RCONEnabled` 与 `RCONPort` 字段，并纳入管理员密码和端口冲突校验。
- 恢复受 JWT 保护的 RCON 命令接口和管理台终端，支持超时、可选 Base64 兼容模式和多行响应。
- 同步上游 Palworld 1.0.0 中文帕鲁、物品、被动技能名称与对应图标资源。
- 配置编辑器覆盖 1.0.0 官方文档中全部 93 个可用参数，并将文档外的默认 INI/底层字段明确标记为兼容参数。
- 按 1.0.0 配置定义将 `MonsterFarmActionSpeedRate` 的允许范围校正为 `0.1` 至 `5`。

### 修复

- 修复 Palworld 1.0.0 存档中的 `Talent_HP` 未映射，导致帕鲁生命个体值显示为 0 且仍标记为“近战”的问题；生命、攻击和防御字段现按个体值语义展示，旧版 `Talent_Melee` 仍保持兼容。
- 将不依赖外部存档运行时的 Python 存档编辑测试纳入 CI，避免编辑辅助逻辑只做语法检查。

## [1.0.0] - 2026-07-11

### 新增

- 基于 React、TypeScript、Vite 与 shadcn/ui 的完整管理界面。
- 按最新官方服务端配置与 pal-conf 中文定义实现的可视化配置编辑器。
- `sav_cli` 当前存档格式支持，包括校验、JSON 导出、JSON 重建和无损往返测试。
- 官方 REST API 的服务器信息、指标、在线玩家、广播、保存、关闭、踢出、封禁和解封操作。
- 新增 `/api/server/settings`、`/api/server/game-data`、`/api/server/save` 和 `/api/server/stop` 管理接口。
- 白名单和玩家管理新增跨平台 `user_id` 支持，不再依赖 Steam ID 作为唯一身份标识。
- 新增 REST API、JWT、归档解压、服务器接口、白名单服务和存档源错误处理测试。
- Windows x86_64、Linux x86_64、Linux ARM64 自动发布包和统一 SHA-256 校验文件。
- 新的应用图标、浏览器 favicon 与 Windows EXE 图标。

### 变更

- 前端从 Vue 迁移到 React，并重构桌面端和移动端导航、玩家、公会、地图、配置与运维界面。
- 服务端配置字段更新为 117 个非 RCON 官方字段，并统一使用准确中文翻译。
- REST 客户端改为逐请求上下文超时、统一 Basic Auth、类型化响应和结构化错误。
- 玩家踢出、封禁和解封统一优先使用官方 `userId`，并保留 Steam 平台兼容路径。
- JWT 迁移至 `github.com/golang-jwt/jwt/v5`，Docker 客户端迁移至新版 Moby API。
- 地图瓦片和固定点位在构建发布包时从当前数据源刷新，运行时继续使用本地嵌入资源。
- Node.js 更新为 24，pnpm 固定为 11.5.3，Go 构建基线更新为 1.25.12。

### 修复

- 修复新版本 `Level.sav` 解析时出现 `Warning: EOF not reached` 的问题。
- 修复存档源路径无效后仍输出同步成功日志的问题。
- 修复玩家同步失败时误判全部玩家下线，并可能触发错误广播或白名单踢出的行为。
- 修复调度器未正确保存导致停止任务后仍继续运行的问题。
- 修复优雅关服参数校验、可选消息和玩家操作消息透传。
- 修复归档解压中的 Zip Slip、Tar Slip、文件截断、权限和资源释放问题。
- 修复下载请求无超时、未检查 HTTP 状态码和失败文件残留问题。
- 修复 REST 客户端共享超时数据竞争、非 200 成功状态码处理和请求构造错误。
- 修复玩家、IP、账号、数值与多个服务端配置字段的错误翻译。
- 修复移动端内容溢出、旧界面难以操作和配置字段验证信息不准确的问题。

### 安全

- 修复配置加载前初始化 JWT 密钥导致密钥为空、令牌可被伪造的问题。
- 登录密码比较改为恒定时间比较，JWT 校验强制限制为 HMAC SHA-256。
- 主 Go 项目可达漏洞扫描和前端依赖审计纳入持续集成。
- 未认证接口继续隐藏玩家 IP、Steam ID 和平台 User ID 等敏感字段。

### 移除

- 移除 Vue 前端及其旧组件和构建配置。
- 移除已计划弃用的 RCON 界面、后端接口、配置、依赖和文档。
- 移除旧版 pal-conf 子模块和无法解析当前存档的旧 sav-cli 获取流程。

### 升级提示

- 使用 REST 功能时，Palworld 服务端必须启用 `RESTAPIEnabled=True`，并为 PST 配置相同的 `AdminPassword`。
- 替换程序前应停止 PST 和 Palworld 服务端，并备份 `config.yaml`、数据库与整个世界存档目录。
- 不要将 JSON 重建后的存档直接覆盖正在运行的 `Level.sav`。

[Unreleased]: https://github.com/xutongxue233/palworld-server-tool/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/xutongxue233/palworld-server-tool/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/xutongxue233/palworld-server-tool/releases/tag/v1.0.0
