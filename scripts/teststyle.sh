#!/bin/bash

set -euo pipefail

PATH=$PATH:$(go env GOPATH)/bin
export PATH

exit_code=0


TARGET_VERSION="1.54.2"
install_golangci_lint=false
# 检查 golangci-lint 是否安装
if ! command -v golangci-lint &> /dev/null; then
    echo "golangci-lint 未安装"
    install_golangci_lint=true
else
    # 获取当前版本
    current_version=$(golangci-lint --version | awk '{print $4}' | sed 's/v//')

    # 比较版本
    if [ "$(printf '%s\n' "$current_version" "$TARGET_VERSION" | sort -r | head -n1)" = "$current_version" ]; then
        echo "golangci-lint 当前版本为 $current_version,符合要求"
    else
        echo "golangci-lint 当前版本为 $current_version,低于目标版本 $TARGET_VERSION"
        install_golangci_lint=true
    fi
fi

# 如果需要安装新版本
if [ "$install_golangci_lint" = true ]; then
    echo "正在从 GitHub 下载 golangci-lint $TARGET_VERSION..."

    # 下载并解压
    temp_dir=$(mktemp -d)
    curl -sSfL "https://521github.com/golangci/golangci-lint/releases/download/v$TARGET_VERSION/golangci-lint-$TARGET_VERSION-linux-amd64.tar.gz" | tar -xzf - -C "$temp_dir"

    # 移动到 PATH 中的目录
    mv "$temp_dir/golangci-lint-$TARGET_VERSION-linux-amd64/golangci-lint" "$(go env GOPATH)/bin/"

    # 清理临时目录
    rm -rf "$temp_dir"

    echo "golangci-lint $TARGET_VERSION 安装完成"
fi

#Clean cache
golangci-lint cache clean

echo
echo "==> Runninglinters <=="

# Run linters that should return errors
golangci-lint run --timeout 30m || exit_code=1

exit ${exit_code}