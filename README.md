# Today · 每日自查清单 TUI

终端交互式每日自查清单（bubbletea TUI）。用方向键与空格勾选当日完成的项目，`q` 退出。
勾选**即时落盘**并按日期归档，所以第二天打开是全新的一天，同时能看到自己连续打卡了多少天。

## 特性

- 纯终端 TUI，开箱即用（首次运行自动生成配置文件）
- 清单可自定义：项目、标题、每项提示语都写在 JSON 里，改完即生效
- 按日期持久化：跨天自动重置，`Ctrl-C` 或关掉终端都不会丢当日的勾选
- 连续打卡天数 + 累计打卡天数
- 非交互模式 `--status`，可接 shell 提示符 / waybar 状态栏
- 零第三方依赖之外的东西：只依赖 [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea)

## 安装

### 方式一：源码构建

```bash
git clone https://github.com/xieguaiwu/Today.git
cd Today
go build -o Today .
sudo install -Dm755 Today /usr/local/bin/Today   # 或 install -Dm755 Today ~/.local/bin/Today
```

仓库自带 `vendor/`，离线可构建：

```bash
GOFLAGS=-mod=vendor CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o Today .
```

### 方式二：GitHub Release

从 [Releases](https://github.com/xieguaiwu/Today/releases) 下载 `Today-<version>-linux-amd64.tar.gz`：

```bash
tar xzf Today-*-linux-amd64.tar.gz
install -Dm755 Today-*/Today ~/.local/bin/Today
```

### 方式三：COPR（Fedora）

```bash
sudo dnf copr enable xieguaiwu/Today
sudo dnf install Today
```

> ⚠️ 当前 COPR 只构建了 **Fedora 43 / 44 / 45** 三个 chroot。若你在 Fedora 42 或更早版本，
> `dnf copr enable` 之后仓库里没有匹配的 release，`dnf install Today` 会找不到包——请改用上面两种方式。

## 使用

```bash
Today            # 打开当日清单
Today --status   # 打印今日进度后退出，例：2026-08-31 3/7 streak=5 total=42
Today --reset    # 清空今日勾选
Today --version
```

按键：

| 键 | 作用 |
|---|---|
| `↑` `↓` / `k` `j` | 移动光标 |
| `空格` / `enter` / `x` | 勾选 / 取消（立即写盘） |
| `r` | 清空当日全部勾选 |
| `q` / `Ctrl-C` | 退出 |

## 配置

首次运行会在下面位置生成默认配置，直接编辑即可：

```
~/.config/Today/config.json
```

默认内容就是 v0.1.0 里写死的那 7 项：

```json
{
  "title": "每日自查清单",
  "items": [
    { "id": "eyes", "label": "Eyes" },
    { "id": "nose", "label": "Nose" },
    { "id": "skin", "label": "Skin" },
    { "id": "lips", "label": "Lips" },
    { "id": "anxiety", "label": "Anxiety" },
    { "id": "cognition", "label": "Cognition" },
    { "id": "weight-fat", "label": "Weight & Fat" }
  ],
  "history_days": 365
}
```

### 两种写法

项目可以写成裸字符串（省事），也可以写成对象（可控）：

```json
{
  "title": "晨间自查",
  "items": [
    "喝水",
    { "label": "Floss", "hint": "牙线 30 秒" },
    { "id": "meditate", "label": "Meditate", "hint": "10 分钟" }
  ]
}
```

| 字段 | 说明 |
|---|---|
| `title` | 顶部标题，留空则用「每日自查清单」 |
| `items[].id` | 历史记录用的稳定键。省略时由 `label` 生成（`Weight & Fat` → `weight-fat`）；纯中文标签会退化成 `item1` 这类位置编号 |
| `items[].label` | 显示文字，必填（空白项会被忽略） |
| `items[].hint` | 可选。光标停在该项时以暗色显示在下一行 |
| `history_days` | 历史保留天数，默认 365 |

**关于 `id`**：历史是按 `id` 记的。省略 `id` 时它由 `label` 推导，所以**改了 label 等于换了一项**，那一项的历史会重新开始。想改名又不丢历史，就显式写死 `id`。

### 路径与环境变量

| 用途 | 优先级 |
|---|---|
| 配置 | `--config` > `$TODAY_CONFIG` > `$XDG_CONFIG_HOME/Today/config.json` > `~/.config/Today/config.json` |
| 历史 | `--data` > `$TODAY_DATA` > `$XDG_DATA_HOME/Today/history.json` > `~/.local/share/Today/history.json` |

路径里的 `~` 会展开。

配置写坏了会**直接报错并退出**，不会悄悄退回默认清单——否则打错一个逗号看起来就像「我的项目全没了」。
想恢复默认：删掉配置文件再跑一次。

## 数据与隐私

历史记录在：

```
~/.local/share/Today/history.json      # 权限 0600
```

格式是按日期分键的简单 JSON：

```json
{
  "version": 1,
  "days": {
    "2026-08-31": { "checked": ["eyes", "nose"], "total": 7, "updated_at": "..." }
  }
}
```

- **本地日期**分天（不是 UTC），过零点自动换新的一天；终端一直开着也会在 30 秒内察觉日期变化
- 写入是**原子**的（同目录临时文件 + rename），掉电/强杀不会留下半截文件
- 文件损坏时会被挪到 `history.json.corrupt-<时间戳>` 并重新开工，不会每次运行都报错
- 全部数据只存本机，无任何网络请求

## 故障排查

**打开 Today 卡住约 5 秒才出现界面**

这不是本程序的问题：bubbletea 在包初始化时会向终端发一条 OSC 11 查询（`\x1b]11;?`）来探测背景色，
超时是 5 秒（`termenv.OSCTimeout`）。绝大多数终端（wezterm / kitty / foot / alacritty / gnome-terminal / xterm）
会立刻回答，所以感觉不到。但**不回答这条查询的终端**——Linux 虚拟控制台（`TERM=linux`）、部分
`screen`/`tmux` 配置——就会吃满这 5 秒。

实测（本机，伪终端）：

| 环境 | 首帧耗时 |
|---|---|
| `TERM=dumb` | 0.03 s |
| `TERM=xterm` / `xterm-256color` / `vt100`，终端不回答查询 | 5.03 s |

注意 `NO_COLOR` **不能**跳过这条查询，只有 `TERM=dumb` 能。所以绕法是：

```bash
TERM=dumb Today
```

代价是失去暗色提示文字（本来 `NO_COLOR` 也会去掉），功能完全不受影响。

**配置改坏了、程序直接退出**

这是有意的：配置解析失败时报错退出，不静默回落到默认清单。按错误信息里的路径修好，
或删掉该文件重新生成默认。

**想确认历史记了什么**

```bash
Today --status
cat ~/.local/share/Today/history.json
```

## 开发

```bash
go test ./...          # 43 个单测，覆盖配置解析 / 持久化 / TUI 状态机
go vet ./...
gofmt -l .             # 应为空
```

打包冒烟测试（伪终端，非交互环境下必须用 `TERM=dumb`，理由见上节）：

```bash
TERM=dumb Today
```

- 语言：Go 1.24+（`go.mod` 已放宽以兼容 COPR 的 golang 版本）
- 依赖：`charmbracelet/bubbletea`，`vendor/` 已入库
- 打包：`packaging/fedora/Today.spec`

## 许可证

MIT
