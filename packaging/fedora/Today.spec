%global debug_package %{nil}

Name:           Today
Version:        0.3.0
Release:        1%{?dist}
Summary:        Terminal daily self-check checklist (bubbletea TUI)

License:        MIT
URL:            https://github.com/xieguaiwu/Today
Source0:        %{url}/archive/v%{version}.tar.gz#/%{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.24

%description
Today is a terminal interactive daily self-check checklist built with
bubbletea. Move the cursor with arrow keys, advance steps with Space,
and quit with q.

Progress is written to disk immediately and stored per calendar day, so a new
day starts blank while the streak counter keeps track of consecutive days.

Habits may be split into sub-steps ("steps": N in the config); partial progress
is shaded in the heatmap but deliberately does not count toward a streak.

A statistics view shows current streak, best streak, completions and a 90-day
rate, plus a year heatmap and a per-weekday chart.

The checklist is configurable in ~/.config/Today/config.json (title, items,
per-item hints, colours, groups, step counts, history retention). A default
config is generated on first run.

Built-in habits: anki单词 / USACO / 无氧锻炼 / PMA / brain, plus the original
Eyes / Nose / Skin / Lips / Anxiety / Cognition / Weight & Fat self-check items.

%prep
%setup -q -n Today-%{version}

%build
export GOFLAGS="-mod=vendor"
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

go build -trimpath -ldflags="-s -w" -o Today .

%install
rm -rf %{buildroot}
install -Dm755 Today %{buildroot}%{_bindir}/Today
install -Dm644 LICENSE %{buildroot}%{_defaultlicensedir}/%{name}/LICENSE
install -Dm644 README.md %{buildroot}%{_defaultdocdir}/%{name}/README.md

%files
%license LICENSE
%doc README.md
%{_bindir}/Today

%changelog
* Tue Sep 01 2026 xgw <xieguaiwu@163.com> - 0.3.0-1
- Steps: habits can require N sub-steps ("steps" in config); Space advances one
  step, f fills, +/- nudge. Partial progress is shaded in the heatmap but does
  not count toward streaks or completion totals
- Statistics view: current streak, best streak, completions, 90-day rate, year
  heatmap with month ruler, and a per-weekday chart
- Per-item colour and group in the config; list rows show this week's cells
- History format v2 stores per-item step counts; v1 "checked" lists still read
  and are treated as fully complete, so upgrading loses nothing
- --status now also reports the partial count

* Mon Aug 31 2026 xgw <xieguaiwu@163.com> - 0.2.0-1
- Checklist moved out of the source into ~/.config/Today/config.json
  (title, items, per-item hints, history_days); default file generated on first run
- Per-day persistence: toggles are written atomically right away, the day
  rolls over at local midnight, streak and totals are shown in the header
- Fixed the header, which still read "What to buy?" from the bubbletea tutorial
- Added --status / --reset / --config / --data / --version
- History file is mode 0600; a corrupt history is quarantined instead of fatal
- Added unit tests for config, store and TUI state machine

* Tue Aug 18 2026 xgw <xieguaiwu@163.com> - 0.1.0-1
- Initial package: daily self-check checklist TUI (v0.1.0)
