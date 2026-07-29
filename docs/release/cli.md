# CLI 发版（solo-cli）

> CLI 是用于配对与管理 agent 的命令行工具，以单一二进制分发。

## 构建

```bash
# Darwin ARM64（本地）
make solo-cli            # → output/darwin/solo-cli

# Linux AMD64
make solo-cli-linux-amd64  # → output/linux/solo-cli
```

版本由 `Makefile` 经 `-ldflags -X .../cli/internal/config.Version=$(VERSION)` 注入（`$(VERSION)` 取自 `git describe`）。源码回退值在 `cli/internal/config/version.go` 的 `var Version`（默认 `dev`），由 cobra root 命令的 `Version` 字段引用。**发版前先打 tag**（见 [versioning.md](versioning.md)）。

## 分发

CLI 无服务进程，直接拷贝二进制即可：

```bash
cp output/darwin/solo-cli /usr/local/bin/solo   # 或目标主机
```

## 验证

```bash
solo --version
```
