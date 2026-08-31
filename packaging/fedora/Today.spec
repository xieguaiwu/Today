%global debug_package %{nil}

Name:           Today
Version:        0.2.0
Release:        1%{?dist}
Summary:        Terminal daily self-check checklist (bubbletea TUI)

License:        MIT
URL:            https://github.com/xieguaiwu/Today
Source0:        %{url}/archive/v%{version}.tar.gz#/%{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.24

%description
Today is a terminal interactive daily self-check checklist built with
bubbletea. Move the cursor with arrow keys, toggle items with Space,
and quit with q.

Checks are written to disk immediately and stored per calendar day, so a new
day starts blank while the streak counter keeps track of consecutive days.

The checklist itself is configurable in ~/.config/Today/config.json (title,
items, per-item hints, history retention). A default config with the built-in
seven items is generated on first run.

Built-in items: Eyes / Nose / Skin / Lips / Anxiety / Cognition / Weight & Fat

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
