# 自有 Pairfob 自动部署

这份目录只用于 tingxueren/pairfob，不能放进上游功能 PR。

线上地址：https://pairfob.tingxueren.com/pair 。Worker 为 pairfob-fork，
复用既有 D1 pairfob-fork-db 和 Durable Objects。Cloudflare 的运行时秘密
OPERATOR_TOKEN、IP_HASH_PEPPER 保持原值，不在 Git、构建变量或日志中存储。

Workers Builds 配置：仓库 tingxueren/pairfob；生产分支为 fork-maintenance，
避免污染上游修复 PR；Root directory 为 /；Bun 版本与根目录 .bun-version 一致。

- Build command：`bash .cloudflare/pairfob-fork/build.sh`
- Deploy command：`bash .cloudflare/pairfob-fork/deploy.sh`
- 关闭其他分支的自动预览部署。
- Build token：Cloudflare 中现有 pairfob-fork-deploy。

build.sh 安装冻结依赖并跑现有完整 Web gate、打包器测试，prepare.ts
从当前线上获取资源清单并恢复旧文件，用已有 retainPwaAssets 保留最多两代历史资源，
生成带源码 commit 和实际时间的 BUILD。最后以 Wrangler dry-run 校验配置。
网络失败、资源清单不合法、检查或构建失败都停止，不发布。

deploy.sh 使用既有 D1 中独立的 __pairfob_fork_deploy_lease 表取得发布租约，
租约覆盖重新读取线上资源、应用迁移和上传；第二个并发发布直接失败，可稍后重试。
异常终止的租约 30 分钟到期，正常退出只释放自己的租约。Workers Builds 最长 20 分钟；
手动发布也使用此入口；子进程合计预算20分钟，超时强制结束，不能绕过租约。

deploy.sh 先应用 D1 迁移，再上传 Worker 和网页；D1 与代码上传不能构成同一事务，
后续数据库变更必须保持对旧代码的兼容性。它不生成秘密，不替换已配对身份。
构建成功不是手机验收通过：网页、加密连接、扫码与触摸滚动仍要实际复测。

手机 PWA 的页面获取策略：优先网络，750ms 超时则显示缓存，后台继续更新；
已打开的页面不会自动更换 CSP。发布后联网关闭/打开，等待数秒再重新打开。
不要先清除网站数据；浏览器配对凭据也存在本地。

手动构建/部署在 dorm-dev 的 ~/Work/Git/pairfob 执行，先通过 build.sh，
再使用获授权的 Cloudflare 环境执行 deploy.sh。不要用上游 wrangler.jsonc
替代本目录配置，它指向不同实例。
