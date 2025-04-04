#!/bin/bash

# 保存初始网络接口状态
old_state=$(ip a)

while true; do
    # 获取当前网络接口状态
    new_state=$(ip a)
    
    # 比较状态差异
    if ! diff <(echo "$old_state") <(echo "$new_state") >/dev/null; then
        echo -e "\n[$(date +'%Y-%m-%d %H:%M:%S.%3N')] 检测到网络接口变化："
        diff --color=auto <(echo "$old_state") <(echo "$new_state") | grep -v '^[0-9]' | sed '/^---/d;/^+++/d'
        old_state="$new_state"
    fi
    
    # 等待0.1秒
    sleep 0.1
done
