%define component_name {{COMPONENT_NAME}}
%define debug_package %{nil}
%define _build_id_links none

Name:           {{PACKAGE_NAME}}
Version:        %{version}
Release:        1%{?dist}
Summary:        {{PACKAGE_SUMMARY}}

License:        {{PACKAGE_LICENSE}}
URL:            {{PACKAGE_URL}}
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  systemd-rpm-macros
Requires:       policycoreutils-python-utils

%description
{{PACKAGE_DESCRIPTION}}

%prep
%setup -q

%build
# Binary is pre-built, nothing to do here

%install
rm -rf $RPM_BUILD_ROOT

# Install the binary to /opt/flomation/<component_name>/
install -d $RPM_BUILD_ROOT/opt/flomation/%{component_name}
install -m 0755 application $RPM_BUILD_ROOT/opt/flomation/%{component_name}/%{component_name}

# Install systemd service file
install -d $RPM_BUILD_ROOT%{_unitdir}
install -m 0644 %{name}.service $RPM_BUILD_ROOT%{_unitdir}/flomation-%{component_name}.service

# Create placeholder directories (actual directories created in %pre)
install -d $RPM_BUILD_ROOT/opt/flomation/%{component_name}/etc
install -d $RPM_BUILD_ROOT/opt/flomation/%{component_name}/logs

%pre
# Create directory structure first (before user creation)
mkdir -p /opt/flomation
mkdir -p /opt/flomation/snapshots

# Create user and group with specific IDs if they don't exist
if ! getent group flomation >/dev/null 2>&1; then
    groupadd -g 5000 flomation
fi

if ! getent passwd flomation >/dev/null 2>&1; then
    useradd -u 5000 -g 5000 -d /opt/flomation -s /sbin/nologin -c "Flomation Service Account" flomation 2>/dev/null || true
fi

# Set ownership on base directories
chown flomation:flomation /opt/flomation
chmod 755 /opt/flomation

chown flomation:flomation /opt/flomation/snapshots
chmod 2770 /opt/flomation/snapshots

# Create component directories
mkdir -p /opt/flomation/%{component_name}
mkdir -p /opt/flomation/%{component_name}/etc
mkdir -p /opt/flomation/%{component_name}/logs

chown flomation:flomation /opt/flomation/%{component_name}
chmod 755 /opt/flomation/%{component_name}

chown flomation:flomation /opt/flomation/%{component_name}/etc
chmod 750 /opt/flomation/%{component_name}/etc

chown flomation:flomation /opt/flomation/%{component_name}/logs
chmod 750 /opt/flomation/%{component_name}/logs

# On upgrade: stop service, take snapshot
if [ $1 -gt 1 ]; then
    # This is an upgrade - stop the service first
    systemctl stop flomation-%{component_name}.service >/dev/null 2>&1 || true

    # Get the old version from RPM database (exclude the package being installed)
    OLD_VERSION=$(rpm -q --last %{name} 2>/dev/null | tail -1 | awk '{print $1}' | sed 's/%{name}-//' || echo "unknown")
    TIMESTAMP=$(date +%%Y%%m%%d_%%H%%M%%S)
    SNAPSHOT_NAME="flomation-%{component_name}-${OLD_VERSION}-upgrade-${TIMESTAMP}.tgz"

    if [ -d /opt/flomation/%{component_name} ]; then
        echo "Creating snapshot: ${SNAPSHOT_NAME}"
        tar czf /opt/flomation/snapshots/${SNAPSHOT_NAME} -C /opt/flomation %{component_name} 2>/dev/null || true
        chown flomation:flomation /opt/flomation/snapshots/${SNAPSHOT_NAME}
        chmod 640 /opt/flomation/snapshots/${SNAPSHOT_NAME}
    fi
fi

%post
# Create symlink from /var/log/flomation/<component_name> to /opt/flomation/<component_name>/logs
mkdir -p /var/log/flomation
ln -sf /opt/flomation/%{component_name}/logs /var/log/flomation/%{component_name}

# Set SELinux contexts if SELinux is enabled
if command -v getenforce >/dev/null 2>&1 && [ "$(getenforce)" != "Disabled" ]; then
    # Set context for application directory and binary
    semanage fcontext -a -t usr_t "/opt/flomation/%{component_name}(/.*)?" 2>/dev/null || true
    restorecon -Rv /opt/flomation/%{component_name} 2>/dev/null || true

    # Set context for log directory
    semanage fcontext -a -t var_log_t "/opt/flomation/%{component_name}/logs(/.*)?" 2>/dev/null || true
    restorecon -Rv /opt/flomation/%{component_name}/logs 2>/dev/null || true

    # Set context for snapshot directory
    semanage fcontext -a -t usr_t "/opt/flomation/snapshots(/.*)?" 2>/dev/null || true
    restorecon -Rv /opt/flomation/snapshots 2>/dev/null || true

    # Set context for /var/log/flomation symlink
    semanage fcontext -a -t var_log_t "/var/log/flomation(/.*)?" 2>/dev/null || true
    restorecon -Rv /var/log/flomation 2>/dev/null || true
fi

# Handle service based on install type
%systemd_post flomation-%{component_name}.service
if [ $1 -eq 1 ]; then
    # Fresh install - enable but don't start
    systemctl daemon-reload >/dev/null 2>&1 || true
    systemctl enable flomation-%{component_name}.service >/dev/null 2>&1 || true
elif [ $1 -gt 1 ]; then
    # Upgrade - restart the service
    systemctl daemon-reload >/dev/null 2>&1 || true
    systemctl start flomation-%{component_name}.service >/dev/null 2>&1 || true
fi

%preun
%systemd_preun flomation-%{component_name}.service

%postun
%systemd_postun_with_restart flomation-%{component_name}.service

# Remove SELinux contexts and symlink on uninstall (not upgrade)
if [ $1 -eq 0 ]; then
    # Remove symlink
    rm -f /var/log/flomation/%{component_name}

    if command -v getenforce >/dev/null 2>&1 && [ "$(getenforce)" != "Disabled" ]; then
        semanage fcontext -d "/opt/flomation/%{component_name}(/.*)?" 2>/dev/null || true
        semanage fcontext -d "/opt/flomation/%{component_name}/logs(/.*)?" 2>/dev/null || true
        semanage fcontext -d "/var/log/flomation(/.*)?" 2>/dev/null || true
    fi
fi

%files
%defattr(-,flomation,flomation,-)
%attr(750,flomation,flomation) /opt/flomation/%{component_name}/%{component_name}
%attr(644,root,root) %{_unitdir}/flomation-%{component_name}.service
%dir %attr(750,flomation,flomation) /opt/flomation/%{component_name}
%dir %attr(750,flomation,flomation) /opt/flomation/%{component_name}/etc
%dir %attr(750,flomation,flomation) /opt/flomation/%{component_name}/logs

%changelog
* {{BUILD_DATE_RPM}} Build System <build@flomation.co> - %{version}-1
- Automated build
- Build commit: {{BUILD_COMMIT_SHA}}
- Build time: {{BUILD_TIMESTAMP}}
