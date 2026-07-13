# Palworld Server Tool v1.8.0

此版本继续严格面向 Palworld 1.0.0，新增安全的多服务器管理。架构采用“一个 PalServer 对应一个独立 PST 节点，另由中央 PST 聚合”的方式：不会在同一个进程中切换全局配置、数据库或任务状态，因此不同世界的存档、备份、看门狗和危险操作不会串服。

## 主要更新

- 顶部节点选择器和概览服务器轨道可展示最多 32 个 PST 节点的在线状态、玩家数、FPS、延迟、控制状态、工具版本和配置问题。
- 切换节点后，概览、玩家、公会、地图、配置、RCON、备份、自动化、部署更新、MOD 与存档迁移页面都会操作目标服务器。
- 所有查询缓存按节点隔离，自动重试与延迟刷新也绑定原节点；有写操作执行时会锁定节点选择，避免完成回调或刷新落到另一个世界。
- 中央节点并发聚合远程状态；远端 PST 在线但 PalServer 离线时仍可进入该节点排障，身份或协议不匹配时则阻止选择。
- 新增完整三语部署文档、配置示例、Fleet API、Swagger 和覆盖认证、代理、正文/查询透传及缓存隔离的回归测试。

## 一服一 PST 的隔离模型

- 每个 PalServer 使用独立 PST 进程、工作目录、`config.yaml`、`pst.db`、`web.port`、存档路径和备份目录。
- 每个节点保留自己的调度器、看门狗、维护互斥锁及危险操作恢复事务；不同节点可并行执行互不相关的维护。
- 中央 PST 只读取节点健康状态，并通过固定 API 白名单转发现有操作，不会热替换 Viper 配置、数据库句柄或后台任务单例。
- 同一主机可运行多个节点，但禁止两个 PST 进程共享 `pst.db`、备份目录或同一个世界存档。

## 配置示例

在被管理节点配置稳定身份和至少 32 个随机字符的入站令牌：

```yaml
web:
  port: 8081

fleet:
  node_id: "second"
  node_name: "第二世界"
  node_token: "replace-with-at-least-32-random-characters"
  nodes: []
```

在中央 PST 加入远程节点，`id` 和 `token` 必须分别匹配远端的 `node_id` 与 `node_token`：

```yaml
fleet:
  node_id: "primary"
  node_name: "主世界"
  timeout_seconds: 15
  nodes:
    - id: "second"
      name: "第二世界"
      base_url: "https://palworld-second.example.com:8081"
      token: "replace-with-at-least-32-random-characters"
      allow_private_network: false
      timeout_seconds: 15
```

公网节点必须使用有效 HTTPS。只有可信回环、局域网或私有组网地址才应设置 `allow_private_network: true`；该开关也代表明确接受普通 HTTP 的明文传输风险。

## 认证与代理安全

- 节点令牌限制为 32-512 个无空白可打印 ASCII 字符，通过专用 `X-PST-Fleet-Token` 发送并使用恒定时间比较。
- 浏览器只保存中央 JWT；远端节点令牌永不返回前端，中央 JWT 也不会转发到远端。
- 节点客户端不使用系统代理、不跟随重定向，要求 TLS 1.2 以上；连接时复核全部 DNS/IP、依次尝试已校验地址，并限制 DNS、连接、响应头与正文在内的整个请求时长。
- 代理仅放行当前 PST 管理界面需要的精确方法与路径，拒绝登录代理、Fleet 递归、未知路由、目录穿越和异常路径段。
- 请求正文上限为 8 MiB，只透传必要头部；远端认证错误映射为网关错误，不会清除中央登录会话。

## 下载文件

- `pst_v1.8.0_windows_x86_64.zip`
- `pst_v1.8.0_linux_x86_64.tar.gz`
- `pst_v1.8.0_linux_aarch64.tar.gz`
- 对应平台的 `pst-agent` 独立程序
- `SHA256SUMS.txt`

完整包包含主程序、对应平台的 `sav_cli`、GPL 与第三方许可证、示例配置和启动脚本。

## 升级与使用

1. 停止旧版 PST，备份现有 `config.yaml`、`pst.db` 与世界存档；不要用发布包内的示例配置覆盖旧配置。
2. 单服务器用户可直接升级，未配置 `fleet` 时仍只显示本地节点，原有功能不受影响。
3. 每个额外 PalServer 复制一套独立 PST 工作目录并分配不同 `web.port`，确认 REST、存档、控制、SteamCMD 与 MOD 路径只指向对应服务器。
4. 为远端节点生成强随机 `node_token`，在中央 PST 添加匹配配置；不要在聊天、日志或浏览器脚本中暴露令牌。
5. 修改配置后重启对应 PST，在中央界面确认节点身份、协议版本、延迟与配置问题，再执行任何写操作。
6. 跨公网部署应使用受信任证书的 HTTPS；私网明文模式只适用于受控网络，并应同时使用主机防火墙限制来源。
7. 危险操作仍应检查游戏自带 `backup/world`，并保留 PST 强制恢复点；多服务器聚合不会降低单节点的停服、校验和原子替换要求。

详细配置见三语 README，完整变更见 [`CHANGELOG.md`](https://github.com/xutongxue233/palworld-server-tool/blob/main/CHANGELOG.md)。
