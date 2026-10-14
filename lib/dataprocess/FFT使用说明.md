# FFT 函数

四个函数位于 `lib/dataprocess/fft.go`，统一使用 `removeDC bool` 控制变换前是否减去当前向量的均值。

| 函数 | 输入 | 输出 | 变换方向 |
| --- | --- | --- | --- |
| `FFTReal(input, removeDC)` | `[]float64` | `[]complex128, error` | 一维实数向量 |
| `FFTRealRows(input, removeDC)` | `[][]float64` | `[][]complex128, error` | 每行独立，仅第二维 |
| `FFTComplex(input, removeDC)` | `[]complex128` | `[]complex128, error` | 一维复数向量 |
| `FFTComplexRows(input, removeDC)` | `[][]complex128` | `[][]complex128, error` | 每行独立，仅第二维 |

```go
// a 是 [period][sample] 的实数矩阵，例如 SelectSamples 返回的 Ch6。
spectrum, err := dataprocess.FFTRealRows(a, true)
if err != nil {
    return err
}
// 保存结果供查看或后续算法读取。
if err := dataprocess.SaveComplexMatrixParts("ch6_fft", spectrum); err != nil {
    return err
}
```

## 频谱约定

- 使用 Gonum v0.16.0 的 `dsp/fourier`，支持包括 2125 在内的非 2 次幂长度，不自动补零。
- 正向变换：`X[k] = sum(x[n] * exp(-2πi*k*n/N))`，不归一化，不加窗，不执行 fftshift。
- 实数和复数输入均返回 **N 个复数频点的完整双边频谱**。实数变换利用共轭对称恢复负频部分，不进行单边幅值倍增。
- 零频（DC）在索引 0。频率间隔为 `Fs/N`；负频率位于数组后半部分。
- `removeDC=true` 时，在 FFT 前减去均值。复数向量同时去掉实部和虚部均值；矩阵逐行减去各行自己的均值。去直流后零频点可能残留浮点舍入误差。
- 两个矩阵函数都只沿第二维变换，不沿第一维、不执行完整二维 FFT。例如输入 `[246][2125]`，输出仍为 `[246][2125]`。
- 不修改输入；结果具有独立存储。每次函数调用使用独立 FFT 实例，矩阵的各行复用本次调用的 FFT 实例。
- 空输入、零列、行长不一致、NaN/Inf 输入报错；计算溢出产生非有限输出时也返回错误。

## 验证

```sh
go test ./test -run TestFFT -count=1 -v
```

测试覆盖独立 DFT 对照、奇偶长度和非 2 次幂长度、实数/复数已知频率信号、逐行变换、去直流开关、输入不变及错误输入。2125 点雷达长度测试将去直流前后的频谱和报告保留在 `test/results/TestFFTRadarLength-*`，可手动删除。

依赖文档：https://pkg.go.dev/gonum.org/v1/gonum@v0.16.0/dsp/fourier
