<h1 align='center'>PalWorld Server Tool</h1>

<p align="center">
   <a href="/README.md">简体中文</a> | <strong>English</strong> | <a href="/README.ja.md">日本語</a>
</p>

<p align="center">
  <img src="./web/src/assets/app-icon.png" width="112" alt="Palworld Server Tool icon" />
</p>

<p align="center">
  <a href="https://github.com/xutongxue233/palworld-server-tool/releases/latest">Download latest release</a> · <a href="./CHANGELOG.md">Changelog</a>
</p>

<p align='center'> 
  Manage Palworld dedicated servers with a React interface, the official REST API, and current SAV parsing.<br/>
  And it took a long and boring time to i18n...
</p>

<p align='center'>
<img alt="GitHub Release" src="https://img.shields.io/github/v/release/xutongxue233/palworld-server-tool?style=for-the-badge">&nbsp;&nbsp;
<img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white">&nbsp;&nbsp;
<img alt="Python" src="https://img.shields.io/badge/Python-FFD43B?style=for-the-badge&logo=python&logoColor=blue">&nbsp;&nbsp;
<img alt="React" src="https://img.shields.io/badge/React-20232A?style=for-the-badge&logo=react&logoColor=61DAFB">
</p>

![PC](./docs/img/pst-en-1.png)

> The current mobile adaptation is good, you can view [Function Screenshot](#function-screenshot)
>
> Of course, the dark mode is also arranged no problem ~

Features based on parsing of `Level.sav` save files:

- [x] Complete player data
- [x] Player Palworld data
- [x] Guild data
- [x] Player Backpack Item data

Features implemented using official REST API:

- [x] Retrieve server information
- [x] Obtain server metrics
- [x] Online player list
- [x] Kick/ban players
- [x] In-game broadcasting
- [x] Smooth server shutdown with broadcast message

Additional features provided by the tool:

- [x] Visual map management
- [x] Whitelist management
- [x] Discover, validate, and safely restore the game's `backup/world` snapshots
- [x] Direct `PalWorldSettings.ini` editing plus WorldOption override detection and safe generation/synchronization
- [x] Restricted process, Docker, systemd, and Windows service lifecycle control
- [x] JWT-protected RCON terminal with all 13 Palworld 1.0.0 official command templates
- [x] Typed scheduled tasks, an intentional-stop-aware watchdog, and generic/Discord webhook notifications
- [x] SteamCMD install, update, file validation, and optional restart for fixed app ID 2394010
- [x] Windows official 1.0.0 mod-loader inventory, dependency preflight, safe `PalModSettings.ini` editing, and rollback
- [x] One-PST-per-server fleet aggregation, full management-target switching, and a restricted authenticated proxy
- [x] Same-platform dedicated-server save migration with read-only preflight, a mandatory restore point, atomic replacement, and rollback
- [x] Automatic PST safety restore points before dangerous save operations

This tool stores synchronized REST API and Level.sav data in a single bbolt database and exposes it through the management interface.

The operations page also includes a JWT-protected RCON terminal for commands that are not available through the REST API.

Due to limited maintenance and development staff, we welcome front-end, back-end developers, and even data engineers to submit PRs!

## Function screenshot

https://github.com/zaigie/palworld-server-tool/assets/17232619/49abcd34-0752-487e-8588-b6d1834f07d5

### Desktop

|                              |                              |
| :--------------------------: | :--------------------------: |
| ![](./docs/img/pst-en-2.png) | ![](./docs/img/pst-en-3.png) |

![](./docs/img/pst-en-4.png)

### Mobile

<p align="center">
<img src="./docs/img/pst-en-m-1.png" width="30%" /><img src="./docs/img/pst-en-m-2.png" width="30%" /><img src="./docs/img/pst-en-m-3.png" width="30%" />
</p>

## Enable the REST API

The server's official REST API must be enabled to synchronize online players and perform server management actions.

The configuration page can read, validate, and write `PalWorldSettings.ini` directly. When `WorldOption.sav` exists, PST explains the override and can synchronize the saved 1.0.0 configuration after stopping the server, creating a full restore point, and validating a lossless rebuild. It can also generate a missing file from a checksum-pinned 1.0.0 baseline.

First set **Administrator password**

![ADMIN](./docs/img/admin-en.png)

Then enable the **REST API**

## Optional: enable RCON

Palworld 1.0.0 can still use RCON, but the official documentation marks it as deprecated; prefer REST API actions for routine administration. For legacy servers or plugin commands, set `RCONEnabled=True`, confirm `RCONPort`, and configure a non-empty `AdminPassword` in `PalWorldSettings.ini`. Discovery reads these values into `pst.db`; rescan from the management menu after changing them. Non-standard addresses can be updated through the authenticated `PUT /api/setup/config` endpoint.

Expose the RCON port only to the PST host or another trusted network. Keep `use_base64` disabled when connecting directly to the official server.

## Automation, watchdog, and notifications

After entering Web management mode, open **Operations → Automation** to create interval, daily, or weekly tasks without learning cron syntax. Actions are restricted to world save, announcement, start, safe stop, save-and-restart, decoded-save synchronization, and an extra PST safety backup. User input is never converted into shell text or an arbitrary RCON command. Tasks, the latest 500 run records, and settings are persisted in `pst.db` and re-registered after PST restarts.

The watchdog checks both the restricted control driver's process state and the Palworld REST `/info` response. Recovery starts only after consecutive failures reach the threshold and is bounded by startup grace, cooldown, and a maximum attempt count. A Stop action from the Web UI or typed scheduler records intentional downtime, so the watchdog does not fight the administrator; starting the server restores the keep-running target.

Offline save edits, configuration writes, RCON, legacy periodic sync/backups, and automated maintenance share one operation lock. RCON `Shutdown` and `DoExit` also record intentional downtime so the watchdog does not fight an administrator command.

Notifications support generic JSON and Discord webhooks with selectable task, lifecycle, unhealthy, and recovery events. Generic endpoints can verify `X-PST-Signature: sha256=<HMAC>`. Public HTTPS destinations are required by default; redirects, localhost, and private-network addresses are rejected. Webhook tokens and signing secrets are never returned by read APIs.

Automation, watchdog, and notification settings are saved directly to `pst.db` from the Web UI. The watchdog requires `palworld.control`, normally populated by discovery or the database configuration API. The design was informed by [palworld-server-docker's scheduled backup and Discord notification flows](https://github.com/thijsvanloef/palworld-server-docker) and [TRRabbit Palworld Server Manager's Scheduler/Guardian UX](https://github.com/TRRabbit/palworld-server-manager), while PST keeps its own allowlisted actions, operation lock, and outbound-only notification boundary.

The watchdog is disabled by default, with a failure threshold of 3, a 90-second startup grace period, and a 120-second recovery cooldown. All of these values are editable under **Operations → Automation**.

## Multi-server nodes

PST 1.8.0 manages multiple Palworld 1.0.0 servers as **one isolated PST node per server plus a central controller**. Each PalServer keeps its own PST process, `pst.db`, save directory, backups, scheduler, watchdog, and maintenance lock. The controller only aggregates node health and forwards existing APIs through a fixed allowlist. Different worlds can therefore run maintenance concurrently while dangerous operations remain protected by the target node's own lock and recovery transaction; global configuration is never swapped inside one process.

When several nodes run on the same host, give every node a **different PST working directory** and `web.port`. For example, use `pst-primary/` and `pst-second/`, each containing its own executable, configuration, and database, with REST, save, control, SteamCMD, and mod paths pointing only to that PalServer instance. Never share `pst.db`, the backup directory, or one world save between two PST processes.

Configure the managed node through `PUT /api/setup/config` with an identity and an inbound token containing at least 32 random characters:

```json
{
  "values": {
    "web.port": 8081,
    "fleet.node_id": "second",
    "fleet.node_name": "Second World",
    "fleet.node_token": "replace-with-at-least-32-random-characters",
    "fleet.nodes": []
  }
}
```

Add that node to the controller's database configuration. `id` must equal the remote `node_id`, and `token` must equal the remote `node_token`:

```json
{
  "values": {
    "fleet.node_id": "primary",
    "fleet.node_name": "Primary World",
    "fleet.node_token": "optional-token-if-this-node-is-also-controlled-remotely",
    "fleet.timeout_seconds": 15,
    "fleet.nodes": [
      {
        "id": "second",
        "name": "Second World",
        "base_url": "https://palworld-second.example.com:8081",
        "token": "replace-with-at-least-32-random-characters",
        "allow_private_network": false,
        "timeout_seconds": 15
      }
    ]
  }
}
```

After signing in to the controller, the header selector and overview rail show up to 32 remote PST nodes with reachability, players, FPS, latency, control state, and configuration issues. Selecting a node scopes the overview, players, guilds, map, configuration, RCON, backups, automation, deployment, mods, and migration pages to that server with separate query caches. Switching is disabled while a write mutation is active so completion handlers cannot land on another world.

Public nodes require valid HTTPS. Set `allow_private_network: true` only for a trusted loopback, LAN, or private-overlay address; it also explicitly accepts plain HTTP risk. The node client ignores system proxies, rejects redirects, revalidates every DNS/IP result at connection time, tries validated addresses in order, and bounds the whole request. The controller blocks login and Fleet recursion, unknown methods/paths, traversal, and request bodies larger than 8 MiB. Browsers hold only the controller JWT: remote node tokens are never returned to the frontend, and the controller JWT is never forwarded upstream.

## SteamCMD install and update

The Palworld 1.0.0 dedicated-server app ID is `2394010`. In Web management mode, open **Operations → Deployment** to run the official `app_update 2394010` install/update workflow with file validation enabled by default. PST does not use a third-party “latest version” API and does not accept shell text, a custom app ID, or arbitrary SteamCMD arguments.

```json
{
  "values": {
    "steamcmd.executable": "C:/steamcmd/steamcmd.exe",
    "steamcmd.install_dir": "D:/PalworldServer",
    "steamcmd.timeout": 1800
  }
}
```

Before every run, PST revalidates the SteamCMD hash, install directory, `appmanifest_2394010.acf`, platform launcher, and plan digest. If world data already exists in the install directory, local `save.path` must resolve to a world inside that installation. PST stops the server and creates a mandatory full restore point; SteamCMD is not started if the backup fails. With `palworld.control`, PST can stop and optionally restart the server. Without it, fully stop PalServer in the host system and explicitly confirm that state in the UI.

See the [official Palworld 1.0.0 SteamCMD deployment guide](https://docs.palworldgame.com/getting-started/deploy-dedicated-server).

## Official mod management (Windows)

Palworld 1.0.0 currently supports official server-side mods only on the **Windows Dedicated Server**. In Web management mode, open **Operations → Mods**. PST scans `<PalServer>/Mods/Workshop/<any-folder>/Info.json`, displays package name, version, author, dependencies, install types, and server compatibility, and uses `<PalServer>/Mods/ManagedMods/<PackageName>/InstallManifest.json` to distinguish deployed, pending-restart, and pending-removal states.

```json
{
  "values": {
    "mods.install_dir": "D:/PalworldServer"
  }
}
```

PST follows the official 1.0.0 `WorkshopRootDir`, `bGlobalEnableMod`, and repeated `ActiveModList` format and detects `-NoMods` and `-workshopdir` launch overrides. A package can enter the server list only when it has an `IsServer: true` rule, at least one package-local target, and one of the five documented install types: UE4SS, Lua, PalSchema, LogicMods, or Paks. Missing, duplicate, or inactive dependencies block the change.

PST **does not download, extract, or execute mod content**. Place trusted official-format packages in the Workshop directory yourself. On confirmation, the backend revalidates the plan digest and stops the server. Existing worlds require local `save.path` to resolve inside the same installation; PST first creates a full safety restore point, then backs up and atomically replaces `PalModSettings.ini`, verifies the installed file, and optionally restarts. If a managed restart fails, PST force-stops the failed runtime, restores the previous settings, and attempts to start the old configuration. Server mods can still crash the server or corrupt saves, so review every package and retain game-managed backups.

See the [official Palworld 1.0.0 mod guide](https://docs.palworldgame.com/settings-and-operation/mod) and [PalworldModUploader format documentation](https://github.com/pocketpairjp/PalworldModUploader).

## Safe save migration

In Web management mode, open **Operations → Save migration** and enter an absolute local path from the old server. The path may point to `Level.sav`, a specific world directory, `Saved`, or a PalServer install root. If more than one world is found, select a world directory explicitly. Preflight is read-only: it hashes the critical files and uses the bundled `sav_cli` to verify the Palworld 1.0.0 save class of `Level.sav`, `LevelMeta.sav`, every player save, and optional `WorldOption.sav`.

Automatic migration is intentionally limited to a **same-platform dedicated server → current dedicated server**. These cases are blocked instead of invoking experimental conversion:

- co-op/single-player sources, or a detected host save named `00000000000000000000000000000001.sav`;
- Windows-to-Linux or Linux-to-Windows migration that would require changing player identities/GUIDs;
- remote URLs, Docker/Kubernetes sources, symbolic links, identical source and destination directories, or invalid save structure/classes.

Execution acquires the maintenance lock, pauses the watchdog, saves and stops the server, and creates a mandatory PST restore point for the current world. The source is copied into a transaction directory on the destination filesystem. Only after source and destination digests remain stable and staged saves validate again does PST atomically replace `Level.sav`, `LevelMeta.sav`, `Players/`, and optional `WorldOption.sav`. The current world's game-managed `backup/` tree and unknown top-level entries are preserved; the source `backup/` tree is not imported. If the source has no `WorldOption.sav`, an existing destination copy is removed. Failed post-install validation rolls back automatically. If rollback itself fails, recovery files remain in `.pst-save-migration-*` and the error reports that path. A configured `palworld.control` driver can restart the server; otherwise confirm the manual stop and start it manually afterward.


## Installation and Deployment

- [File Deployment](#file-deployment)
  - [Linux](#linux)
  - [Windows](#windows)
- [Docker Depolyment](#docker-deployment)
  - [Monolithic Deployment](#monolithic-deployment)
  - [Agent Deployment](#agent-deployment)
  - [Synchronizing Archives from k8s-pod](#synchronizing-archives-from-k8s-pod)
- [Synchronizing Archives from Docker Container](#synchronizing-archives-from-docker-container)

> The task of parsing `Level.sav` requires some system memory (often 1GB-3GB) in a short period (<20s) , this portion of memory is released after the parsing task is completed. Ensure your server has enough memory.

Rimer believes that by **putting the pst tool and the game server on the same physical machine**, there are some situations where you might not want to deploy them on the same machine:

- Must be deployed separately on another server
- Only need to deploy on a local PC
- The game server performance is weak and not satisfied, using one of the above two schemes

Please refer to [pst-agent deployment tutorial](./README.agent.en.md) or [Synchronizing Archives from k8s-pod](#synchronizing-archives-from-k8s-pod)

### File Deployment

Download the latest executable files at:

- [Github Releases](https://github.com/zaigie/palworld-server-tool/releases)

#### Linux

##### Download and Extract

```bash
# Download pst_{version}_{platform}_{arch}.tar.gz and extract to the pst directory
mkdir -p pst && tar -xzf pst_v1.9.1_linux_x86_64.tar.gz -C pst
```

##### Configuration

1. Open the directory and allow execution

   ```bash
   cd pst
   chmod +x pst sav_cli
   ```

2. Run `./pst` and open the Web interface. On first access, the page requires you to set and confirm the Web administrator password, then enters admin mode automatically. PST scans running PalServer processes, Steam libraries, and common install locations first; enter a directory only if discovery finds nothing.

   `save.decode_path` may be left empty; PST then uses the bundled `sav_cli` next to the main executable.

   For non-standard systemd, Docker, TLS, or Fleet settings, sign in and call `PUT /api/setup/config` from Swagger. The endpoint accepts dotted keys and stores them in `pst.db`:

   ```json
   {
     "values": {
       "palworld.control.mode": "systemd",
       "palworld.control.target": "palworld.service",
       "save.path": "/srv/palworld/Pal/Saved"
     }
   }
   ```

> [!NOTE]
> Palworld 1.0.0 includes built-in world backups, so routine recovery should use the game backups and PST now defaults `save.backup_interval` to `0`. Mandatory safety backups made before player or Pal save edits remain enabled.
>
> `palworld.config_path` must be a local `PalWorldSettings.ini` visible to PST. Web writes use digest checking, a previous-file backup, and atomic replacement. `palworld.control` never runs arbitrary shell text; it supports only the restricted `process`, `docker`, `systemd`, and `windows_service` drivers.
>
> The backup page reads `backup/world/<timestamp>` from the active world directly. A restore re-hashes every file, stops the server, creates a PST restore point containing the current world, then swaps files on the same filesystem with rollback on failure. This requires a local `save.path`. A previously running server can be started again when a restricted control driver is configured.
>
> WorldOption generation/synchronization accepts only configuration already saved to the server INI and uses pinned Palworld 1.0.0 type metadata. It also requires a stopped server, a full restore point, lossless rebuilt-file validation, and atomic installation. Server-only fields unsupported by WorldOption remain in the INI and are reported explicitly.

##### Run

```bash
./pst
```

```log
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:75 | Starting PalWorld Server Tool...
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:76 | Version: Develop
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:77 | Listening on http://127.0.0.1:8080 or http://192.168.1.66:8080
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:78 | Swagger on http://127.0.0.1:8080/swagger/index.html
```

For background operation (running after SSH window is closed):

```bash
# Run in the background and save the log in server.log
nohup ./pst > server.log 2>&1 &
# To view logs
tail -f server.log
```

##### Stopping Background Process

```bash
kill $(ps aux | grep 'pst' | awk '{print $2}') | head -n 1
```

##### Access

Access via browser at http://127.0.0.1:8080 or http://{Local Network IP}:8080

Access at http://{Server IP}:8080 after opening firewall and security group in cloud servers.

> [!WARNING]
> If you open the file for the first time, nothing will be displayed. Please **wait until the first sav archive synchronization is complete**
>
> If your server configuration is sufficient and performance is good, you can try to make `save.sync_interval` shorter.

#### Windows

##### Download and Extract

Extract `pst_v1.9.1_windows_x86_64.zip` to any directory (recommend naming the folder `pst`).

The Windows `pst.exe` uses the same Web-service architecture as Linux. Keep its console window open and visit the address printed in the log with a browser.

On first launch, PST scans the Steam registry, every `libraryfolders.vdf`, common SteamCMD/SteamLibrary locations, and paths that can be inferred from an existing configuration. A clear best match is configured automatically; ties are shown for selection, and an installation directory is requested only when no PalServer installation can be found.

##### Configuration

Paths normally require no manual editing. Discovery stores them directly in `pst.db`. Enter the PalServer directory only when no candidate is found; use the authenticated `GET/PUT /api/setup/config` endpoint for advanced settings such as custom arguments, TLS, or Fleet nodes. Values written while PST is running are loaded after a PST restart.

`save.decode_path` may be left empty; PST then uses the bundled `sav_cli.exe` next to the main executable.

Advanced settings are written to `pst.db` through the authenticated `PUT /api/setup/config` endpoint:

```json
{
  "values": {
    "palworld.control.mode": "process",
    "palworld.control.target": "C:\\PalServer\\PalServer.exe",
    "palworld.control.working_directory": "C:\\PalServer"
  }
}
```

##### Running

Two ways to run on Windows:

1. start.bat (Recommended)

   Find and double-click the `start.bat` file in the extracted directory.

2. Press `Win + R`, enter `powershell` to open Powershell, navigate to the directory of the downloaded executable file using the `cd` command.

   ```powershell
   .\pst.exe
   ```

```log
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:75 | Starting PalWorld Server Tool...
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:76 | Version: Develop
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:77 | Listening on http://127.0.0.1:8080 or http://192.168.31.214:8080
2024/01/31 - 22:39:20 | INFO | palworld-server-tool/main.go:78 | Swagger on http://127.0.0.1:8080/swagger/index.html
```

If you see the preceding interface, it indicates that the operation is successful. Keep the window open.

##### Access

Access via browser at http://127.0.0.1:8080 or http://{Local Network IP}:8080

Access at http://{Server IP}:8080 after opening firewall and security group in cloud servers.

> [!WARNING]
> If you open the file for the first time, nothing will be displayed. Please **wait until the first sav archive synchronization is complete**
>
> If your server configuration is sufficient and performance is good, you can try to make `save.sync_interval` shorter.

### Docker Deployment

#### Monolithic Deployment

Only one container is needed. Map the game's save directory to the container's internal directory, running on the same physical host as the game server.

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

Most importantly, use `-v` to map the game's save file (Level.sav) directory to the container's `/game` directory.

##### Persistence

If you need to persist the `pst.db` file:

```bash
# Create the file first to prevent it from being recognized as a directory
touch pst.db
```

Then add `-v ./pst.db:/app/pst.db` in `docker run -v`.

##### Environment Variables

Environment variables can override runtime values loaded from the database. The table below lists them:

> [!WARNING]
> Pay attention to the distinction between single and multiple underscores. It's best to copy the variable names from the table below for modifications!

|         Variable Name         |      Default Value      |  Type  |                                       Description                                       |
| :---------------------------: | :---------------------: | :----: | :-------------------------------------------------------------------------------------: |
|        WEB\_\_PASSWORD        |           ""            |  Text  |                          Password for Web interface admin mode                          |
|          WEB\_\_PORT          |          8080           | Number |    **Changing the container mapping port is recommended instead of modifying this**     |
|                               |                         |        |                                                                                         |
|                               |                         |        |                                                                                         |
|     TASK\_\_SYNC_INTERVAL     |           60            | Number |                             Synchronize player online data                              |
|    TASK\_\_PLAYER_LOGGING     |          false          |  Bool  |                      Players log in/log out of broadcast messages                       |
| TASK\_\_PLAYER_LOGIN_MESSAGE  |           ""            |  Text  |                       Players log in to broadcast message content                       |
| TASK\_\_PLAYER_LOGOUT_MESSAGE |           ""            |  Text  |                    The player logs out the broadcast message content                    |
|                               |                         |        |                                                                                         |
|        RCON\_\_ADDRESS        |   "127.0.0.1:25575"   |  Text  |                              Official RCON service address                              |
|       RCON\_\_PASSWORD       |           ""            |  Text  |                        AdminPassword in the server configuration                        |
|      RCON\_\_USE_BASE64      |          false          |  Bool  |                    Compatibility mode for Base64-aware RCON proxies                     |
|        RCON\_\_TIMEOUT        |            5            | Number |                                  Per-command timeout                                    |
|                               |                         |        |                                                                                         |
|        REST\_\_ADDRESS        | "http://127.0.0.1:8212" |  Text  |  The address corresponding to the service REST API can be used in a container network   |
|       REST\_\_PASSWORD        |           ""            |  Text  |                        AdminPassword in the server configuration                        |
|        REST\_\_TIMEOUT        |            5            | Number |                                     Request Timeout                                     |
|                               |                         |        |                                                                                         |
|         SAVE\_\_PATH          |           ""            |  Text  |           Game save path **be sure to fill in the path inside the container**           |
|      SAVE\_\_DECODE_PATH      |     "/app/sav_cli"      |  Text  | ⚠️ Built into the container, do not modify, or it will cause save analysis tool errors  |
|     SAVE\_\_SYNC_INTERVAL     |           600           | Number |                          Interval for syncing player save data                          |
|    SAVE\_\_BACKUP_INTERVAL    |            0            | Number | Game backups are primary; use a value above 0 for an extra PST schedule                |
|   SAVE\_\_BACKUP_KEEP_DAYS    |            7            | Number |                        Interval for auto backup player save data                        |
| MANAGE\_\_KICK_NON_WHITELIST  |          false          |  Bool  | Automatically kicked out when it detects that a player is not whitelisted but is online |

#### Agent Deployment

Two containers are required: `palworld-server-tool` and `palworld-server-tool-agent`.

Applicable for:

- Separate deployment on other servers.
- Deployment only on a local personal computer.
- If the game server's performance is weak and does not meet the requirements, use one of the above two schemes.

##### First, run the agent container

```bash
docker run -d --name pst-agent \
-p 8081:8081 \
-v /path/to/your/Pal/Saved:/game \
-e SAVED_DIR="/game" \
jokerwho/palworld-server-tool-agent:latest
```

You need to `-v` to the directory where the game save file (Level.sav) is located, mapping it to the `/game` directory in the container.

| Variable Name | Default Value | Type |                              Description                               |
| :-----------: | :-----------: | :--: | :--------------------------------------------------------------------: |
|   SAVED_DIR   |      ""       | Text | Game `Saved` path **be sure to fill in the path inside the container** |

##### Then, run the pst container

```bash
docker run -d --name pst \
-p 8080:8080 \
-v ./backups:/app/backups \
-e WEB__PASSWORD="your password" \
-e REST__ADDRESS="http://{GameServerIP}:{RestAPIPort}" \
-e REST__PASSWORD="your admin password" \
-e SAVE__PATH="http://{GameServerIP}:{AgentPort}/sync" \
-e SAVE__SYNC_INTERVAL=120 \
jokerwho/palworld-server-tool:latest
```

##### Persistence

If you need to persist the `pst.db` file:

```bash
# Create the file first to prevent it from being recognized as a directory
touch pst.db
```

Then add `-v ./pst.db:/app/pst.db` in `docker run -v`.

##### Environment Variables

> [!WARNING]
> Pay attention to the distinction between single and multiple underscores. It's best to copy the variable names from the table below for modifications!

|         Variable Name         |      Default Value      |  Type  |                                       Description                                       |
| :---------------------------: | :---------------------: | :----: | :-------------------------------------------------------------------------------------: |
|        WEB\_\_PASSWORD        |           ""            |  Text  |                          Password for Web interface admin mode                          |
|          WEB\_\_PORT          |          8080           | Number |    **Changing the container mapping port is recommended instead of modifying this**     |
|                               |                         |        |                                                                                         |
|                               |                         |        |                                                                                         |
|     TASK\_\_SYNC_INTERVAL     |           60            | Number |                             Synchronize player online data                              |
|    TASK\_\_PLAYER_LOGGING     |          false          |  Bool  |                      Players log in/log out of broadcast messages                       |
| TASK\_\_PLAYER_LOGIN_MESSAGE  |           ""            |  Text  |                       Players log in to broadcast message content                       |
| TASK\_\_PLAYER_LOGOUT_MESSAGE |           ""            |  Text  |                    The player logs out the broadcast message content                    |
|                               |                         |        |                                                                                         |
|        RCON\_\_ADDRESS        |   "127.0.0.1:25575"   |  Text  |                              Official RCON service address                              |
|       RCON\_\_PASSWORD       |           ""            |  Text  |                        AdminPassword in the server configuration                        |
|      RCON\_\_USE_BASE64      |          false          |  Bool  |                    Compatibility mode for Base64-aware RCON proxies                     |
|        RCON\_\_TIMEOUT        |            5            | Number |                                  Per-command timeout                                    |
|                               |                         |        |                                                                                         |
|        REST\_\_ADDRESS        | "http://127.0.0.1:8212" |  Text  |  The address corresponding to the service REST API can be used in a container network   |
|       REST\_\_PASSWORD        |           ""            |  Text  |                        AdminPassword in the server configuration                        |
|        REST\_\_TIMEOUT        |            5            | Number |                                     Request Timeout                                     |
|                               |                         |        |                                                                                         |
|         SAVE\_\_PATH          |           ""            |  Text  |   pst-agent service address, format as<br> http://{Game server IP}:{Agent port}/sync    |
|      SAVE\_\_DECODE_PATH      |     "/app/sav_cli"      |  Text  | ⚠️ Built into the container, do not modify, or it will cause save analysis tool errors  |
|     SAVE\_\_SYNC_INTERVAL     |           600           | Number |                          Interval for syncing player save data                          |
|    SAVE\_\_BACKUP_INTERVAL    |            0            | Number | Game backups are primary; use a value above 0 for an extra PST schedule                |
|   SAVE\_\_BACKUP_KEEP_DAYS    |            7            | Number |                        Interval for auto backup player save data                        |
| MANAGE\_\_KICK_NON_WHITELIST  |          false          |  Bool  | Automatically kicked out when it detects that a player is not whitelisted but is online |

#### Synchronizing Archives from k8s-pod

Starting from v0.5.3, it is supported to synchronize game server archives within a cluster without the need for an agent.

> After v0.5.8, due to the addition of player backpack data viewing, the directory of the entire Sav file is copied, and you must ensure that the Palu server container has a tar tool in order to compress and decompress.

> Make sure that the serviceaccount used by pst has "pods/exec" permissions!

You only need to change the `SAVE__PATH` environment variable, in the following format:

```bash
SAVE__PATH="k8s://<namespace>/<podname>/<container>:<Game Archive Directory>"
```

For example:

```bash
SAVE__PATH="k8s://default/palworld-server-0/palworld-server:/palworld/Pal/Saved"
```

> Since the time and location (including HASH) of the Level.sav file created by the game server are uncertain at the first instance, you only need to point to the Saved directory level, and the program will automatically scan.

When pst and the game server are in the same namespace, you can omit it:

```bash
SAVE__PATH="k8s://palworld-server-0/palworld-server:/palworld/Pal/Saved"
```

### Synchronizing Archives from Docker Container

Starting from v0.5.3, it is supported to synchronize game server archives inside a container without the need for an agent.

#### File Deployment Usage

For binary deployments, use automatic discovery first. If it fails, enter the PalServer directory in the Web setup flow or update `save.path` through `PUT /api/setup/config`:

```yaml
save:
  path: "docker://<container_name_or_id>:<game_save_directory>"
```

For example:

```yaml
save:
  path: docker://palworld-server:/palworld/Pal/Saved
# or
save:
  path: docker://04b0a9af4288:/palworld/Pal/Saved
```

#### Docker Deployment Usage

If the pst application is deployed as a Docker single container, then you need to modify the `SAVE__PATH` environment variable and mount the Docker daemon inside the pst container.

1. Mount the daemon

In the original `docker run` command, add the line `-v /var/run/docker.sock:/var/run/docker.sock`.

2. Modify the environment variable

Change the `SAVE__PATH` environment variable as follows:

```bash
SAVE__PATH="docker://<container_name_or_id>:<game_save_directory>"
```

For example:

```bash
SAVE__PATH="docker://palworld-server:/palworld/Pal/Saved"
#or
SAVE__PATH="docker://04b0a9af4288:/palworld/Pal/Saved"
```

> [!WARNING]
> If you see an error like `Error response from daemon: client version 1.44 is too new. Maximum supported API version is 1.43` after running, it is because the Docker API version used by your current docker engine is too low. In this case, please add another environment variable:
>
> -e DOCKER_API_VERSION="1.43" (your API version)

> Since the time and location (including HASH) of the Level.sav file created by the game server are uncertain at the first instance, you only need to point to the Saved directory level, and the program will automatically scan.

## Projects Stats

![Stats](https://repobeats.axiom.co/api/embed/8724e69c284e0645f764a4a1cd525477be13cbe8.svg "Repobeats analytics image")

## REST API Document

[APIFox Online document](https://q4ly3bfcop.apifox.cn/)

## Acknowledgements

- [PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) for the current `palsav-flex` parser, Oodle compression, and save rebuilding support
- [palworld-server-toolkit](https://github.com/magicbear/palworld-server-toolkit) for providing high performance save file parsing
- [pal-conf](https://github.com/Bluefissure/pal-conf) for the current server setting catalog and translation reference
- [PalEdit](https://github.com/EternalWraith/PalEdit) for providing the initial conceptualization and logic for data processing

## LICENSE

The main application is licensed under [Apache 2.0](LICENSE). The separate `sav_cli` process contains GPL-3.0-or-later components; `sav_cli-GPL-3.0.txt` is included in release packages.

```

```
