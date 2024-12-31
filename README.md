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
- cluster: `/alert/cluster/{cluster_name}` # 集群名称,一个alertmanager为一个集群
- config: `/alert/cluster/{cluster_name}/config` # 配置文件
- tmpl: `/alert/cluster/{cluster_name}/tmpl/{tmpl_file}` # 一个tmpl_file 对应一个告警模板文件
- `/alert/cluster/{cluster_name}/tmpl/{tmpl_file}/tmpl` # 告警模板文件内容
- `/alert/cluster/{cluster_name}/tmpl/{tmpl_file}/enable` # 同路径下的tmpl_file.tmpl告警模板文件是否启用


### consul ui 设计
使用go template 渲染页面，consul-client 获取consul数据，并展示
页面：
- prometheus 配置文件管理
- prometheus 告警规则管理
- alertmanager 配置文件管理
- alertmanager 告警模板管理
- 用户管理（consul token） 一个consul token 对应一个用户
- 角色管理（consul token 权限）

布局:
- 菜单（左侧）：prometheus 配置文件管理、prometheus 告警规则管理、alertmanager 配置文件管理、alertmanager 告警模板管理、用户管理（consul token） 一个consul token 对应一个用户、角色管理（consul token 权限）
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


### alertmanager 配置文件管理
1、ui 页面
1.1对应templates/alertmanager_configs.html
- 点击菜单"alertmanager配置文件管理"时展示alertmanager 节点列表
- 节点列表有一个操作按钮"编辑"，点击"编辑"按钮，弹出alertmanager 配置文件编辑弹窗
- 弹窗中展示alertmanager 配置文件内容
- 使用monaco editor 编辑器
- 弹窗中有一个"保存"按钮，点击"保存"按钮，保存alertmanager 配置文件

1.2 对应templates/alertmanager_tmpls.html
- 点击菜单"alertmanager告警模板管理"时展示alertmanager 节点列表
- 节点列表有一个操作按钮"编辑"，点击"编辑"按钮，弹出alertmanager 告警模板编辑弹窗
- 弹窗中展示alertmanager 告警模板内容
- 使用monaco editor 编辑器
- 弹窗中有一个"保存"按钮，点击"保存"按钮，保存alertmanager 告警模板
- 告警模板列表有一个操作按钮"启用"，点击"启用"按钮，启用该告警模板
- 告警模板列表有一个操作按钮"禁用"，点击"禁用"按钮，禁用该告警模板
- 告警模板列表有一个操作按钮"删除"，点击"删除"按钮，删除该告警模板
- 告警模板列表有一个操作按钮"添加"，点击"添加"按钮，弹出alertmanager 告警模板编辑弹窗
2、go 实现
- 在alert_handlers.go 中实现,ui中需要的操作逻辑。
- 在service目录下实现alert_service.go实现consul 操作




### agent.sh 设计
变量：
- PROMETHEUS_CLUSTER_NAME: 主机名+ip
- ALERTMANAGER_CLUSTER_NAME: 主机名+ip
- CONSUL_ADDR: consul 地址
- CONSUL_TOKEN: consul token
- PROMETHEUS_CONFIG_PATH: prometheus 配置文件路径
- PROMETHEUS_RULES_DIR_PATH: prometheus 告警规则目录路径
- ALERTMANAGER_TMPL_PATH: alertmanager 告警模板路径 
- ALERTMANAGER_CONFIG_PATH: alertmanager 配置文件路径

 
- `ENABLE_CONSUL_REGISTRY=${ENABLE_CONSUL_REGISTRY:-"true"}` 是否开启consul 注册,主动注册prometheus/alertmanager 服务到consul
 
- `ENABLE_UPLOAD=${ENABLE_UPLOAD:-"true"}`是否开启consul 配置上传, 主动上传prometheus/alertmanager 配置、告警规则、告警模板到consul

prometheus 实现功能：
- 使用declare -A last_modify_indices 在内存中建立consul key 的ModifyIndex缓存,用于判断key是否变化
- 分别实现watch_config和watch_rules两个函数监听配置和规则变化
- watch_config函数:
  1. 监听/prom/cluster/{cluster_name}/config路径
  2. 使用curl长轮询等待变更:curl -H "X-Consul-Token: $CONSUL_TOKEN" "$CONSUL_ADDR/v1/kv/$cluster_path/config?index=${index}&wait=5m"
  3. 检测到变化时更新prometheus.yml配置文件并重载
- watch_rules函数:
  1. 监听/prom/cluster/{cluster_name}/rules路径
  2. 使用curl长轮询递归监听:curl -H "X-Consul-Token: $CONSUL_TOKEN" "$CONSUL_ADDR/v1/kv/$rules_path?index=${index}&wait=5m&recurse"
  3. 根据key类型路由到不同处理函数:
     - rules/{rule_file}/rules: 更新规则文件内容
     - rules/{rule_file}/enable: 更新规则启用状态
     - rules 事件优先级高于enable 事件，先同步rules 配置，然后同步enable状态，防止当rules下次enable时，文件内容与consul不一致。
  4. 通过比较ModifyIndex与缓存判断是否需要处理
  5. 处理完成后更新缓存的ModifyIndex

- restart_prometheus函数:
  1. 使用调用prometheus 的reload api (默认开启，下面方法注释，需要取消注释)
  2. 通过kill -HUP 发送信号重启prometheus
  3. 针对docker 容器化的prometheus, 使用docker kill -s HUP <container_id> 重启prometheus

alertmanager 实现功能：
- 使用declare -A last_modify_indices 在内存中建立consul key 的ModifyIndex缓存,用于判断key是否变化
- 分别实现watch_config和watch_tmpl两个函数监听配置和模板变化
- watch_alert_config函数:
  1. 监听/alert/cluster/{cluster_name}/config路径
  2. 使用curl长轮询等待变更:curl -H "X-Consul-Token: $CONSUL_TOKEN" "$CONSUL_ADDR/v1/kv/$cluster_path/config?index=${index}&wait=5m"
  3. 检测到变化时更新alertmanager.yml配置文件
- watch_tmpl函数:
  1. 监听/alert/cluster/{cluster_name}/tmpl路径
  2. 使用curl长轮询递归监听:curl -H "X-Consul-Token: $CONSUL_TOKEN" "$CONSUL_ADDR/v1/kv/$tmpl_path?index=${index}&wait=5m&recurse"
  3. 根据key类型路由到不同处理函数:
     - tmpl/{tmpl_file}/tmpl: 更新模板文件内容
     - tmpl/{tmpl_file}/enable: 更新模板启用状态
     - tmpl 事件优先级高于enable 事件，先同步tmpl 配置，然后同步enable状态，防止当tmpl下次enable时，文件内容与consul不一致。
- restart_alertmanager函数:
  1. 使用调用alertmanager 的reload api (默认开启，下面方法注释，需要取消注释)
  2. 通过kill -HUP 发送信号重启alertmanager
  3. 针对docker 容器化的alertmanager, 使用docker kill -s HUP <container_id> 重启alertmanager

consul_register函数:
- 如果本地prometheus 配置文件存在，则注册到consul。
- 如果本地prometheus配置文件不存在，且consul中存在prometheus配置文件，则不创建。
- 如果本地prometheus配置文件不存在，且consul中不存在prometheus配置文件，则创建默认配置（prometheus 默认空配置)
- 如果本地alertmanager 配置文件存在且不为空，则注册到consul。
- 如果本地alertmanager 配置文件不存在，且consul中存在alertmanager 配置文件，则不创建。
- 如果本地alertmanager 配置文件不存在，且consul中不存在alertmanager 配置文件，则创建默认配置
- 创建rules/tmpl consul key路径。

consul_upload函数:
- 上传prometheus 告警规则、告警模板到consul





