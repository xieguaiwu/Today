# Today · 每日自查清单 TUI

终端交互式每日自查清单（bubbletea TUI）。用方向键与空格勾选当日完成的项目，`q` 退出。
勾选**即时落盘**并按日期归档，所以第二天打开是全新的一天，同时能看到自己连续打卡了多少天。

## 特性

- 纯终端 TUI，开箱即用（首次运行自动生成配置文件）
- 清单可自定义：项目、标题、提示语、**步骤数**都写在 JSON 里，改完即生效
- **多步习惯**：一个习惯可以拆成几步，部分完成用深浅格子区分（参考 streak App 的分级语义）
- 按日期持久化：跨天自动重置，`Ctrl-C` 或关掉终端都不会丢当日的进度
- 统计视图：当前连胜 / 最高连胜 / 完成次数 / 最近 90 天持续率 + 年度打卡热力图 + 星期分布
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
Today --status   # 今日进度：2026-09-01 3/5 streak=24 total=24 partial=2
Today --stats    # 每个习惯的 完成次数/当前连胜/最高连胜/最近90天持续率
Today --reset    # 清空今日进度
Today --version
```

`--stats` 输出示例（适合脚本对账，不依赖终端渲染）：

```
$ Today --stats
item           done   streak   best  rate90
全部               24       24     24     27%
anki             24       24     24     27%
usaco             0        0      0      0%
anaerobic         7        5      5      8%
pma               0        0      0      0%
brain             8        8      8      9%
```

按键：

| 键 | 作用 |
|---|---|
| `↑` `↓` / `k` `j` | 移动光标 |
| `空格` / `enter` / `x` | **完成一步**（单步习惯就是勾选/取消；已满再按回到 0） |
| `f` | 直接拉满 / 清零 |
| `+` `-` | 加一步 / 减一步 |
| `r` | 清空当日全部进度 |
| `Tab` / `Esc` | 列表 ↔ 统计 |
| `←` `→` / `h` `l` | 统计页：切换条目 |
| `↑` `↓` / `j` `k` | 统计页：切换年份 |
| `q` / `Ctrl-C` | 退出 |

## 步骤（多步习惯）

有些习惯不是「做没做」而是「做了几步」。给条目加 `steps` 就能表达：

```json
{ "id": "anki", "label": "anki单词", "steps": 5, "color": "#3B82F6", "group": "学习" }
```

列表里显示成进度条而不是勾选框：

```
> ▹▹▹▸ 3/5 anki单词     ■■■▓░··  24🔥
  ▹▸▸ 1/3 USACO          □○·····   0🔥
  [x] brain               ■■□····   8🔥

2 项部分完成（未计入连胜/完成次数）
```

**口径（重要）**：

| 概念 | 定义 |
|---|---|
| 完整完成 | 步数达到 `steps` —— **只有这个计入连胜、完成次数、热力图亮格** |
| 部分完成 | `0 < 步数 < steps` —— 热力图按比率降档显示，**不计入任何统计** |

这样「打了卡但只做了一半」不会虚增连胜，和源 App 的分级语义一致。

热力图/周格的灰度：`■` 满 · `▓` 约 3/4 · `▒` 约 1/2 · `░` 少量 · `□` 没做 · `·` 未来 · `◉`/`○` 今天。
即使终端不支持颜色，这些字符本身也能区分档位。

`steps` 省略或写 1 就是普通勾选框，**老配置完全不受影响**。上限 12。

## 配置

首次运行会在下面位置生成默认配置，直接编辑即可：

```
~/.config/Today/config.json
```

默认配置就是这 5 个习惯（带颜色/分组/步骤）：

```json
{
  "title": "每日自查清单",
  "items": [
    { "id": "anki",      "label": "anki单词", "hint": "Anki 新词 + 复习", "color": "#3B82F6", "group": "学习", "steps": 5 },
    { "id": "usaco",     "label": "USACO",    "hint": "算法训练/比赛题", "color": "#84CC16", "group": "学习", "steps": 3 },
    { "id": "anaerobic", "label": "无氧锻炼", "hint": "力量训练",       "color": "#F87171", "group": "健身", "steps": 4 },
    { "id": "pma",       "label": "PMA",      "hint": "数学/物理额外练习", "color": "#2563EB", "group": "学习", "steps": 2 },
    { "id": "brain",     "label": "brain",    "hint": "专注力/冥想训练", "color": "#EF4444", "group": "健身" }
  ],
  "history_days": 365
}
```

步骤数只是预设，按自己实际情况改 `steps` 即可；不需要多步的删掉这个字段就是普通勾选框。

> v0.1.0 那 7 项健康自查（Eyes / Nose / Skin / Lips / Anxiety / Cognition / Weight & Fat）
> 已于 v0.3.1 从默认清单移除。想留着就在 `items` 里自己加回去 —— 历史数据不会丢，
> 只要 `id` 写对（例如 `weight-fat`）。

### 两种写法

项目可以写成裸字符串（省事），也可以写成对象（可控）：

```json
{
  "title": "晨间自查",
  "items": [
    "喝水",
    { "label": "Floss", "hint": "牙线 30 秒" },
    { "id": "meditate", "label": "Meditate", "hint": "10 分钟", "steps": 3 }
  ]
}
```

| 字段 | 说明 |
|---|---|
| `title` | 顶部标题，留空则用「每日自查清单」 |
| `items[].id` | 历史记录用的稳定键。省略时由 `label` 生成（`Weight & Fat` → `weight-fat`）；纯中文标签会退化成 `item1` 这类位置编号 |
| `items[].label` | 显示文字，必填（空白项会被忽略） |
| `items[].hint` | 可选。光标停在该项时以暗色显示在下一行 |
| `items[].steps` | 可选。完成该习惯需要几步，默认 1（=普通勾选框），上限 12 |
| `items[].color` | 可选。`#RGB` / `#RRGGBB`，用于周格、热力图、条目名。省略则从内置色板取色 |
| `items[].group` | 可选。分类标签（如「健身」「学习」），目前仅作元数据 |
| `history_days` | 历史保留天数，默认 365 |

> 「全部」的完成次数按**去重天数**算（与它自己的热力图亮格数一致），
> 不是各习惯次数相加。

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
  "version": 2,
  "days": {
    "2026-09-01": { "progress": { "anki": 5, "usaco": 1 }, "total": 12, "updated_at": "..." }
  }
}
```

`progress` 记的是**每个条目当天完成了几步**。v1 格式（`"checked": ["anki", ...]`）仍然可读，
按「该条目满步」解释，所以升级不会丢历史。

- **本地日期**分天（不是 UTC），过零点自动换新的一天；终端一直开着也会在 30 秒内察觉日期变化
- 写入是**原子**的（同目录临时文件 + rename），掉电/强杀不会留下半截文件
- 文件损坏时会被挪到 `history.json.corrupt-<时间戳>` 并重新开工，不会每次运行都报错
- 单个时间戳写坏不会毁掉整份记录（时间戳按可缺省字段解析）
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
go test ./...          # 87 个单测：配置解析 / 持久化 / 步骤与统计口径 / TUI 状态机 / 渲染降级
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
