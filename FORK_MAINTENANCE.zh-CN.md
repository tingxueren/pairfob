# Pairfob fork 维护说明

本说明用于 [tingxueren/pairfob](https://github.com/tingxueren/pairfob) 的
`fork-maintenance` 分支。上游为
[arronKler/pairfob](https://github.com/arronKler/pairfob)。

## Git 提交身份：仅本仓库

自己的提交统一使用 `tingxueren <zhangdongsheng1224@gmail.com>`。
在每个 Pairfob 克隆目录内执行：

```bash
git config --local user.name tingxueren
git config --local user.email zhangdongsheng1224@gmail.com
```

配置保存在该克隆的 `.git/config`，只作用于这个仓库（包括它的关联
worktree）。不要使用 `--global`，不要修改 `~/.gitconfig`，也不要通过
全局环境变量覆盖提交身份；其他仓库继续使用各自已有配置。

仓库根目录的 `.gitconfig` 不会被 Git 自动读取，本维护方式不依赖它。
`.git/config` 不纳入版本管理，因此新克隆需要重新执行上述命令。

核验配置来源及实际生效的作者、提交者：

```bash
git config --local --show-origin --get-regexp '^user\.'
git var GIT_AUTHOR_IDENT
git var GIT_COMMITTER_IDENT
```

第一条命令应显示 `file:.git/config` 以及上述个人用户名、邮箱；后两条
也应显示这套身份。若不一致，先检查仓库配置和 `GIT_AUTHOR_*`、
`GIT_COMMITTER_*` 环境变量的覆盖，再提交。

配置只影响后续提交。已有提交的身份改写需要用户明确授权，并保留
上游作者和提交者。普通 push 和改写后的历史推送均须先得到用户确认。

## 维护入口

- 开发、编译和测试在 dorm-dev 的 `~/Work/Git/pairfob` 进行；本机源码以同步为主。
- Core 适配及验证记录：[docs/core-parity.zh-CN.md](docs/core-parity.zh-CN.md)。
- 自有网页部署：[.cloudflare/pairfob-fork/README.zh-CN.md](.cloudflare/pairfob-fork/README.zh-CN.md)。
- 替换本机、mini 的 Pairfob 二进制前，先告知用户，由用户处理云壳审核。

本文件与自有部署配置属于 fork 维护内容，不放入上游功能 PR。

## 每次改动后的四节点网页验收（必跑）

以后本 fork 的改动，完成相应模块测试后、发布或推广前，都必须跑真实四节点
验收；不能用手工填好标题的 fixture 或单个函数测试替代。失败时先定位，
不得把缺节点、采集失败或覆盖不足当成通过。开发、Go 回放和浏览器在 dorm-dev；
本机仅执行只读采集及 rsync。两端源码先同步，包括未提交修改。

从本机的 Pairfob 仓库运行：

```bash
bash scripts/verify-core-ui.sh --runner dorm-dev
```

脚本通过既有 SSH 别名读取 `m3=local,dorm-dev=dorm-dev,nerd=nerd,mini=mini`。
不重配 SSH、不安装二进制、不读配对密钥、不发送终端输入，也不重启 Core。
节点需要 Node、`~/.local/bin/hc` 和自维护 Pairfob；dorm-dev 另外需要仓库依赖、
Go 1.26.11、Bun，以及 Playwright 1.61.0 和对应 Chromium。现有浏览器工具在
`.dev/browser-tools/node_modules/playwright`；可用 `PAIRFOB_PLAYWRIGHT_MODULE`
指定已安装模块。脚本不自动安装依赖。测试服务器仅监听 loopback，独占默认
5197 端口；冲突时失败，用 `PAIRFOB_CORE_UI_PORT` 改端口，不清理别人进程。

验收覆盖：

- 四个不同节点的真实 `terminal.list` 与各机器 `herdr-app` 文档。
  校验节点集合、唯一主机和二进制 fork 标识；版本不一致直接失败。
- 生产 `Core.Observe` 的冷启动与后台独立文档加载，然后进入生产 PWA
  store 和 React 页面；预期身份、分组和自定义名称独立从原始捕获计算。
- 全部 live terminal 的一一对应、项目/标签页归属、手工会话名、远端标记，
  并拒绝把远端 cwd 投影成本机文件根。没有自定义标题或远端样本也不通过。
- 同一网页接收后续改名快照时立即更新，恢复原标题也无需重载。
- 项目名称固定排序、最近打开排序、远端隐藏/恢复及设置持久化。
- 390×844 手机触摸、820×768 和 1024×768 平板触摸、1440×900 桌面鼠标，
  四节点共 16 组 DOM 校验、溢出检查和截图。
- 内存中的反向校验：故意丢 pane、改旧标题、错项目名、把远端路径当本机、
  合并同名不同目录，确认验收会报错；不修改用户会话。

输出保存在忽略目录 `.dev/core-ui/<run>/`，包含 `captures/`、`projected/`、
`screenshots/` 和 `report.json`，自动回传本机。真实路径、标题和截图属于
私人数据，不提交 Git，也不上传公共 CI。报告记录源码 commit、工作区内容
摘要、Playwright 版本和各节点二进制标识；源码不一致或运行中变化不能通过。
采集 30 分钟后失效；需重新采集，不能用旧证据验收新状态。

需要仅在 dorm-dev 回放刚采集的数据时：

```bash
bash scripts/verify-core-ui.sh --input .dev/core-ui/<run>
```

推广后还需核对预期二进制，例如：

```bash
PAIRFOB_EXPECT_BINARY_COMMIT=<目标commit至少7位> \
  bash scripts/verify-core-ui.sh --runner dorm-dev
```

该检查在 runner 端也生效。脚本只验证真实 Core 元数据经候选适配器到网页的
链路；**不覆盖正在运行 daemon 的文档缓存、加密 relay/P2P 传输、手机 PWA
缓存和实体设备行为**。后者需在线版本核对与手机抽查，不能仅凭这套验收
判定“手机旧标题已修复”。纯验收工具/文档变化不要求重装四机二进制。
