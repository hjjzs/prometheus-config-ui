# 基于Consul的promethus告警、配置、服务发现 全流程解决方案

## 需求
- 基于Consul 保存prometheus/alertmanager 配置文件
- 基于Consul 保存prometheus 告警规则
- 基于Consul 保存alertmanager 告警规则
- 编写agent.sh脚本监听consul数据变化，并更新prometheus/alertmanager 配置文件，告警规则
- 基于go template 实现consul ui, 展示prometheus/alertmanager 配置文件，告警规则,已经编辑配置文件和告警配置。

- consul 需要管理和保存多个prometheus/alertmanager 配置文件，告警规则，告警信息

## 技术栈
- go
- consul
- prometheus
- alertmanager
- go template

## 功能设计
### consul key 设计
根路径：
- prometheus: `/prom`  # 下面保存prometheus 所有key
- alertmanager: `/alert` # 下面保存alertmanager 所有key

prometheus key 设计：
- cluster: `/prom/cluster/{cluster_name}` # 集群名称,一个prometheus为一个集群

- config: `/prom/cluster/{cluster_name}/config` # 配置文件
- rules: `/prom/cluster/{cluster_name}/rules/{rules_file}` # 一个rules_file 对应一个告警规则文件
- `/prom/cluster/{cluster_name}/rules/{rules_file}/rules.yml` # 告警规则文件内容
- `/prom/cluster/{cluster_name}/rules/{rules_file}/enable` # 同路径下的rule.yml告警规则文件是否启用

alertmanager key 设计：
- 待定

### consul ui 设计
使用go template 渲染页面，consul-client 获取consul数据，并展示
页面：
- prometheus 配置文件管理
- prometheus 告警规则管理
- alertmanager 配置文件管理
- alertmanager 告警规则管理
- 用户管理（consul token） 一个consul token 对应一个用户
- 角色管理（consul token 权限）

布局:
- 菜单（左侧）：prometheus 配置文件管理、prometheus 告警规则管理、alertmanager 配置文件管理、alertmanager 告警规则管理、用户管理（consul token） 一个consul token 对应一个用户、角色管理（consul token 权限）
- 内容（右侧）：显示菜单对应的页面
- 右侧菜单可以折叠
- 可以使用bootstrap 框架



### prometheus 配置文件管理
1、ui 页面
- 展示prometheus节点
- 点击prometheus节点，展示prometheus 配置文件
- 如果prometheus 配置文件不存在，则展示添加prometheus 配置文件按钮
- 点击添加prometheus 配置文件按钮，弹出添加prometheus 配置文件弹窗
- 添加prometheus 配置文件弹窗，展示prometheus 配置文件内容
- 配置文件弹窗,分配置文件global、alerting、rule_files、scrape_configs部分，对应prometheus 配置文件的global、alerting、rule_files、scrape_configs部分
2、go 实现
- 在prom_handlers.go 中实现使用prometheus官方库实现consul中保存的config yml 配置解析。
- 将解析的prometheus.yml文件展示到ui页面
- 在consul_service.go 中实现consul 操作





<!-- ### 用户管理模块设计
- 用户管理：添加用户、删除用户、修改用户、用户列表
- 在service目录下面创建consul_service.go 文件，实现consul操作
- 在user_handler.go中实现用户管理功能 -->






<!-- ### agent.sh 设计
变量：
- PROMETHEUS_CLUSTER_NAME: 主机名+ip
- ALERTMANAGER_CLUSTER_NAME: 主机名+ip
- CONSUL_ADDR: consul 地址
- CONSUL_TOKEN: consul token
- PROMETHEUS_CONFIG_PATH: prometheus 配置文件路径
- PROMETHEUS_RULES_DIR_PATH: prometheus 告警规则目录路径
- ALERTMANAGER_RULES_PATH: alertmanager 告警规则路径 
- ALERTMANAGER_CONFIG_PATH: alertmanager 配置文件路径

实现功能：
- 扫描prometheus 告警规则目录，获取所有告警规则文件，并保存到consul中
- watch consul key: `/prom/cluster/{cluster_name}/`
- 根据不同的key路由到不同的处理逻辑 -->






