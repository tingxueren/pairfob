# 自维护 Pairfob 与 herdr-core 的适配

开发及验证在 dorm-dev，源码维护于 `~/Work/Git/pairfob` 的
`fork-maintenance`。本机和 mini 仅同步源码；替换 Mac 二进制前需人工批准。
自有 relay 为 `https://pairfob.tingxueren.com`，fork push 后由 Cloudflare Build 部署。

## 已提供的功能

- 项目、标签页、分屏读取 core 的 `herdr-app` 共享文档；终端身份仍来自
  core address，按机器限定。隐藏节点和 stack 选中节点只影响几何，终端仍可访问。
- 没有 App 项目标签的终端，若工作目录位于 `/Users/<用户>/` 或
  `/home/<用户>/` 下，则按真实目录独立分组；主目录和临时目录仍在机器默认组。
  目录中的空格保留，远端目录只作显示，不成为本机文件访问根。
- 手工 pane 名优先，其次显示有 agent 的自定义 App 标签页名，再回退到
  原生会话任务标题；默认 `main`、纯数字标签和普通 shell 不遮盖任务或命令名。
- 新建标签页、左右/上下分屏、项目和标签页重命名写回同一文档。
  保留未知字段和整数，使用文档版本 CAS；创建终端后写文档冲突会明确报告
  未知结果，必须刷新核对，不能自动重发创建请求。
- 可选文档每两秒在后台独立连接刷新。慢远端、查询失败不会阻塞输入。
- Codex/Claude 使用 SessionStart/SessionEnd 启动钩子绑定原生会话 ID。
  记录按 core ID、terminal、foreground PID 和进程出生时间隔离。
  启动时最多两秒只读等待 foreground 刷新，拒绝嵌套 agent 和错误继承 ID。
- daemon journal 用可信 ID 提供对话、任务标题和步骤。runtime 的 HistoryQuery 仍不支持，daemon 提供对话 History RPC；没有可靠 ID 的旧运行会话保持未绑定，下次新建或 resume 后生效。
- 远端文本预览、目录、搜索、路径解析及 Git status/diff/branches 通过
  `machine.exec` 调目标机 fork helper。目标机读取前后核验 terminal PID、
  启动时间、cwd；使用现有 Inspector 的路径、符号链接和大小限制。
  缺 helper、目标离线或 cwd 改变时明确失败，绝不回退读取本机同名路径。

## 配置

在目标机器使用自维护二进制后执行：

```sh
~/.local/bin/pairfob agent-observation codex
~/.local/bin/pairfob agent-observation claude
```

只追加专用钩子，保留已有命令和设置，不修改模型、权限或 daemon 配置。
Codex 需要启用其 hooks 功能；dorm-dev 的 Codex 0.169.0 已启用。
core 进程无需重启；已有 Codex 也无需为安装钩子强制退出。
每个目标机自己的 Pairfob 可读取本机绑定；聚合 core 中远端 PID 不在宿主机验证。

远端文件目标需 `~/.local/bin/pairfob` 为此 fork 候选，并已运行 core。
helper 是内部只读命令，不需启动额外 Pairfob daemon或将 SSH 凭证交给网页。

## 边界

App 私有选中/zoom 和 UI 模块不写回；手机选择不抢桌面焦点。
像素固定大小不能直接映射到终端 cell，投影使用可用的权重或默认均分。
远端文件上传、删除、改名、媒体及大文件下载仍拒绝，不伪装成本机操作；
远端会话对话请连接该节点自己的 Pairfob。未增加 worktree、命名 core session
或详细 agent inspection，缺失能力继续由 capabilities 隐藏。
关闭整个项目由可选 `close_workspace` capability 单独控制：Herdr 0.9.0
及以上支持，Core 和未声明能力的旧 daemon 隐藏入口，单个终端的关闭独立保留。
Mac 二进制已可交叉编译，但需要本机批准与实际验收后才能推广。

## 验证

2026-11-14：Astra 多轮复核修正身份、CAS、字段保留、异步连接和过滤边界。
完整 verify 四 gate 通过，随后 journal 过滤修正单独重跑相关 race 检查。
隔离 real PTY 覆盖 split/rename、真实 SessionStart 进程祖先绑定、helper
失效终端拒绝及独立目标 core 的 machine.exec 全链路。
真实历史扫描覆盖 818 Claude、26 Codex 记录；注入上下文不进入用户对话。
开发证据保存在 `.dev/core-parity/`，不提交历史记录及部署凭证。

后续每次 fork 改动必须执行四节点真实数据网页验收，入口为
`bash scripts/verify-core-ui.sh --runner dorm-dev`。流程、环境、覆盖边界及
私有截图/报告位置见 [维护说明](../FORK_MAINTENANCE.zh-CN.md#每次改动后的四节点网页验收必跑)。
该验收不替代实体手机与 live daemon/relay 链路检查。
