# 雷达测试工具链

- `exampleCode/`：已验证的参考实现。原代码依赖原应用的上下文，通过独立 `go.mod` 排除在工具链构建之外，不修改其解析逻辑。
- `lib/`：公共协议定义、组帧、UDP 接收和 CSV 导出。
- `test/`：协议、导出及本地 UDP 集成测试。
- `toolbox/`：基于公共库实现的工具入口。

## receiveData

需要 Go 1.25 或更新版本。仓库根目录执行：

```sh
go run ./toolbox/receiveData
```

默认监听所有本地 IPv4 网卡的 UDP **1995** 端口，最多等待 30 秒。收到首个通过同步信号校验的完整帧后，在当前工作目录写入 `radar_时间戳.csv`，然后退出。可以直接构建 Windows 工具：

```sh
go build -o toolbox/receiveData/receiveData.exe ./toolbox/receiveData
./toolbox/receiveData/receiveData.exe
```

自定义输出、等待时间及雷达来源：

```sh
go run ./toolbox/receiveData -output frame.csv -timeout 60s -source 192.168.1.100
go run ./toolbox/receiveData -timeout 0
```

`-timeout 0` 表示持续等待，Ctrl+C 取消；`-listen` 可指定本地监听地址，默认 `0.0.0.0:1995`。输出路径的父目录必须存在，不覆盖已有文件。失败时退出码为 1。

电脑与雷达应处于可接收广播的同一局域网，Windows 防火墙需要允许该程序接收 UDP 1995。接收广播不需要主动向雷达发送数据。存在多台雷达时建议用 `-source` 选择来源；否则锁定首个合法帧起点的来源 IP 和 UDP 源端口，直至本次接收结束。

### 协议与完整帧

遵循 `exampleCode/merger.go`：

- UDP 数据报长 8196 字节：4 字节小端字节偏移量 + 8192 字节载荷。
- 从偏移量 0 开始，连续接收 1024 包，组成 8 MiB 原始帧。
- 非法包、丢包、重复包或乱序导致当前帧作废；遇到偏移量 0 立即重新开始。没有帧编号，不能可靠地跨帧补包，因此采用参考实现的顺序组帧方式。
- 8 通道交错排列，每个值是小端 16 位，右移 2 位得到 ADC 值（0–16383），与参考代码一致，不做符号扩展。
- 每周期 2125 个采样点，检查通道 6、7 的 bit 1 同步信号，检查范围与容差严格沿用参考实现。校验不通过则继续等待下一帧。

### CSV 格式

```csv
period_index,sample_in_period,ch6,ch7
```

每行包含周期编号 `period_index`、周期内索引 `sample_in_period` 和同一采样时刻的 **ch6、ch7** 两个 ADC 值，按采集顺序排列。两种索引均从 0 开始：周期编号为采样序号 / 2125（整除），周期内索引为采样序号 % 2125；最后一行索引为 `246,1537`。不包含全帧采样索引列或其他通道。每帧 **524288 行数据**，另加一行表头；包含 246 个完整周期和最后一个周期的 1538 个真实采样点，不导出参考数组末尾的补零区域。原始 UDP 帧仍按 8 通道完整接收，导出时只解码 ch6、ch7。

## dataProcess

读取 receiveData CSV，依次完成内存加载、ch6/ch7 周期矩阵转换、完整周期筛选与采样截取、无表头矩阵导出：

```sh
go build -o toolbox/dataProcess/dataProcess.exe ./toolbox/dataProcess
./toolbox/dataProcess/dataProcess.exe filename.csv splitChannel
./toolbox/dataProcess/dataProcess.exe filename.csv splitChannel 100 500
```

默认保留完整周期的全部 2125 个采样点；可选起止索引从 0 开始且包含两端。输出到当前工作目录的 `datach6.csv`、`datach7.csv`，每行一个周期，不覆盖已有文件。标准单帧默认输出两个 246 × 2125 矩阵，末尾不完整周期丢弃。四个公共函数位于 `lib/dataprocess`，详细说明见 [dataProcess 使用说明](toolbox/dataProcess/使用说明.md)。

`dataProcess` 还支持按周期 FFT：

```sh
./toolbox/dataProcess/dataProcess.exe filename.csv fftChannel
./toolbox/dataProcess/dataProcess.exe filename.csv fftChannel 100 500 false
```

默认截取完整周期并逐行去直流，最后一个参数 `false` 可关闭去直流。处理顺序为 ReadCSV → ToMatrices → SelectSamples → FFTRealRows。两个通道分别生成 `fftch6.txt` / `fftch7.txt` 完整复数文件，以及同前缀的 `_real.csv`、`_img.csv`，共六个无表头文件，保存在当前目录。具体参数见 [dataProcess 使用说明](toolbox/dataProcess/使用说明.md)。

## 验证

```sh
go test ./...
go vet ./...
```

测试包含模拟协议数据、丢包后重新同步、无效长度/偏移、SYNC 校验、CSV 全部采样行、文件覆盖保护和回环 UDP 收帧/超时。真实雷达及局域网广播接收需在设备环境中验证。
