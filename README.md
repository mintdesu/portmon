# portmon

基于 iptables 的端口流量监控工具。适用于多人共用的按流量计费 Linux 服务器，按端口统计每个人的服务分别用了多少流量。

## 功能

- 按端口统计入站/出站流量
- 支持单端口和端口范围（如 `10002-10010`）
- 数据按天存储为 CSV 文件，可用 Excel 打开
- 命令行查看报表：按端口、按用户、按月、按日期范围
- 支持 SIGHUP 热重载配置，无需重启
- 附带 systemd 服务文件

## 安装

### 前置依赖

```bash
# Debian / Ubuntu
sudo apt update && sudo apt install -y iptables

# CentOS / RHEL
sudo yum install -y iptables
```

### 下载二进制

从 [Releases](https://github.com/mintdesu/portmon/releases) 下载对应架构的文件：

```bash
# x86_64 服务器
wget https://github.com/mintdesu/portmon/releases/latest/download/portmon-linux-amd64
chmod +x portmon-linux-amd64
sudo mv portmon-linux-amd64 /usr/local/bin/portmon

# ARM64 服务器
wget https://github.com/mintdesu/portmon/releases/latest/download/portmon-linux-arm64
chmod +x portmon-linux-arm64
sudo mv portmon-linux-arm64 /usr/local/bin/portmon
```

### 写配置文件

```bash
sudo mkdir -p /etc/portmon
sudo nano /etc/portmon/config.yaml
```

内容如下，按你的实际情况改端口和名字：

```yaml
interface: eth0              # 网卡名，用 ip addr 查看
ports:
  - port: 10000
    name: "web-api"
    owner: "alice"
  - port: 10001
    name: "game-server"
    owner: "bob"
  - port: 10002-10010        # 支持端口范围
    name: "proxy"
    owner: "charlie"
interval: 60                 # 采集间隔（秒）
data_dir: "/var/lib/portmon" # 数据存储目录
log_retention_days: 90       # CSV 数据保留天数，0 表示不自动清理
cleanup_on_exit: false       # 退出时是否清理 iptables 规则
iptables_path: "iptables"   # iptables 路径，一般不用改
```

### 设置开机自启

```bash
# 创建服务文件
sudo tee /etc/systemd/system/portmon.service > /dev/null << 'EOF'
[Unit]
Description=portmon per-port network traffic collector
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/portmon daemon
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

# 启动并设为开机自启
sudo systemctl daemon-reload
sudo systemctl enable --now portmon

# 检查是否跑起来了
sudo systemctl status portmon
```

## 使用

```bash
# 看今天各端口用了多少流量
portmon report today

# 看指定日期范围
portmon report --from 2024-01-01 --to 2024-01-31

# 按用户汇总
portmon report --by-owner

# 按月汇总本月；log_retention_days 需覆盖本月起始日期
portmon report --monthly

# 看当前 iptables 实时计数；首次使用前先执行 portmon iptables setup
portmon status
```

输出示例：

```
Port        Name          Owner     Inbound      Outbound     Total
10000       web-api       alice     12.5 GB      3.2 GB       15.7 GB
10001       game-server   bob       45.1 GB      8.7 GB       53.8 GB
10002-10010 proxy         charlie   2.3 GB       0.5 GB       2.8 GB
Total                               59.9 GB      12.4 GB      72.3 GB
```

## 管理 iptables 规则

程序使用独立的 iptables 链（`PORTMON_IN` / `PORTMON_OUT`），不会影响现有防火墙规则。

```bash
# 手动创建规则
portmon iptables setup

# 清理所有 portmon 规则
portmon iptables cleanup
```

## 常用运维操作

```bash
# 改了配置后重载，不用重启
sudo systemctl reload portmon

# 查看日志
sudo journalctl -u portmon -f

# 数据文件在这里，每天一个 CSV
ls /var/lib/portmon/
```

## 从源码编译

需要 Go 1.22+：

```bash
git clone https://github.com/mintdesu/portmon.git
cd portmon
go build -trimpath -ldflags="-s -w" -o portmon .
```

## 注意事项

- 需要 root 权限运行（iptables 操作需要）
- CSV 数据默认保留 90 天，可用 `log_retention_days` 调整
- 查询日期早于保留范围时，`report` 会报错提示调大 `log_retention_days` 或设为 `0`
- 流量单位按十进制换算（1 GB = 1,000,000,000 字节），和运营商计费口径一致
- 已在 Debian 12 / iptables v1.8.9 (nf_tables) 上测试通过，其他系统建议先跑 `portmon iptables setup`，再跑 `portmon status` 确认兼容性

## 许可证

MIT
