# ctrssh（中文）

[English](README.md)

把跑在 SSH 可达主机上的远端容器，暴露成一个普通的 SSH 端点。这样你可以直接 `ssh work.dev`（或在 VSCode Remote-SSH / JetBrains Gateway / Cursor 中打开），而无需从容器对外暴露任何 TCP 端口。

## 工作原理

```
本地 ssh 客户端 ──▶ ProxyCommand ──▶ ctrssh connect --stdio
                                          │
                                          ▼
                              ssh user@host docker exec -i ctr sshd -i
                                          │
                                          ▼
                            容器内的 openssh-server（inetd 模式）
                                          │
                                          ▼
                                       你的 shell
```

不跑守护进程，不暴露端口，整条链路是纯 stdio。

## 安装

```bash
go install github.com/SXGC/ctrssh/cmd/ctrssh@latest
```

## 使用

```bash
# 每个工作区一次性配置
ctrssh add work --host me@server --container devbox --user vscode
ctrssh prepare work     # 在容器内安装 sshd 并注入公钥
ctrssh config-ssh       # 写入 ~/.ssh/config（add 命令会自动调用）

# 日常使用
ssh work.dev
code --remote ssh-remote+work.dev /workspaces
# JetBrains Gateway：在主机列表里选 work.dev
```

## 命令

| 命令 | 用途 |
|---|---|
| `add <name>` | 注册一个工作区，并自动刷新 ssh 配置 |
| `list` | 列出已注册的工作区 |
| `rm <name>` | 删除工作区 |
| `prepare <name>` | 在远端做一次性准备（幂等） |
| `config-ssh` | 刷新 `~/.ssh/config` 中由本工具维护的条目 |
| `connect --stdio <name>` | ProxyCommand 调用入口（无需手动执行） |
| `doctor <name>` | 分段诊断连接链路 |

## 环境要求

- **本地**：PATH 中要有 OpenSSH 客户端（POSIX 系统和 Windows 10+ 自带）
- **远端主机**：装有 docker CLI，当前用户有 docker 权限，SSH 可达
- **容器**：任意 Linux 发行版；`prepare` 会在缺少时自动安装 `openssh-server`

## 文件

```
~/.config/ctrssh/
  workspaces.yaml          # 工作区注册表
  id_ctrssh, id_ctrssh.pub # 本工具专用密钥对
~/.ssh/config              # ctrssh 通过标记块写入 Host 配置
```
