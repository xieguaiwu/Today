# Today · 每日自查清单 TUI

终端交互式每日自查清单（bubbletea TUI）。用方向键与空格勾选当日完成的项目，`q` 退出。适合作为每天固定时段的自我检查工具。

## 特性

- 纯终端 TUI，零配置
- 内置 7 个自查项：Eyes / Nose / Skin / Lips / Anxiety / Cognition / Weight & Fat
- 上下键（或 `k`/`j`）移动光标，空格勾选/取消，`q`/`Ctrl-C` 退出

## 安装

### 方式一：源码构建

```bash
go build -o Today .
./Today
```

### 方式二：GitHub Release

从 [Releases](https://github.com/xieguaiwu/Today/releases) 下载 `Today-<version>-linux-amd64.tar.gz`：

```bash
tar xzf Today-*-linux-amd64.tar.gz
sudo install -Dm755 Today /usr/local/bin/Today
```

### 方式三：COPR（Fedora）

```bash
sudo dnf copr enable xieguaiwu/Today
sudo dnf install Today
```

## 使用

```bash
Today
```

## 开发

- 语言：Go 1.25+，依赖 [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea)
- 测试：`go test ./...`

## 许可证

MIT
