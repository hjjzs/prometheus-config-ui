# prom/alert 配置告警同步系统实现

**背景：**

用Prometheus，资产登记，告警，配置更新这些，弄一套完整的解决方案出来，可以落地的。有没有可视化界面可以配置。

**框架设计**

![image](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/a/6E118PXkyiw7OWnj/66c03237574547c2a5155dab48e3cc7d0365.png)

*   基于Consul 保存prometheus/alertmanager 配置文件
    
*    基于Consul 保存prometheus 告警规则
    
*   基于Consul 保存alertmanager 告警规则，告警模板
    
*   编写agent.sh脚本监听consul数据变化，并更新prometheus/alertmanager 配置文件，告警规则，告警模板。
    
*   基于go template 实现consul ui, 展示prometheus/alertmanager 配置文件，告警规则，告警模板。
    
*    consul 需要管理和保存多个prometheus/alertmanager 配置文件，告警规则，告警信息
    

虚拟机发现部分实现：
./docs/Prometheus动态服务发现（实现）.md

**技术栈**

*   go
    
*   consul
    
*   prometheus
    
*   alertmanager
    
*   go template
    
*   jquery
    
*   bootstrup
    

**consul key/value 设计**

根路径：

*   prometheus: `/prom`  # 下面保存prometheus 所有key
    
*   alertmanager: `/alert` # 下面保存alertmanager 所有key
    

prometheus key 设计：

*   cluster: `/prom/cluster/{cluster_name}` # 集群名称,一个prometheus为一个集群
    
*   config: `/prom/cluster/{cluster_name}/config` # 配置文件
    
*   rules: `/prom/cluster/{cluster_name}/rules/{rules_file}` # 一个rules\_file 对应一个告警规则文件
    
*   `/prom/cluster/{cluster_name}/rules/{rules_file}/rules` # 告警规则文件内容
    
*   `/prom/cluster/{cluster_name}/rules/{rules_file}/enable` # 同路径下的rule.yml告警规则文件是否启用，保存true/false
    

alertmanager key 设计：

*   cluster: `/alert/cluster/{cluster_name}` # 集群名称,一个alertmanager为一个集群
    
*   config: `/alert/cluster/{cluster_name}/config` # 配置文件
    
*   tmpl: `/alert/cluster/{cluster_name}/tmpl/{tmpl_file}` # 一个tmpl\_file 对应一个告警模板文件
    
*   `/alert/cluster/{cluster_name}/tmpl/{tmpl_file}/tmpl` # 告警模板文件内容
    
*   `/alert/cluster/{cluster_name}/tmpl/{tmpl_file}/enable` # 同路径下的tmpl\_file.tmpl告警模板文件是否启用,保存true/false
    

**web ui 设计**

**prometheus 配置文件管理ui 页面**

*   对应templates/prometheus\_configs.html
    
*   点击菜单`prom配置文件管理`时展示prometheus 节点列表
    
*   节点列表有一个操作按钮`编辑`，点击`编辑`按钮，弹出prometheus 配置文件编辑弹窗
    
*   编辑框可以全屏，按`esc`推出全屏。
    
*   弹窗中展示prometheus 配置文件内容
    
*   弹窗中有一个"保存"按钮，点击"保存"按钮，保存prometheus 配置文件
    

**prometheus 告警规则管理ui 页面**

*   对应templates/prometheus\_rules.html
    
*   点击菜单`prom告警规则管理`时展示prometheus 节点列表
    
*   点击节点，展示该节点下的告警规则列表,列表为折叠卡片
    
*   告警规则列表有一个操作按钮"编辑"，点击"编辑"按钮，弹出prometheus 告警规则编辑弹窗
    
*   弹窗中展示prometheus 告警规则内容
    
*   弹窗中有一个"保存"按钮，点击"保存"按钮，保存prometheus 告警规则
    
*   告警规则列表有一个操作按钮"启用"，点击"启用"按钮，启用该告警规则
    
*   告警规则列表有一个操作按钮"禁用"，点击"禁用"按钮，禁用该告警规则
    
*   告警规则列表有一个操作按钮"删除"，点击"删除"按钮，删除该告警规则
    
*   告警规则列表有一个操作按钮"添加"，点击"添加"按钮，弹出prometheus 告警规则编辑弹窗
    

**alertmanager 配置和模板管理ui界面**

与prometheus 配置告警规则管理类似。

agent.sh 设计

**变量：**

*   PROMETHEUS\_CLUSTER\_NAME  #主机名+ip
    
*   ALERTMANAGER\_CLUSTER\_NAME #主机名+ip
    
*   CONSUL\_ADDR #consul 地址
    
*   CONSUL\_TOKEN #consul token
    
*   PROMETHEUS\_CONFIG\_PATH #prometheus 配置文件路径
    
*   PROMETHEUS\_RULES\_DIR\_PATH #prometheus 告警规则目录路径
    
*   ALERTMANAGER\_TMPL\_PATH #alertmanager 告警模板路径
    
*   ALERTMANAGER\_CONFIG\_PATH #alertmanager 配置文件路径
    
*   `ENABLE_CONSUL_REGISTRY=${ENABLE_CONSUL_REGISTRY:-"true"}` 是否开启consul 注册,主动注册prometheus/alertmanager 服务到consul
    
*   `ENABLE_UPLOAD=${ENABLE_UPLOAD:-"true"}`是否开启上传功能, 主动上传prometheus/alertmanager 告警规则、告警模板到consul。 如果`ENABLE_CONSUL_REGISTRY`为false`ENABLE_UPLOAD`也将默认关闭
    

**具体实现**

**prometheus 实现功能：**

*   使用declare -A last\_modify\_indices 在内存中建立consul key 的ModifyIndex缓存,用于判断key是否变化
    
*   分别实现watch\_config和watch\_rules两个函数监听配置和规则变化
    

**watch\_config函数**:

  1. 监听/prom/cluster/{cluster\_name}/config路径

  2. 使用curl长轮询等待变更:`curl -H "X-Consul-Token: $CONSUL_TOKEN" "$CONSUL_ADDR/v1/kv/$cluster_path/config?index=${index}&wait=5m"`

  3. 检测到变化时更新prometheus.yml配置文件并重载

**watch\_rules函数:**

  1. 监听/prom/cluster/{cluster\_name}/rules路径

  2. 使用curl长轮询递归监听:`curl -H "X-Consul-Token: $CONSUL_TOKEN" "$CONSUL_ADDR/v1/kv/$rules_path?index=${index}&wait=5m&recurse"`

  3. 根据key类型路由到不同处理函数:

     - rules/{rule\_file}/rules: 更新规则文件内容

     - rules/{rule\_file}/enable: 更新规则启用状态

     - rules 事件优先级高于enable 事件，先同步rules 配置，然后同步enable状态，防止当rules下次enable时，文件内容与consul不一致。

  4. 通过比较ModifyIndex与缓存判断是否需要处理

  5. 处理完成后更新缓存的ModifyIndex

**restart\_prometheus函数:**

  1. 使用调用prometheus 的reload api (默认开启，下面方法注释，需要取消注释)

  2. 通过kill -HUP 发送信号重启prometheus

  3. 针对docker 容器化的prometheus, 使用docker kill -s HUP <container\_id> 重启prometheus

**alertmanager 实现功能：**

*   使用declare -A last\_modify\_indices 在内存中建立consul key 的ModifyIndex缓存,用于判断key是否变化
    
*   分别实现watch\_alert\_config和watch\_tmpl两个函数监听配置和模板变化
    

**watch\_alert\_config函数:**

  1. 监听/alert/cluster/{cluster\_name}/config路径

  2. 使用curl长轮询等待变更:`curl -H "X-Consul-Token: $CONSUL_TOKEN" "$CONSUL_ADDR/v1/kv/$cluster_path/config?index=${index}&wait=5m"`

  3. 检测到变化时更新alertmanager.yml配置文件

**watch\_tmpl函数:**

  1. 监听/alert/cluster/{cluster\_name}/tmpl路径

  2. 使用curl长轮询递归监听:`curl -H "X-Consul-Token: $CONSUL_TOKEN" "$CONSUL_ADDR/v1/kv/$tmpl_path?index=${index}&wait=5m&recurse"`

  3. 根据key类型路由到不同处理函数:

     - tmpl/{tmpl\_file}/tmpl: 更新模板文件内容

     - tmpl/{tmpl\_file}/enable: 更新模板启用状态

     - tmpl 事件优先级高于enable 事件，先同步tmpl 配置，然后同步enable状态，防止当tmpl下次enable时，文件内容与consul不一致。

**restart\_alertmanager函数:**

  1. 使用调用alertmanager 的reload api (默认开启，下面方法注释，需要取消注释)

  2. 通过kill -HUP 发送信号重启alertmanager

  3. 针对docker 容器化的alertmanager, 使用docker kill -s HUP <container\_id> 重启alertmanager

**注册实现**

由变量`ENABLE_CONSUL_REGISTRY`控制是否开启。

**consul\_register函数:**

*   如果本地prometheus 配置文件存在，则注册到consul。
    
*    如果本地prometheus配置文件不存在，且consul中存在prometheus配置文件，则不创建。
    
*   如果本地prometheus配置文件不存在，且consul中不存在prometheus配置文件，则创建默认配置（prometheus 默认空配置)
    
*   如果本地alertmanager 配置文件存在且不为空，则注册到consul（如果文件为空视为文件不存在）。
    
*   如果本地alertmanager 配置文件不存在，且consul中存在alertmanager 配置文件，则不创建。
    
*   如果本地alertmanager 配置文件不存在，且consul中不存在alertmanager 配置文件，则创建默认配置
    
*   创建rules/tmpl consul key路径。
    

**consul\_upload函数:**

\- 上传prometheus 告警规则、告警模板到consul

**部署**

**管理服务端部署**

编译：
```shell
go mod tidy
go build -o consul-web main.go
```
运行：

```shell
# 参数
./consul-web -h
Usage of ./consul-web:
  -address string
    	consul address (default "192.168.48.129:8500")
  -port string
    	server port (default "8080")
  -token string
    	consul token (default "5e7f0c19-73ac-6023-c8ba-eb77988cd641")

# 示例
./consul-web -port 8080 -token 5e7f0c19-73ac-6023-c8ba-eb77988cd641 -address 192.168.48.129:8500 
```

**agent.sh 部署**

项目agent/目录下，agent.sh 脚本。

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/3277673f-d0d1-4154-b557-f3509edee739.png)

根据部署环境修改配置，或设置相应的环境变量。

**演示**

运行./consul-web 后， 运行agent.sh. 可以看见集群注册完成。

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/fdf00325-5142-4cb4-8b5d-a4365d310d1c.png)

如果，运行agent.sh 节点的prometheus 已存在配置文件，agent.sh 会将配置上传。

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/d6d6098c-4046-4c17-9d80-878776d2bc4a.png)

点击保存会修改配置，保存时会检查配置格式。

错误演示：

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/6aa00cf4-781e-4751-af68-ff31181f6ce3.png)

周期时间比超时时间小，不符合要求，报错。报错检查使用prometheus官方go 语言库检测。

重启函数，更具自身情况修改。建议使用热更新。

prometheus 重启函数名：restart\_prometheus（），这里未展示图片。

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/36dab33e-d1d9-4434-8270-35b77c1834b6.png)

alertmanager 配置与prometheus 配置相相似。

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/4bdb595c-cbf6-4f9a-8ac9-8ef7c7fe0077.png)

告警规则配置：

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/9a1f35e0-f57e-4265-aa2b-bc4964976ddd.png)

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/c8ae95d3-ad90-4dd9-97ec-ea085c279378.png)

添加规则：

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/8c3e20fd-e29d-456e-9f72-bbe5e204c98e.png)

alertmanager 告警模块与 prometheus 告警规则配置相似

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/30c040d3-1c64-4e44-8145-7980fff72616.png)

添加告警模块，有html 语法提示。 模板名称不能有后缀

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/d8c0404d-d7c1-472c-9a06-d89ab38cbecc.png)

当更新配置，告警或模板时。agent.sh会立马根据环境变量设置的路径修改对应的配置、告警、模块。

agent.sh 内部有一个缓存用于存储文件index，用于判断文件内容是否发生变化。加速处理。

![image.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/1X3lE6LK454mnJbv/img/7e3ca188-47fd-43d1-9563-63f1f0c80132.png)