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
- `/prom/cluster/{cluster_name}/rules/{rules_file}/rules` # 告警规则文件内容
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
- 对应templates/prometheus_configs.html
- 点击菜单"prom配置文件管理"时展示prometheus 节点列表
- 节点列表有一个操作按钮"编辑"，点击"编辑"按钮，弹出prometheus 配置文件编辑弹窗
- 弹窗中展示prometheus 配置文件内容
- 弹窗中有一个"保存"按钮，点击"保存"按钮，保存prometheus 配置文件

2、go 实现
- 在prom_handlers.go 中实现,ui中需要的操作逻辑。
- 在service目录下实现prom_service.go实现consul 操作
- 保存prometheus config时，使用prometheus库校验配置文件是否正确


### prometheus 告警规则管理
1、ui 页面
- 对应templates/prometheus_rules.html
- 点击菜单"prom告警规则管理"时展示prometheus 节点列表
- 点击节点，展示该节点下的告警规则列表
- 告警规则列表有一个操作按钮"编辑"，点击"编辑"按钮，弹出prometheus 告警规则编辑弹窗
- 弹窗中展示prometheus 告警规则内容
- 弹窗中有一个"保存"按钮，点击"保存"按钮，保存prometheus 告警规则
- 告警规则列表有一个操作按钮"启用"，点击"启用"按钮，启用该告警规则
- 告警规则列表有一个操作按钮"禁用"，点击"禁用"按钮，禁用该告警规则
- 告警规则列表有一个操作按钮"删除"，点击"删除"按钮，删除该告警规则
- 告警规则列表有一个操作按钮"添加"，点击"添加"按钮，弹出prometheus 告警规则编辑弹窗

2、go 实现
- 在prom_handlers.go 中实现,ui中需要的操作逻辑。
- 在service目录下实现prom_service.go实现consul 操作



### agent.sh 设计
变量：
- PROMETHEUS_CLUSTER_NAME: 主机名+ip
- ALERTMANAGER_CLUSTER_NAME: 主机名+ip
- CONSUL_ADDR: consul 地址
- CONSUL_TOKEN: consul token
- PROMETHEUS_CONFIG_PATH: prometheus 配置文件路径
- PROMETHEUS_RULES_DIR_PATH: prometheus 告警规则目录路径
- ALERTMANAGER_RULES_PATH: alertmanager 告警规则路径 
- ALERTMANAGER_CONFIG_PATH: alertmanager 配置文件路径

输出脚本到项目agnet目录下面

实现功能：
- 在内存中建立consul key 的index 缓存，用于判断consul key 是否发生变化
- watch consul key: `/prom/cluster/{cluster_name}/`
- 使用类型的curl 命令：RESPONSE=$(curl -H "X-Consul-Token: ${TOKEN}" -s "${CONSUL_URL}/${KEY}?index=${INDEX}&wait=5m&recurse")
- 根据不同的key路由到不同的处理逻辑
- prometheus 相关处理逻辑（根据index 判断是否发生变化,如果cache 中没有index，则更新cache并默认key发生变化）： 
1、如果key`/prom/cluster/{cluster_name}/config` 发生变化，则更新prometheus 配置文件
2、如果key`/prom/cluster/{cluster_name}/rules/{rules_file}` 发生变化，则更新prometheus 告警规则
3、如果key`/prom/cluster/{cluster_name}/rules/{rules_file}/enable` 发生变化，则更新prometheus 告警规则启用状态








