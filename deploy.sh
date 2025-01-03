#!/bin/bash

# 检查是否为root用户
if [ "$EUID" -ne 0 ]; then 
  echo "请使用root权限运行此脚本"
  exit 1
fi

# 创建临时目录
TMP_DIR=$(mktemp -d)
cd $TMP_DIR


DOWNLOAD_URL="https://github.com/HynoR/nft/releases/download/v1.02e/nat-go-amd64"
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
touch /etc/nat/.env

# 安装service文件
cat > /etc/systemd/system/nat-go@.service << 'EOL'
[Unit]
Description=NFTable Forward Manage Service (%i)
After=network.target

[Service]
Type=simple
User=root
EnvironmentFile=/etc/nat/.env
ExecStart=/usr/local/bin/nat-go -c /etc/nat/%i
Restart=always
RestartSec=10

# 安全相关设置
ProtectSystem=full
ProtectHome=true
PrivateTmp=true
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOL

# 重新加载systemd
systemctl daemon-reload

# 清理临时目录
rm -rf $TMP_DIR

echo "安装完成!"
echo "使用方法:"
echo "1. 将配置文件放在 /etc/nat/ 目录下"
echo "2. 启动服务: systemctl start nat-go@<配置文件名>"
echo "3. 设置开机自启: systemctl enable nat-go@<配置文件名>"