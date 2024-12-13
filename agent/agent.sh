#!/bin/bash

# 变量定义
IPADDR="192.168.48.129"
PROMETHEUS_CLUSTER_NAME=${HOSTNAME}-${IPADDR}
ALERTMANAGER_CLUSTER_NAME=${HOSTNAME}-${IPADDR}
CONSUL_ADDR=${CONSUL_ADDR:-"localhost:8500"}
CONSUL_TOKEN=${CONSUL_TOKEN:-"5e7f0c19-73ac-6023-c8ba-eb77988cd641"}
PROMETHEUS_CONFIG_PATH=${PROMETHEUS_CONFIG_PATH:-"/opt/prometheus/prometheus/etc/prometheus.yml"}
PROMETHEUS_RULES_DIR_PATH=${PROMETHEUS_RULES_DIR_PATH:-"/opt/prometheus/prometheus/etc/rules"}
ALERTMANAGER_CONFIG_PATH=${ALERTMANAGER_CONFIG_PATH:-"/opt/prometheus/alert/etc/alertmanager.yml"}
ALERTMANAGER_TMPL_PATH=${ALERTMANAGER_TMPL_PATH:-"/opt/prometheus/alert/etc/tmpl"}

# 是否开启consul 注册,主动注册prometheus/alertmanager 服务到consul
ENABLE_CONSUL_REGISTRY=${ENABLE_CONSUL_REGISTRY:-"true"}
# ENABLE_UPLOAD 是否开启consul 配置上传, 主动上传prometheus/alertmanager 配置、告警规则、告警模板到consul
ENABLE_UPLOAD=${ENABLE_UPLOAD:-"true"}

# 进程ID数组
declare -a CHILD_PIDS

# 清理函数
cleanup() {
    log "开始清理进程..."
    
    # 获取所有子进程的进程组ID
    local pgids=()
    for pid in "${CHILD_PIDS[@]}"; do
        if ps -p $pid > /dev/null; then
            pgid=$(ps -o pgid= -p $pid | tr -d ' ')
            pgids+=($pgid)
        fi
    done
    
    # 终止每个进程组中的所有进程
    for pgid in "${pgids[@]}"; do
        log "终止进程组 $pgid"
        kill -TERM -$pgid 2>/dev/null || true
    done
    
    # 等待一段时间
    sleep 4
    
    # 强制终止仍在运行的进程
    for pgid in "${pgids[@]}"; do
        if kill -0 -$pgid 2>/dev/null; then
            log "强制终止进程组 $pgid"
            kill -9 -$pgid 2>/dev/null || true
        fi
    done
    
    # 查找并清理遗留的curl进程
    local curl_pids=$(pgrep -f "curl.*$CONSUL_ADDR" || true)
    if [ ! -z "$curl_pids" ]; then
        log "清理遗留的curl进程: $curl_pids"
        kill -9 $curl_pids 2>/dev/null || true
    fi
    
    log "清理完成"
    exit 0
}

# 设置信号处理
trap cleanup SIGTERM SIGINT SIGHUP

# 日志函数
log() {
    echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# 监听配置变化
watch_config() {
    local cluster_path="prom/cluster/$PROMETHEUS_CLUSTER_NAME"
    local index=0
    local last_modify_index=0
    
    while true; do
        # 使用长轮询等待变更
        RESPONSE=$(curl -H "X-Consul-Token: $CONSUL_TOKEN" -s \
            "$CONSUL_ADDR/v1/kv/$cluster_path/config?index=${index}&wait=5m")
        
        # 检查响应是否为空
        if [ -z "$RESPONSE" ] || [ "$RESPONSE" = "null" ]; then
            log "[PromConfig] 无法获取consul响应或路径不存在"
            sleep 30
            continue
        fi
        
        # 获取新的ModifyIndex
        new_index=$(echo $RESPONSE | jq -r '.[0].ModifyIndex')
        
        if [ "$new_index" != "null" ] && [ "$new_index" != "$last_modify_index" ]; then
            log "[PromConfig] 检测到变化,当前ModifyIndex: $new_index (上次: $last_modify_index)"
            
            # 解析配置
            value=$(echo $RESPONSE | jq -r '.[0].Value' | base64 -d 2>/dev/null)
            
            if [ ! -z "$value" ]; then
                handle_prometheus_config "$value"
            fi
            
            # 更新ModifyIndex
            last_modify_index=$new_index
            index=$new_index
        fi
        
        log "[PromConfig] 监控中... (当前 Index: $index)"
        sleep 1
    done
}

# 监听规则变化
watch_rules() {
    local cluster_path="prom/cluster/$PROMETHEUS_CLUSTER_NAME"
    local rules_path="$cluster_path/rules"
    local index=0
    declare -A last_modify_indices
    declare -A pending_enables  # 用于存储待处理的enable事件
    
    while true; do
        # 使用长轮询等待变更
        RESPONSE=$(curl -H "X-Consul-Token: $CONSUL_TOKEN" -s \
            "$CONSUL_ADDR/v1/kv/$rules_path?index=${index}&wait=5m&recurse")
        
        # 检查响应是否为空
        if [ -z "$RESPONSE" ] || [ "$RESPONSE" = "null" ]; then
            log "[PromRules] 无法获取consul响应或路径不存在"
            sleep 30
            continue
        fi
        
        # 获取新的最大 ModifyIndex
        new_index=$(echo $RESPONSE | jq -r 'map(.ModifyIndex) | max')
        
        if [ "$new_index" != "null" ] && [ "$new_index" != "$index" ]; then
            log "[PromRules] 检测到变化,当前index: $new_index"
            
            # 获取所有项目并存储到数组
            readarray -t ITEMS < <(echo $RESPONSE | jq -r '.[] | @base64')
            
            # 调试输出
            log "[PromRules] 当前缓存状态:"
            for k in "${!last_modify_indices[@]}"; do
                log "  $k -> ${last_modify_indices[$k]}"
            done
            
            # 先处理rules事件
            for item in "${ITEMS[@]}"; do
                DECODED=$(echo $item | base64 -d)
                key=$(echo $DECODED | jq -r '.Key')
                value=$(echo $DECODED | jq -r '.Value' | base64 -d 2>/dev/null)
                modify_index=$(echo $DECODED | jq -r '.ModifyIndex')
                cached_index=${last_modify_indices[$key]:-0}
                
                # 检查是否需要处理这个key
                if [ -z "$cached_index" ] || [ "$modify_index" -gt "$cached_index" ]; then
                    log "[PromRules] 处理变更的key: $key (ModifyIndex: $modify_index, 缓存的Index: $cached_index)"
                    
                    if [[ $key =~ $rules_path/([^/]+)/rules$ ]]; then
                        rule_file="${BASH_REMATCH[1]}"
                        if [ ! -z "$value" ]; then
                            handle_prometheus_rule "$rule_file" "$value"
                            last_modify_indices[$key]=$modify_index
                            log "[PromRules] 更新缓存 $key -> $modify_index"
                        fi
                    elif [[ $key =~ $rules_path/([^/]+)/enable$ ]]; then
                        # 存储enable事件以便后续处理
                        rule_file="${BASH_REMATCH[1]}"
                        pending_enables[$rule_file]="$value"
                        last_modify_indices[$key]=$modify_index
                    fi
                else
                    log "[PromRules] 跳过未变更的key: $key (ModifyIndex: $modify_index, 缓存的Index: $cached_index)"
                fi
            done
            
            # 后处理所有pending的enable事件
            for rule_file in "${!pending_enables[@]}"; do
                enable_value="${pending_enables[$rule_file]}"
                log "[PromRules] 处理规则启用状态: $rule_file -> $enable_value"
                handle_prometheus_rule_enable "$rule_file" "$enable_value"
            done
            
            # 清空pending_enables数组
            pending_enables=()
            
            # 更新全局index
            index=$new_index
        fi
        
        log "[PromRules] 监控中... (当前 Index: $index)"
        sleep 1
    done
}


# 重启prometheus,可以更具prometheus部署方式,使用不同的命令
restart_prometheus() {
    # 当prometheus 使用--web.enable-lifecycle参数开启热加载时,使用reload。
    if curl -s -XPOST http://localhost:9090/-/reload > /dev/null; then
        log "[Prometheus] Prometheus configuration reloaded successfully"
    else
        log "[Prometheus] Failed to reload Prometheus configuration"
    fi

    # 当prometheus本地部署并使用--config.file参数指定配置文件时,使用kill -HUP pid
    
    # if [ -f "$PROMETHEUS_CONFIG_PATH" ]; then
    #     pid=$(pgrep -f "prometheus --config.file=$PROMETHEUS_CONFIG_PATH")
    #     if [ ! -z "$pid" ]; then
    #         kill -HUP $pid
    #         log "Prometheus configuration reloaded successfully"
    #     else
    #         log "Failed to reload Prometheus configuration"
    #     fi
    # fi

    # 当prometheus 使用docker部署时,使用通过docker kill -s HUP 重启prometheus
    # docker kill -s HUP $(docker ps | grep prom/prometheus | awk '{print $1}')

}

# 处理prometheus配置文件变化
handle_prometheus_config() {
    local value=$1
    
    log "[PromConfig] Updating Prometheus config file..."
    echo "$value" > $PROMETHEUS_CONFIG_PATH
    
    # 检查prometheus 配置文件是否存在
    if [ ! -f "$PROMETHEUS_CONFIG_PATH" ]; then
        log "\033[31m[PromConfig] Error: Prometheus config file does not exist\033[0m"
        return
    fi
    
    restart_prometheus
}

# 处理prometheus规则文件变化
handle_prometheus_rule() {
    local rule_file=$1
    local content=$2
    local rule_path="$PROMETHEUS_RULES_DIR_PATH/${rule_file}.yml"

    # 检查是否有disabled文件,如果有则删除
    if [ -f "${rule_path}.disabled" ]; then
        rm "${rule_path}.disabled"
    fi
    
    log "[PromRules] Updating Prometheus rule file: $rule_file"
    echo "$content" > "$rule_path"
    
    # 检查prometheus 规则文件是否存在
    if [ ! -f "$rule_path" ]; then
        log "\033[31m[PromRules] Error: create rule file failed\033[0m"
        return
    fi
    
    restart_prometheus
}

# 处理prometheus规则启用状态变化
handle_prometheus_rule_enable() {
    local rule_file=$1
    local enable=$2
    local rule_path="$PROMETHEUS_RULES_DIR_PATH/${rule_file}.yml"

    if [ "$enable" = "true" ]; then
        if [ -f "${rule_path}.disabled" ]; then
            mv "${rule_path}.disabled" "$rule_path"
            log "[PromRules] Enabled rule file: $rule_file"
        fi
    else
        if [ -f "$rule_path" ]; then
            mv "$rule_path" "${rule_path}.disabled"
            log "[PromRules] Disabled rule file: $rule_file"
        fi
    fi
    
    restart_prometheus
}

# 监听alertmanager配置变化
watch_alert_config() {
    local cluster_path="alert/cluster/$ALERTMANAGER_CLUSTER_NAME"
    local index=0
    local last_modify_index=0
    
    while true; do
        # 使用长轮询等待变更
        RESPONSE=$(curl -H "X-Consul-Token: $CONSUL_TOKEN" -s \
            "$CONSUL_ADDR/v1/kv/$cluster_path/config?index=${index}&wait=5m")
        
        # 检查响应是否为空
        if [ -z "$RESPONSE" ] || [ "$RESPONSE" = "null" ]; then
            log "[AlertConfig] 无法获取consul响应或路径不存在"
            sleep 30
            continue
        fi
        
        # 获取新的ModifyIndex
        new_index=$(echo $RESPONSE | jq -r '.[0].ModifyIndex')
        
        if [ "$new_index" != "null" ] && [ "$new_index" != "$last_modify_index" ]; then
            log "[AlertConfig] 检测到变化,当前ModifyIndex: $new_index (上次: $last_modify_index)"
            
            # 解析配置
            value=$(echo $RESPONSE | jq -r '.[0].Value' | base64 -d 2>/dev/null)
            
            if [ ! -z "$value" ]; then
                handle_alertmanager_config "$value"
            fi
            
            # 更新ModifyIndex
            last_modify_index=$new_index
            index=$new_index
        fi
        
        log "[AlertConfig] 监控中... (当前 Index: $index)"
        sleep 1
    done
}

# 监听alertmanager模板变化
watch_tmpl() {
    local cluster_path="alert/cluster/$ALERTMANAGER_CLUSTER_NAME"
    local tmpl_path="$cluster_path/tmpl"
    local index=0
    declare -A last_modify_indices
    declare -A pending_enables  # 用于存储待处理的enable事件
    
    while true; do
        # 使用长轮询等待变更
        RESPONSE=$(curl -H "X-Consul-Token: $CONSUL_TOKEN" -s \
            "$CONSUL_ADDR/v1/kv/$tmpl_path?index=${index}&wait=5m&recurse")
        
        # 检查响应是否为空
        if [ -z "$RESPONSE" ] || [ "$RESPONSE" = "null" ]; then
            log "[AlertTmpl] 无法获取consul响应或路径不存在"
            sleep 30
            continue
        fi
        
        # 获取新的最大 ModifyIndex
        new_index=$(echo $RESPONSE | jq -r 'map(.ModifyIndex) | max')
        
        if [ "$new_index" != "null" ] && [ "$new_index" != "$index" ]; then
            log "[AlertTmpl] 检测到变化,当前index: $new_index"
            
            # 获取所有项目并存储到数组
            readarray -t ITEMS < <(echo $RESPONSE | jq -r '.[] | @base64')
            
            # 先处理tmpl事件
            for item in "${ITEMS[@]}"; do
                DECODED=$(echo $item | base64 -d)
                key=$(echo $DECODED | jq -r '.Key')
                value=$(echo $DECODED | jq -r '.Value' | base64 -d 2>/dev/null)
                modify_index=$(echo $DECODED | jq -r '.ModifyIndex')
                cached_index=${last_modify_indices[$key]:-0}

                # 检查是否需要处理这个key
                if [ -z "$cached_index" ] || [ "$modify_index" -gt "$cached_index" ]; then
                    if [[ $key =~ $tmpl_path/([^/]+)/tmpl$ ]]; then
                        tmpl_file="${BASH_REMATCH[1]}"
                        if [ ! -z "$value" ]; then
                            log "[AlertTmpl] 处理模板文件: $tmpl_file"
                            handle_alertmanager_tmpl "$tmpl_file" "$value"
                            last_modify_indices[$key]=$modify_index
                        fi
                    elif [[ $key =~ $tmpl_path/([^/]+)/enable$ ]]; then
                        # 存储enable事件以便后续处理
                        tmpl_file="${BASH_REMATCH[1]}"
                        pending_enables[$tmpl_file]="$value"
                        last_modify_indices[$key]=$modify_index
                    fi
                fi
            done
            
            # 后处理所有pending的enable事件
            for tmpl_file in "${!pending_enables[@]}"; do
                enable_value="${pending_enables[$tmpl_file]}"
                log "[AlertTmpl] 处理模板启用状态: $tmpl_file -> $enable_value"
                handle_alertmanager_tmpl_enable "$tmpl_file" "$enable_value"
            done
            
            # 清空pending_enables数组
            pending_enables=()
            
            # 更新全局index
            index=$new_index
        fi
        
        log "[AlertTmpl] 监控中... (当前 Index: $index)"
        sleep 1
    done
}

# 重启alertmanager,可以更具alertmanager部署方式,使用不同的命令
restart_alertmanager() {

    # 当alertmanager 热加载时,使用reload。
    if curl -s -XPOST http://localhost:9093/-/reload > /dev/null; then
        log "[AlertManager] Alertmanager configuration reloaded successfully"
    else
        log "[AlertManager] Failed to reload Alertmanager configuration"
    fi

    # 当alertmanager本地部署并使用--config.file参数指定配置文件时,使用kill -HUP pid
    # if [ -f "$ALERTMANAGER_CONFIG_PATH" ]; then
    #     pid=$(pgrep -f "alertmanager --config.file=$ALERTMANAGER_CONFIG_PATH")
    #     if [ ! -z "$pid" ]; then
    #         kill -HUP $pid
    #         log "Alertmanager configuration reloaded successfully"
    #     else
    #         log "Failed to reload Alertmanager configuration"
    #     fi
    # fi
    
    # 当alertmanager 使用docker部署时,使用通过docker kill -s HUP 重启alertmanager
    # docker kill -s HUP $(docker ps | grep prom/alertmanager | awk '{print $1}')
}

# 处理alertmanager配置文件变化
handle_alertmanager_config() {
    local value=$1
    
    log "[AlertConfig] Updating Alertmanager config file..."
    echo "$value" > $ALERTMANAGER_CONFIG_PATH

    restart_alertmanager
}

# 处理alertmanager模板文件变化
handle_alertmanager_tmpl() {
    local tmpl_file=$1
    local content=$2
    local tmpl_path="$ALERTMANAGER_TMPL_PATH/${tmpl_file}.tmpl"
    
    # 检查是否有disabled文件,如果有则删除
    if [ -f "${tmpl_path}.disabled" ]; then
        rm "${tmpl_path}.disabled"
    fi

    log "[AlertTmpl] Updating Alertmanager template file: $tmpl_file"
    echo "$content" > "$tmpl_path"

    # 检查alertmanager 模板文件是否存在
    if [ ! -f "$tmpl_path" ]; then
        log "\033[31m[AlertTmpl] Error: create template file failed\033[0m"
        return
    fi

    # 重新加载alertmanager配置
    restart_alertmanager
}

# 处理alertmanager模板启用状态变化
handle_alertmanager_tmpl_enable() {
    local tmpl_file=$1
    local enable=$2
    local tmpl_path="$ALERTMANAGER_TMPL_PATH/${tmpl_file}.tmpl"

    if [ "$enable" = "true" ]; then
        if [ -f "${tmpl_path}.disabled" ]; then
            mv "${tmpl_path}.disabled" "$tmpl_path"
            log "[AlertTmpl] Enabled template file: $tmpl_file"
        fi
    else
        if [ -f "$tmpl_path" ]; then
            mv "$tmpl_path" "${tmpl_path}.disabled"
            log "[AlertTmpl] Disabled template file: $tmpl_file"
        fi
    fi
    
    restart_alertmanager
}

# Consul服务注册
consul_register() {
    log "[Registry] 开始注册集群到Consul..."
    
    # 注册Prometheus集群
    local prom_key="prom/cluster/$PROMETHEUS_CLUSTER_NAME/config"
    if [ -f "$PROMETHEUS_CONFIG_PATH" ]; then
        local prom_config=$(cat "$PROMETHEUS_CONFIG_PATH")
        if curl -H "X-Consul-Token: $CONSUL_TOKEN" -X PUT \
            --data-binary "$prom_config" \
            "$CONSUL_ADDR/v1/kv/$prom_key" > /dev/null 2>&1; then
            log "[Registry] Prometheus集群注册成功"
        else
            log "\033[31m[Registry] Prometheus集群注册失败\033[0m"
        fi
    else
        # 如果配置文件不存在，查看consul是否存在prometheus集群配置,如果存在不创建,不存在则创建空配置
        # 检查consul是否存在prometheus集群配置
        response=$(curl -s -H "X-Consul-Token: $CONSUL_TOKEN" -X GET "$CONSUL_ADDR/v1/kv/$prom_key")
        if [ ! -z "$response" ]; then
            log "[Registry] Prometheus集群配置已存在,跳过注册"
        else
            # 不存在则创建空配置
            if curl -H "X-Consul-Token: $CONSUL_TOKEN" -X PUT \
                --data-binary "" \
                "$CONSUL_ADDR/v1/kv/$prom_key" > /dev/null 2>&1; then
                log "[Registry] Prometheus集群(空配置)注册成功"
            else
                log "\033[31m[Registry] Prometheus集群注册失败\033[0m"
            fi
        fi
    fi
    # 创建prometheus 告警规则目录
    if ! curl -H "X-Consul-Token: $CONSUL_TOKEN" -X PUT \
        "$CONSUL_ADDR/v1/kv/prom/cluster/$PROMETHEUS_CLUSTER_NAME/rules/" > /dev/null 2>&1; then
        log "\033[31m[Registry] 创建prometheus 告警规则目录失败\033[0m"
    fi
    
    # 注册Alertmanager集群
    local alert_key="alert/cluster/$ALERTMANAGER_CLUSTER_NAME/config"
    # 检查alertmanager 配置文件是否存在且不为空
    if [ -f "$ALERTMANAGER_CONFIG_PATH" ] && [ -s "$ALERTMANAGER_CONFIG_PATH" ]; then
        local alert_config=$(cat "$ALERTMANAGER_CONFIG_PATH")
        if curl -H "X-Consul-Token: $CONSUL_TOKEN" -X PUT \
            --data-binary "$alert_config" \
            "$CONSUL_ADDR/v1/kv/$alert_key" > /dev/null 2>&1; then
            log "[Registry] Alertmanager集群注册成功"
        else
            log "\033[31m[Registry] Alertmanager集群注册失败\033[0m"
        fi
    else
        # 如果配置文件不存在，查看consul是否存在alertmanager集群配置,如果存在不创建,不存在则创建示例配置
        # 检查consul是否存在alertmanager集群配置
        response=$(curl -s -H "X-Consul-Token: $CONSUL_TOKEN" -X GET "$CONSUL_ADDR/v1/kv/$alert_key")
        if [ ! -z "$response" ]; then
            log "[Registry] Alertmanager集群配置已存在,跳过注册"
        else
            # 不存在则创建示例配置
            # 示例配置
            example_config="global:
  smtp_require_tls: false
route:
  group_by: [alertname]
  group_wait: 5s
  group_interval: 1m
  repeat_interval: 5s
  receiver: dingding
receivers:
- name: dingding
  webhook_configs:
    - url: http://192.168.48.129:8060/dingtalk/webhook1/send
      send_resolved: true
templates:
 - tmpl/*.tmpl"
            if curl -H "X-Consul-Token: $CONSUL_TOKEN" -X PUT \
                --data-binary "$example_config" \
                "$CONSUL_ADDR/v1/kv/$alert_key" > /dev/null 2>&1; then
                log "[Registry] Alertmanager集群(空配置)注册成功"
            else
                log "\033[31m[Registry] Alertmanager集群注册失败\033[0m"
            fi
        fi
    fi
    # 创建alertmanager 告警模板目录
    if ! curl -H "X-Consul-Token: $CONSUL_TOKEN" -X PUT \
        "$CONSUL_ADDR/v1/kv/alert/cluster/$ALERTMANAGER_CLUSTER_NAME/tmpl/" > /dev/null 2>&1; then
        log "\033[31m[Registry] 创建alertmanager 告警模板目录失败\033[0m"
    fi
}

# Consul配置上传
consul_upload() {
    log "[Upload] 开始上传prometheus 告警规则、告警模板到Consul..."
    
    # 上传Prometheus规则
    if [ -d "$PROMETHEUS_RULES_DIR_PATH" ]; then
        for rule_file in "$PROMETHEUS_RULES_DIR_PATH"/*.yml; do
            if [ -f "$rule_file" ]; then
                local rule_name=$(basename "$rule_file" .yml)
                local rule_content=$(cat "$rule_file")
                local rule_path="prom/cluster/$PROMETHEUS_CLUSTER_NAME/rules/$rule_name/rules"
                local enable_path="prom/cluster/$PROMETHEUS_CLUSTER_NAME/rules/$rule_name/enable"
                
                # 上传规则内容
                if curl -H "X-Consul-Token: $CONSUL_TOKEN" -X PUT \
                    --data-binary "$rule_content" \
                    "$CONSUL_ADDR/v1/kv/$rule_path" > /dev/null 2>&1; then
                    log "[Upload] 规则文件 $rule_name 上传成功"
                else
                    log "\033[31m[Upload] 规则文件 $rule_name 上传失败\033[0m"
                fi
                
                # 设置规则启用状态
                if curl -H "X-Consul-Token: $CONSUL_TOKEN" -X PUT \
                    --data-binary "true" \
                    "$CONSUL_ADDR/v1/kv/$enable_path" > /dev/null 2>&1; then
                    log "[Upload] 规则 $rule_name 启用状态设置成功"
                else
                    log "\033[31m[Upload] 规则 $rule_name 启用状态设置失败\033[0m"
                fi
            fi
        done
    fi
    
    log "[Upload] 开始上传alertmanager 告警模板到Consul..."
    
    # 上传Alertmanager模板
    if [ -d "$ALERTMANAGER_TMPL_PATH" ]; then
        for tmpl_file in "$ALERTMANAGER_TMPL_PATH"/*.tmpl; do
            if [ -f "$tmpl_file" ]; then
                local tmpl_name=$(basename "$tmpl_file" .tmpl)
                local tmpl_content=$(cat "$tmpl_file")
                local tmpl_path="alert/cluster/$ALERTMANAGER_CLUSTER_NAME/tmpl/$tmpl_name/tmpl"
                local enable_path="alert/cluster/$ALERTMANAGER_CLUSTER_NAME/tmpl/$tmpl_name/enable"
                
                # 上传模板内容
                if curl -H "X-Consul-Token: $CONSUL_TOKEN" -X PUT \
                    --data-binary "$tmpl_content" \
                    "$CONSUL_ADDR/v1/kv/$tmpl_path" > /dev/null 2>&1; then
                    log "[Upload] 模板文件 $tmpl_name 上传成功"
                else
                    log "\033[31m[Upload] 模板文件 $tmpl_name 上传失败\033[0m"
                fi
                
                # 设置模板启用状态
                if curl -H "X-Consul-Token: $CONSUL_TOKEN" -X PUT \
                    --data-binary "true" \
                    "$CONSUL_ADDR/v1/kv/$enable_path" > /dev/null 2>&1; then
                    log "[Upload] 模板 $tmpl_name 启用状态设置成功"
                else
                    log "\033[31m[Upload] 模板 $tmpl_name 启用状态设置失败\033[0m"
                fi
            fi
        done
    fi
}

# 注册和上传
register_and_upload() {
    # 注册prometheus/alertmanager 服务到consul
    if [ "$ENABLE_CONSUL_REGISTRY" = "true" ]; then
        consul_register

        # 上传prometheus/alertmanager 告警规则、告警模板到consul
        if [ "$ENABLE_UPLOAD" = "true" ]; then
            consul_upload
        fi
    fi
}

# 主函数
main() {
    log "\033[32m[Agent] Starting agent with cluster names:\033[0m"
    log "\033[32m[Agent] Prometheus: $PROMETHEUS_CLUSTER_NAME\033[0m"
    log "\033[32m[Agent] Alertmanager: $ALERTMANAGER_CLUSTER_NAME\033[0m"
    log "\033[32m[Agent] Consul address: $CONSUL_ADDR\033[0m"
    log "\033[32m[Agent] Prometheus config path: $PROMETHEUS_CONFIG_PATH\033[0m"
    log "\033[32m[Agent] Prometheus rules directory: $PROMETHEUS_RULES_DIR_PATH\033[0m"
    log "\033[32m[Agent] Alertmanager config path: $ALERTMANAGER_CONFIG_PATH\033[0m"
    log "\033[32m[Agent] Alertmanager templates directory: $ALERTMANAGER_TMPL_PATH\033[0m"
    
    # 检查必要的命令是否存在，如果不存在尝试使用yum安装
    for cmd in curl jq base64 pgrep pkill; do
        if ! command -v $cmd &> /dev/null; then
            log "\033[31m[Agent] Error: Required command '$cmd' not found\033[0m"
            log "\033[32m[Agent] Trying to install $cmd...\033[0m"
            if ! yum install -y $cmd; then
                log "\033[31m[Agent] Failed to install $cmd\033[0m"
                exit 1
            fi
        fi
    done

    # 注册和上传
    register_and_upload

    # 启动prometheus配置监控
    watch_config &
    CONFIG_PID=$!
    CHILD_PIDS+=($CONFIG_PID)
    
    # 启动prometheus规则监控
    watch_rules &
    RULES_PID=$!
    CHILD_PIDS+=($RULES_PID)
    
    # 启动alertmanager配置监控
    watch_alert_config &
    ALERT_CONFIG_PID=$!
    CHILD_PIDS+=($ALERT_CONFIG_PID)
    
    # 启动alertmanager模板监控
    watch_tmpl &
    TMPL_PID=$!
    CHILD_PIDS+=($TMPL_PID)
    
    # 等待子进程，同时允许信号处理
    wait
}

# 运行主函数
main 