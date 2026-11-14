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
