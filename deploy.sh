#!/bin/bash

# 检查是否为root用户
if [ "$EUID" -ne 0 ]; then 
  echo "请使用root权限运行此脚本"
  exit 1
fi

# 检查nftables是否已安装
if command -v nft > /dev/null 2>&1; then
    echo "nftables 已安装"
else
    echo "nftables 未安装，请先安装nftables"
    exit 1
fi

# 检查IP转发是否已启用
echo "检查IP转发设置..."
IP_FORWARD=$(cat /proc/sys/net/ipv4/ip_forward)
if [ "$IP_FORWARD" -eq 1 ]; then
    echo "IP转发已启用"
else
    echo "IP转发未启用，现在启用..."

    # 临时启用IP转发
    echo 1 > /proc/sys/net/ipv4/ip_forward

    # 检查是否已存在于sysctl配置中
    if grep -q "net.ipv4.ip_forward" /etc/sysctl.conf; then
        # 如果存在但值不为1，则修改值
        sed -i 's/net.ipv4.ip_forward *= *0/net.ipv4.ip_forward = 1/' /etc/sysctl.conf
    else
        # 如果不存在，则添加配置
        echo "net.ipv4.ip_forward = 1" >> /etc/sysctl.conf
    fi

    # 应用更改
    sysctl -p

    echo "IP转发已永久启用"
fi

# 检测系统架构
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "不支持的架构: $ARCH"
        exit 1
        ;;
esac
echo "检测到系统架构: $ARCH"

# 获取最新版本
echo "正在获取最新版本信息..."
LATEST_RELEASE=$(curl -s https://api.github.com/repos/HynoR/nft/releases/latest)
if [ $? -ne 0 ]; then
    echo "获取最新版本信息失败!"
    exit 1
fi

VERSION=$(echo $LATEST_RELEASE | grep -o '"tag_name": "[^"]*' | sed 's/"tag_name": "//')
if [ -z "$VERSION" ]; then
    echo "无法解析版本信息!"
    exit 1
fi
echo "最新版本: $VERSION"

# 创建临时目录
TMP_DIR=$(mktemp -d)
cd $TMP_DIR

# 构建下载URL
DOWNLOAD_URL="https://github.com/HynoR/nft/releases/download/${VERSION}/nat-go-linux-${ARCH}"
echo "下载URL: $DOWNLOAD_URL"

# 下载二进制文件
echo "正在下载nat-go..."
curl -L -o nat-go $DOWNLOAD_URL

# 检查下载是否成功
if [ $? -ne 0 ]; then
    echo "下载失败!"
    exit 1
fi

# 设置执行权限并移动到目标目录
chmod +x nat-go
mv nat-go /usr/local/bin/

# 创建配置目录
mkdir -p /etc/nat
mkdir -p /etc/nat/cfg
touch /etc/nat/.env
touch /etc/nat/cfg/default.conf

# 如果default.conf不存在或为空，则创建示例配置
if [ ! -s /etc/nat/cfg/default.conf ]; then
    cat > /etc/nat/cfg/default.conf << 'EOL'
# 本机:1234 -> 1.1.1.1:39000
SINGLE,1234,39000,1.1.1.1
EOL
fi

# 安装service文件
cat > /etc/systemd/system/nat-go.service << 'EOL'
[Unit]
Description=NFTable Forward Manage Service
After=network.target

[Service]
Type=simple
User=root
EnvironmentFile=/etc/nat/.env
ExecStart=/usr/local/bin/nat-go -c /etc/nat/cfg
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOL

# 重新加载systemd
systemctl daemon-reload

# 清理临时目录
rm -rf $TMP_DIR

echo "安装完成!"
echo "使用方法:"
echo "1. 将配置文件放在 /etc/nat/cfg 目录下"
echo "2. 启动服务: systemctl start nat-go"
echo "3. 设置开机自启: systemctl enable nat-go"
echo "4. 检查服务状态: systemctl status nat-go"
echo "5. 查看配置示例: cat /etc/nat/cfg/default.conf"