#!/bin/bash

# 变量定义
IPADDR="192.168.48.129"
PROMETHEUS_CLUSTER_NAME=${HOSTNAME}-${IPADDR}
ALERTMANAGER_CLUSTER_NAME=${HOSTNAME}-${IPADDR}
CONSUL_ADDR=${CONSUL_ADDR:-"localhost:8500"}
CONSUL_TOKEN=${CONSUL_TOKEN:-"5e7f0c19-73ac-6023-c8ba-eb77988cd641"}
PROMETHEUS_CONFIG_PATH=${PROMETHEUS_CONFIG_PATH:-"/tmp/prometheus/prometheus/etc/prometheus.yml"}
PROMETHEUS_RULES_DIR_PATH=${PROMETHEUS_RULES_DIR_PATH:-"/tmp/prometheus/prometheus/etc/rules"}
ALERTMANAGER_CONFIG_PATH=${ALERTMANAGER_CONFIG_PATH:-"/etc/alertmanager/alertmanager.yml"}
ALERTMANAGER_TMPL_PATH=${ALERTMANAGER_TMPL_PATH:-"/etc/alertmanager/tmpl"}

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
    sleep 1
    
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
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
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
            log "[Config] 无法获取consul响应或路径不存在"
            sleep 5
            continue
        fi
        
        # 获取新的ModifyIndex
        new_index=$(echo $RESPONSE | jq -r '.[0].ModifyIndex')
        
        if [ "$new_index" != "null" ] && [ "$new_index" != "$last_modify_index" ]; then
            log "[Config] 检测到变化，当前ModifyIndex: $new_index (上次: $last_modify_index)"
            
            # 解析配置
            value=$(echo $RESPONSE | jq -r '.[0].Value' | base64 -d 2>/dev/null)
            
            if [ ! -z "$value" ]; then
                handle_prometheus_config "$value"
            fi
            
            # 更新ModifyIndex
            last_modify_index=$new_index
            index=$new_index
        fi
        
        log "[Config] 监控中... (当前 Index: $index)"
        sleep 1
    done
}

# 监听规则变化
watch_rules() {
    local cluster_path="prom/cluster/$PROMETHEUS_CLUSTER_NAME"
    local rules_path="$cluster_path/rules"
    local index=0
    declare -A last_modify_indices
    
    while true; do
        # 使用长轮询等待变更
        RESPONSE=$(curl -H "X-Consul-Token: $CONSUL_TOKEN" -s \
            "$CONSUL_ADDR/v1/kv/$rules_path?index=${index}&wait=5m&recurse")
        
        # 检查响应是否为空
        if [ -z "$RESPONSE" ] || [ "$RESPONSE" = "null" ]; then
            log "[Rules] 无法获取consul响应或路径不存在"
            sleep 5
            continue
        fi
        
        # 获取新的最大 ModifyIndex
        new_index=$(echo $RESPONSE | jq -r 'map(.ModifyIndex) | max')
        
        if [ "$new_index" != "null" ] && [ "$new_index" != "$index" ]; then
            log "[Rules] 检测到变化，当前index: $new_index"
            
            # 获取所有项目并存储到数组
            readarray -t ITEMS < <(echo $RESPONSE | jq -r '.[] | @base64')
            
            # 遍历数组处理每个项目
            for item in "${ITEMS[@]}"; do
                # 解码项目
                DECODED=$(echo $item | base64 -d)
                
                # 获取key信息
                key=$(echo $DECODED | jq -r '.Key')
                value=$(echo $DECODED | jq -r '.Value' | base64 -d 2>/dev/null)
                modify_index=$(echo $DECODED | jq -r '.ModifyIndex')
                
                # 获取缓存的index
                cached_index=${last_modify_indices[$key]:-0}
                
                # 调试输出
                log "[Rules] 当前缓存状态:"
                for k in "${!last_modify_indices[@]}"; do
                    log "  $k -> ${last_modify_indices[$k]}"
                done
                
                # 检查是否需要处理这个key
                if [ -z "$cached_index" ] || [ "$modify_index" -gt "$cached_index" ]; then
                    log "[Rules] 处理变更的key: $key (ModifyIndex: $modify_index, 缓存的Index: $cached_index)"
                    
                    if [[ $key =~ $rules_path/([^/]+)/rules$ ]]; then
                        rule_file="${BASH_REMATCH[1]}"
                        if [ ! -z "$value" ]; then
                            handle_prometheus_rule "$rule_file" "$value"
                            last_modify_indices[$key]=$modify_index
                            log "[Rules] 更新缓存 $key -> $modify_index"
                        fi
                    elif [[ $key =~ $rules_path/([^/]+)/enable$ ]]; then
                        rule_file="${BASH_REMATCH[1]}"
                        if [ ! -z "$value" ]; then
                            handle_prometheus_rule_enable "$rule_file" "$value"
                            last_modify_indices[$key]=$modify_index
                            log "[Rules] 更新缓存 $key -> $modify_index"
                        fi
                    fi
                else
                    log "[Rules] 跳过未变更的key: $key (ModifyIndex: $modify_index, 缓存的Index: $cached_index)"
                fi
            done
            
            # 更新全局index
            index=$new_index
        fi
        
        log "[Rules] 监控中... (当前 Index: $index)"
        sleep 1
    done
}

# 处理prometheus配置文件变化
handle_prometheus_config() {
    local value=$1
    
    log "Updating Prometheus config file..."
    echo "$value" > $PROMETHEUS_CONFIG_PATH
    
    # 重新加载prometheus配置
    if curl -s -XPOST http://localhost:9090/-/reload > /dev/null; then
        log "Prometheus configuration reloaded successfully"
    else
        log "Failed to reload Prometheus configuration"
    fi
}

# 处理prometheus规则文件变化
handle_prometheus_rule() {
    local rule_file=$1
    local content=$2
    local rule_path="$PROMETHEUS_RULES_DIR_PATH/${rule_file}.yml"
    
    log "Updating Prometheus rule file: $rule_file"
    echo "$content" > "$rule_path"
    
    # 重新加载prometheus配置
    if curl -s -XPOST http://localhost:9090/-/reload > /dev/null; then
        log "Prometheus rules reloaded successfully"
    else
        log "Failed to reload Prometheus rules"
    fi
}

# 处理prometheus规则启用状态变化
handle_prometheus_rule_enable() {
    local rule_file=$1
    local enable=$2
    local rule_path="$PROMETHEUS_RULES_DIR_PATH/${rule_file}.yml"
    
    if [ "$enable" = "true" ]; then
        if [ -f "${rule_path}.disabled" ]; then
            mv "${rule_path}.disabled" "$rule_path"
            log "Enabled rule file: $rule_file"
        fi
    else
        if [ -f "$rule_path" ]; then
            mv "$rule_path" "${rule_path}.disabled"
            log "Disabled rule file: $rule_file"
        fi
    fi
    
    # 重新加载prometheus配置
    if curl -s -XPOST http://localhost:9090/-/reload > /dev/null; then
        log "Prometheus configuration reloaded successfully"
    else
        log "Failed to reload Prometheus configuration"
    fi
}

# 主函数
main() {
    log "Starting agent with cluster name: $PROMETHEUS_CLUSTER_NAME"
    log "Consul address: $CONSUL_ADDR"
    log "Prometheus config path: $PROMETHEUS_CONFIG_PATH"
    log "Prometheus rules directory: $PROMETHEUS_RULES_DIR_PATH"
    
    # 检查必要的命令是否存在
    for cmd in curl jq base64 pgrep pkill; do
        if ! command -v $cmd &> /dev/null; then
            log "Error: Required command '$cmd' not found"
            exit 1
        fi
    done
    
    # 启动配置监控
    watch_config &
    CONFIG_PID=$!
    CHILD_PIDS+=($CONFIG_PID)
    
    # 启动规则监控
    watch_rules &
    RULES_PID=$!
    CHILD_PIDS+=($RULES_PID)
    
    # 等待子进程，同时允许信号处理
    wait
}

# 运行主函数
main 