# 官方基线与定制层升级

`feat-custom-ui` 是部署分支：它包含官方发布版本，以及账号卡片、配额展示和 Antigravity/Gemini 兼容修复等定制改动。升级时合并新的官方版本，继续保留这层改动，再构建本仓库镜像。

`.github/custom-upstream-version` 记录实际合入的官方稳定版标签。构建流程会从官方仓库获取该标签，确认它是当前提交的祖先，再将版本号、当前提交和构建时间传给 Dockerfile。仅修改版本文件、没有合入对应官方版本时，构建会失败。

## 更新流程

前端本地验证和 CI 使用 Node 24、pnpm 9，与 Docker 构建保持一致；修复安全问题后的国际化依赖要求 Node 22 或更高版本。

1. 从最新 `feat-custom-ui` 创建独立升级分支，并获取官方标签：

   ```sh
   git fetch origin feat-custom-ui
   git fetch upstream --tags
   git switch -c upgrade/upstream-vX.Y.Z-custom origin/feat-custom-ui
   git merge --no-ff vX.Y.Z
   ```

2. 检查官方修改与定制代码重叠的部分。对共享文件保留双方需要的行为；不要整文件覆盖。例如，Antigravity 的定制流式错误处理与官方非流式错误处理、图片计费更新需要同时保留。
3. 将 `.github/custom-upstream-version` 更新为 `vX.Y.Z`。保留上游维护的 `backend/cmd/server/VERSION`；镜像版本由构建参数明确指定，不依赖该兜底文件。
4. 运行 CI，并核对账号卡片/表格切换、配额展示、Gemini 请求规范化及错误转发。查看 `git diff vX.Y.Z` 可确认最终定制层的范围。
5. 使用合并提交（merge commit）将升级分支合入 `feat-custom-ui`，保留官方标签的祖先关系；不要用 GitHub 的 squash/rebase 合并方式重写这些提交。随后由定制镜像工作流发布 `ghcr.io/meyaomiao/sub2api:latest` 和对应的 `sha-*` 标签。其他分支手动构建只发布 SHA 标签，不更新生产使用的 `latest`。
6. 部署前备份数据库和部署配置，记录当前镜像摘要，并阅读该官方版本的迁移说明。用新镜像的 `--version` 输出核对基线版本和提交，再更新应用容器；部署时记录实际使用的镜像摘要。
7. 验证健康接口、页面版本、定制功能和实际 API 请求。涉及数据库迁移时，回退需同时考虑旧程序与迁移后结构的兼容性，不能只依赖旧镜像仍然存在。

## 版本显示

`latest` 是本仓库的发布标签，不代表官方版本号。此前定制构建没有传入 `VERSION` 和 `COMMIT`，导致已经合入 v0.2.18 的程序仍显示兜底值 0.2.17，提交显示为 `docker`。现在界面显示已验证的官方基线版本，镜像标签和程序 `--version` 输出保留定制构建的具体提交。

## 独立的客户端隐私过滤

DSH 的本机请求过滤服务属于独立部署层，不包含在本镜像中。升级应用镜像时继续保留现有过滤入口，不应把切换服务端版本当成客户端自动环境信息过滤的替代方案。
