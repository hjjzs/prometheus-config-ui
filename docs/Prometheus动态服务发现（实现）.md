# Prometheus动态服务发现（实现）
> tode: 添加一个自动清理consul-server 中down掉的services脚本
>

**Consul 服务部署**：

**前置条件**

1、下载consul [https://releases.hashicorp.com/consul/1.20.1/consul_1.20.1_linux_amd64.zip](https://releases.hashicorp.com/consul/1.20.1/consul_1.20.1_linux_amd64.zip)

2、解压文件

3、将consul 可执行命令拷贝到`/usr/local/bin/`

**启动Consul**

创建consul开启acl的配置文件

```shell
mkdir -p /opt/consul/{config,data}
cd /opt/consul

cat > config/acl.hcl << EOF
acl = {
  enabled = true
  default_policy = "deny"   # 默认为 deny，意味着没有明确授权的请求会被拒绝
  enable_token_persistence = true   # 保持 token 持久化
}
EOF
```

编写consul.service 文件：

```shell
# 修改consul启动参数-bind 为自己服务器地址
cat > /usr/lib/systemd/system/consul.service << EOF
[Unit]
Description=Consul Service
Documentation=https://www.consul.io/
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/consul agent -server -ui -bootstrap-expect=1 -config-dir=/opt/consul/config -data-dir=/opt/consul/data -bind=192.168.48.129  -client=0.0.0.0 -log-file=/var/log/consul.log

ExecReload=/bin/kill -HUP $MAINPID
KillSignal=SIGINT
TimeoutStopSec=5
Restart=on-failure
SyslogIdentifier=consul

[Install]
WantedBy=multi-user.target
EOF
```

启动并开机自启服务

```shell
systemctl enable --now consul.service
```

初始化acl

```shell
consul acl bootstrap
```

![](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/meonaAk0bAprnXxj/img/56b671ca-2880-421e-bab2-ac482915c0c8.png)

添加tokens到consul，然后重启consul

```shell
cat > config/acl.hcl << EOF
acl = {
  enabled = true
  default_policy = "deny"   # 默认为 deny，意味着没有明确授权的请求会被拒绝
  enable_token_persistence = true   # 保持 token 持久化
  tokens {
    agent = "your token" # 添加你的token
  }
}
EOF

systemctl restart consul
```

**创建token及配置policy权限**

创建服务注册和proemtheus-read policy文件

```shell
mkdir policy-hcl
cat > policy-hcl/prometheus-read-policy.hcl << EOF
service_prefix "" {
  policy = "read"
}
node_prefix "" {
  policy = "read"
}
agent_prefix "" {
  policy = "read"
}
EOF

cat > policy-hcl/service-register-policy.hcl << EOF
service_prefix "" { 
  policy = "write"
}
EOF
```

通过文件创建policy

```shell
consul acl policy create -name "prometheus-read" -rules @policy-hcl/prometheus-read-policy.hcl -token 5e7f0c19-xxxxxx-eb77988cd641

consul acl policy create -name "service-register" -rules @policy-hcl/service-register-policy.hcl -token 5e7f0c19-xxxxx-eb77988cd641
```

创建token

```shell
# 改token 用于prom 服务发现
consul acl token create -description "Prometheus token for service discovery" -policy-name "prometheus-read" -token 5e7f0c19-xxx-eb77988cd641

# 用于服务注册到consul 使用
consul acl token create -description "service-register" -policy-name "service-register" -token 5e7f0c19-73ac-6023-c8ba-eb77988cd641
```

（拓展知识）访问consul web ui使用

登录：

+ 使用`consul acl bootstrap`命令输出的`SecretID（token）`登录



![](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/meonaAk0bAprnXxj/img/7524469b-769d-4695-83f2-a016094305cb.png)

+ 可以查看到使用命令创建的token，后续需要使用token时可以在web ui中复制。



![](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/meonaAk0bAprnXxj/img/63ae2ac9-49ba-4e52-ba8e-dbafc87aace7.png)

 **配置自动清理脚本**

用于自当清理不健康的consul service中的实例。

```shell
#!/bin/bash

# 默认值设置
CONSUL_HOST=""
CONSUL_TOKEN=""
LOG_FILE="/var/log/consul-down-clean.log"

# 检查jq是否安装
if ! command -v jq &> /dev/null; then
    echo "jq 未安装，请先安装 jq"
    exit 1
fi

# 验证必需参数
if [[ -z "$CONSUL_HOST" || -z "$CONSUL_TOKEN" ]]; then
    echo "错误: consul-host 和 token 是必需的参数"
    show_help
fi

# 获取所有服务的健康状态
echo "正在获取不健康的服务实例..."
services=$(curl -s -H "X-Consul-Token: ${CONSUL_TOKEN}" \
    "${CONSUL_HOST}/v1/health/state/critical" | \
    jq -r '.[] | select(.ServiceID != null) | .ServiceID')

if [ -z "$services" ]; then
    echo "没有发现不健康的服务实例"
    exit 0
fi

# 计数器
total=0
success=0
failed=0

# 遍历并注销不健康的服务
for service_id in $services; do
    echo "正在注销服务: $service_id"
    ((total++))
    
    response=$(curl -s -w "%{http_code}" -o /dev/null -X PUT \
        -H "X-Consul-Token: ${CONSUL_TOKEN}" \
        "${CONSUL_HOST}/v1/agent/service/deregister/${service_id}")
    
    if [ "$response" == "200" ]; then
        echo "成功注销服务: $service_id"
        ((success++))
    else
        echo "注销服务失败: $service_id, HTTP状态码: $response"
        ((failed++))
    fi
done

# 将统计信息写入日志文件，并且格式化
# 时间 - 总计处理 - 成功注销 - 失败数量
echo "$(date) - 总计处理: $total - 成功注销: $success - 失败数量: $failed" >> $LOG_FILE

```

**Prometheus 搭建**

**基于docker-compose部署：**

```shell
cat > docker-compose.yaml << EOF
version: '3.8'
services:
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - 9090:9090
    command: --config.file=/etc/prometheus/prometheus.yml --web.external-url="http://192.168.48.129:9090/" #填外部访问地址
    volumes:
      - ./prometheus/etc:/etc/prometheus:ro
      - ./prometheus/data:/prometheus/data
    networks:
      - prome-net
networks:
  prome-net:
    ipam:
      driver: default
      config:
        - subnet: "10.121.79.0/24"
EOF
```

**配置prometheus.yml文件：**

```shell
mkdir -p prometheus/{data,etc}
chown 65534:65534 prometheus/data    # 65534为prometheus容器内部nobody uid、gid，不然启动报错没权限

cat > prometheus/etc/prometheus.yml << EOF
global:
  scrape_interval:     15s # Set the scrape interval to every 15 seconds. Default is every 1 minute.
  evaluation_interval: 15s # Evaluate rules every 15 seconds. The default is every 1 minute.
  # scrape_timeout is set to the global default (10s).

# Alertmanager configuration
alerting:
  alertmanagers:
  - static_configs:
    - targets:
      - alertmanager:9093

# Load rules once and periodically evaluate them according to the global 'evaluation_interval'.
rule_files:
  - "/etc/prometheus/rules/basic.yml"
  # - "first_rules.yml"
  # - "second_rules.yml"

scrape_configs:
  - job_name: api-server
    scrape_interval: 15s
    scrape_timeout: 5s
    consul_sd_configs:
      - server: '192.168.48.129:8500' # 修改你consul服务所在的ip地址
        refresh_interval: 5s
        token: "0ddcf46a-79a9-98f5-480b-c4b6e3d1f428" # consul中创建用于prom 服务发现使用的token
        #services: []
        services: ["api-server"] # 这里是匹配注册到consul中的服务名称
        #tags: ['centos','node-exporter'] # 这是匹配注册到consul中的tags, 要对应一致，不然查找到对象
  - job_name: core-server
    scrape_interval: 15s
    scrape_timeout: 5s
    consul_sd_configs:
      - server: '192.168.48.129:8500' # 修改你consul服务所在的ip地址
        refresh_interval: 5s
        token: "0ddcf46a-79a9-98f5-480b-c4b6e3d1f428" 
        #services: []
        services: ["core-server"] # 这里是匹配注册到consul中的服务名称
        #tags: ['centos','node-exporter'] # 这是匹配注册到consul中的tags, 要对应一致，不然查找到对象

EOF
```

**consul-client 端rpm构建**

1、下载node_exporter包： 

`wget https://github.com/prometheus/node_exporter/releases/download/v1.8.2/node_exporter-1.8.2.linux-amd64.tar.gz`

2、解压，待用

3、创建consul-client-init.sh脚本

脚本主要实现被监控端主机的初始化操作，如设置consul-server 的地址，连接consul-server 需要的token。以及该设备所属的组。

```shell
#!/bin/bash

# 默认值设置
CONSUL_HOST=""
CONSUL_TOKEN=""
GROUP=""

# 显示帮助信息
show_help() {
    echo "用法: $0 [选项]"
    echo "选项:"
    echo "  --consul-host   指定Consul服务器地址 (必需)"
    echo "  --token        指定Consul连接Token (必需)"
    echo "  --group        指定主机组(必需)"
    echo "  -h, --help     显示帮助信息"
    exit 1
}

# 参数解析
while [[ $# -gt 0 ]]; do
    case $1 in
        --consul-host)
            CONSUL_HOST="$2"
            shift 2
            ;;
        --token)
            CONSUL_TOKEN="$2"
            shift 2
            ;;
        --group)
            GROUP="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            ;;
        *)
            echo "未知参数: $1"
            show_help
            ;;
    esac
done

# 验证必需参数
if [[ -z "$CONSUL_HOST" || -z "$CONSUL_TOKEN" || -z "$GROUP" ]]; then
    echo "错误: consul-host 和 token 和 group 是必需的参数"
    show_help
fi




# 创建配置目录
mkdir -p /etc/consul.d

# 生成register.sh
cat > /etc/consul.d/register.sh << EOF
#!/usr/bin/env bash
CONSUL_HOST="${CONSUL_HOST}"
CONSUL_TOKEN="${CONSUL_TOKEN}"
HOSTNAME="\$(hostname)"
HOST_ADDR="\$(hostname -I | awk '{print \$1}')"
HOST_ID="\${HOSTNAME}-\${HOST_ADDR}"


curl -X PUT \
    -H "X-Consul-Token:\${CONSUL_TOKEN}" \
    -d '{
        "id": "'\${HOST_ID}'",
        "name": "${GROUP}",
        "address": "'\${HOST_ADDR}'",
        "port": 9100,
        "tags": ["node-exporter,centos"],
        "checks": [{
            "http": "http://'\${HOST_ADDR}':9100/",
            "interval": "5s"
        }]
    }' \
    http://\${CONSUL_HOST}/v1/agent/service/register

if [ \$? -ne 0 ]; then
    # 如果curl命令失败，返回非0返回码
    exit 1
else
    # 如果curl命令成功，返回0返回码
    exit 0
fi
EOF

# 生成unregister.sh
cat > /etc/consul.d/unregister.sh << EOF
#!/bin/bash
CONSUL_HOST="${CONSUL_HOST}"
CONSUL_TOKEN="${CONSUL_TOKEN}"
HOST_ID="\$(hostname)-\$(hostname -I | awk '{print \$1}')"

curl -X PUT \
    -H "X-Consul-Token: \${CONSUL_TOKEN}" \
    http://\${CONSUL_HOST}/v1/agent/service/deregister/\${HOST_ID}
EOF

# 生成node_exporter systemd服务文件
cat > /etc/systemd/system/node_exporter.service << EOF
[Unit]
Description=Node Exporter
After=network.target
Before=consul-client.service

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/node_exporter
ExecReload=/bin/kill -HUP $MAINPID
KillSignal=SIGINT
TimeoutStopSec=5
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

# 生成consul-client服务文件
cat > /etc/systemd/system/consul-client.service << EOF
[Unit]
Description=Consul Client Service
After=network.target node_exporter.service
Requires=node_exporter.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/etc/consul.d/register.sh
ExecStop=/etc/consul.d/unregister.sh

[Install]
WantedBy=multi-user.target
EOF

# 设置执行权限
chmod +x /etc/consul.d/register.sh
chmod +x /etc/consul.d/unregister.sh

# 重新加载systemd配置并启动服务
systemctl daemon-reload
systemctl enable node_exporter
systemctl restart node_exporter

systemctl enable consul-client
systemctl restart consul-client

echo "Consul客户端初始化完成！"

```

4、创建build-rpm.sh 构建工具脚本

```shell
#!/bin/bash

# 设置变量
PACKAGE_NAME="node-exporter-consul"
VERSION="1.0.0"
RELEASE="1"
BUILD_ROOT="$(pwd)/rpmbuild"
SPEC_FILE="${BUILD_ROOT}/SPECS/${PACKAGE_NAME}.spec"

# 显示帮助信息
show_help() {
    echo "用法: $0 [init|build]"
    echo "命令:"
    echo "  init    初始化RPM构建环境"
    echo "  build   构建RPM包"
    exit 1
}

# 检查必需文件
check_required_files() {
    if [ ! -f "consul-client-init.sh" ]; then
        echo "错误: consul-client-init.sh 文件不存在！"
        echo "请确保consul-client-init.sh文件在当前目录。"
        exit 1
    fi

    # 检查文件是否具有执行权限
    if [ ! -x "consul-client-init.sh" ]; then
        echo "警告: consul-client-init.sh 没有执行权限，正在添加..."
        chmod +x consul-client-init.sh
    fi

    if [ ! -f "node_exporter" ]; then
        echo "错误: node_exporter 文件不存在！"
        echo "请将node_exporter二进制文件复制到当前目录"
        exit 1
    fi

    if [ ! -x "node_exporter" ]; then
        echo "警告: node_exporter 没有执行权限，正在添加..."
        chmod +x node_exporter
    fi
}

# 初始化构建环境
init_env() {
    echo "正在初始化构建环境..."
    
    # 检查必需文件
    check_required_files
    
    # 安装必要的RPM构建工具
    sudo yum -y install rpm-build rpmdevtools gcc make

    # 创建RPM构建目录结构
    mkdir -p ${BUILD_ROOT}/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
    mkdir -p ${BUILD_ROOT}/SOURCES/${PACKAGE_NAME}-${VERSION}

    # 复制所需文件到SOURCES目录
    cp consul-client-init.sh ${BUILD_ROOT}/SOURCES/${PACKAGE_NAME}-${VERSION}/
    
    cp node_exporter ${BUILD_ROOT}/SOURCES/${PACKAGE_NAME}-${VERSION}/

    # 创建spec文件
    cat > ${SPEC_FILE} << EOF
Name:           ${PACKAGE_NAME}
Version:        ${VERSION}
Release:        ${RELEASE}%{?dist}
Summary:        Node Exporter with Consul registration
Group:         System Environment/Daemons
License:        MIT
URL:            http://example.com
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  systemd
Requires:       curl
Requires:       systemd

%description
Node Exporter with automatic Consul registration capability

%prep
%setup -q

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}/usr/local/bin
mkdir -p %{buildroot}/usr/lib/systemd/system

# 安装node_exporter
install -m 755 node_exporter %{buildroot}/usr/local/bin/

# 安装consul注册脚本
install -m 755 consul-client-init.sh %{buildroot}/usr/local/bin/

%files
%defattr(-,root,root,-)
/usr/local/bin/node_exporter
/usr/local/bin/consul-client-init.sh

%post
echo "请运行consul-client-init.sh进行初始化节点"

%preun
if [ \$1 -eq 0 ]; then
    systemctl stop node_exporter.service >/dev/null 2>&1 || :
    systemctl disable node_exporter.service >/dev/null 2>&1 || :
    systemctl stop consul-client.service >/dev/null 2>&1 || :
    systemctl disable consul-client.service >/dev/null 2>&1 || :
fi

%postun
if [ \$1 -eq 0 ]; then
    rm -rf /etc/consul.d
    rm -f /etc/systemd/system/consul-client.service
    rm -f /etc/systemd/system/node_exporter.service
    rm -f /usr/local/bin/consul-client-init.sh
    rm -f /usr/local/bin/node_exporter
    systemctl daemon-reload >/dev/null 2>&1 || :
fi

%changelog
* $(date "+%a %b %d %Y") Builder <builder@example.com> - ${VERSION}-${RELEASE}
- Initial package release
EOF

    echo "构建环境初始化完成！"
}

# 构建RPM包
build_rpm() {
    echo "开始构建RPM包..."

    # 检查源文件目录是否存在
    if [ ! -d "${BUILD_ROOT}/SOURCES/${PACKAGE_NAME}-${VERSION}" ]; then
        echo "错误: 源文件目录不存在！"
        echo "请先运行 '$0 init' 初始化构建环境。"
        exit 1
    fi
    
    # 检查必需文件
    check_required_files
    
    # 检查node_exporter是否存在
    if [ ! -f "${BUILD_ROOT}/SOURCES/${PACKAGE_NAME}-${VERSION}/node_exporter" ]; then
        echo "错误: node_exporter文件不存在！"
        echo "请将node_exporter复制到: ${BUILD_ROOT}/SOURCES/${PACKAGE_NAME}-${VERSION}/"
        exit 1
    fi

    # 创建源码包
    cd ${BUILD_ROOT}/SOURCES
    tar czf ${PACKAGE_NAME}-${VERSION}.tar.gz ${PACKAGE_NAME}-${VERSION}

    # 构建RPM包
    rpmbuild --define "_topdir ${BUILD_ROOT}" -ba ${SPEC_FILE}

    if [ $? -eq 0 ]; then
        echo "RPM包构建成功！"
        echo "RPM包位置: ${BUILD_ROOT}/RPMS/x86_64/${PACKAGE_NAME}-${VERSION}-${RELEASE}*.rpm"
    else
        echo "RPM包构建失败！"
        exit 1
    fi
}

# 主程序
case "$1" in
    init)
        init_env
        ;;
    build)
        build_rpm
        ;;
    *)
        show_help
        ;;
esac 
```

5、构建rpm包

环境准备

```shell
# 创建工作环境
mkdir -p /opt/consul-client
cd /opt/consul-client
# 创建上面的两个脚本，并拷贝node_exporter 二进制文件到工作目录。
```

![](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/meonaAk0bAprnXxj/img/8eefd73b-bc2d-41f9-b069-243f7ca9ad4a.png)

初始化构建

```shell
./build-rpm.sh init   # 初始化构建环境
```

![](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/meonaAk0bAprnXxj/img/49cd24b9-6d40-41c4-a4f7-90b7d1ceb517.png)

![](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/meonaAk0bAprnXxj/img/f3f624fb-eef9-4e0b-9cc5-aa2991755214.png)

构建rpm

```shell
./build-rpm.sh build
```

![](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/meonaAk0bAprnXxj/img/c51c87d3-2ce7-494a-9f1c-094caee7add0.png)

**rpm 部署和使用**

rpm 包需要安装在需要被监控的服务器上，安装完成后需要运行初始化脚本。当这台初始化好的虚拟机通过弹性伸缩拓展数量时，新添加的机器会被prometheus 自动发现并监控。

**rpm 安装**

```shell
rpm -ivh node-exporter-consul-1.0.0-1.el7.x86_64.rpm
```

![](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/meonaAk0bAprnXxj/img/06916346-dbf9-4ea7-a8d6-cbf3e1d3528e.png)

**运行初始化脚本**

```shell
$consul-client-init.sh -h
用法: /usr/local/bin/consul-client-init.sh [选项]
选项:
  --consul-host   指定Consul服务器地址,格式ip:port 例如:172.128.10.10:8500 (必需)
  --token        指定Consul连接Token  (必需)
  --group        指定主机组(必需)
  -h, --help     显示帮助信息

```

```shell
consul-client-init.sh \
  --consul-host 192.168.48.129:8500 \
  --token 9fb3ed00-79e0-7996-720e-c02f4200810e \
  --group api-server
```

到此步骤，这台新设备以成功被prometheus 监控。

![](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/meonaAk0bAprnXxj/img/bc632155-7700-4ea0-af98-e32fd748671e.png)

![](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/meonaAk0bAprnXxj/img/30f96b4c-7e26-41ad-a7d9-97798604deea.png)

**卸载rpm包**

```shell
rpm -e node-exporter-consul
```

卸载rpm和正常关机会自动从consul服务器，注销。

