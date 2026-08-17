%global debug_package %{nil}

Name:           Today
Version:        0.1.0
Release:        1%{?dist}
Summary:        Terminal daily self-check checklist (bubbletea TUI)

License:        MIT
URL:            https://github.com/xieguaiwu/Today
Source0:        %{url}/archive/v%{version}.tar.gz#/%{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.24

%description
Today is a terminal interactive daily self-check checklist built with
bubbletea. Move the cursor with arrow keys, toggle items with Space,
and quit with q. Zero configuration required.

Built-in items: Eyes / Nose / Skin / Lips / Anxiety / Cognition / Weight & Fat

%prep
%setup -q -n Today-%{version}

%build
export GOFLAGS="-mod=mod"
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
* Tue Aug 18 2026 xgw <xieguaiwu@163.com> - 0.1.0-1
- Initial package: daily self-check checklist TUI (v0.1.0)
