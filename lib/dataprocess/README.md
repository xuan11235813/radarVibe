# 复数矩阵文件读写

```go
import "radar/lib/dataprocess"

// 保存：输入为非空、各行等长的 [][]complex128。
a := [][]complex128{{1 + 2i, 3 - 4i}, {5, -6i}}
if err := dataprocess.SaveComplexMatrix("matrix.txt", a); err != nil {
    return err
}

// 读取：b 的类型为 [][]complex128。
b, err := dataprocess.ReadComplexMatrix("matrix.txt")
if err != nil {
    return err
}
_ = b
```

文件为 UTF-8 纯文本，可用记事本打开；扩展名可用 `.txt` 或 `.csv`。无表头，每行对应矩阵的一行，元素以逗号分隔：

```text
(1+2i),(3-4i)
(5+0i),(0-6i)
```

保存使用 CRLF 换行；读取支持 CRLF、LF、UTF-8 BOM 和字段两侧空格。有限数值以可恢复 `float64` 原始精度的格式保存，支持科学计数法及 NaN/Inf（NaN 的底层载荷位不保留）。空矩阵、空行 slice、各行长度不一致、非法复数字符串会报错。读取时 CSV 空白行忽略。

输出目录必须已存在；保存不覆盖已有同名文件，写入失败会清理本次创建的文件。读取失败返回错误，不返回部分矩阵。

## 实部、虚部分别保存

```go
// 传入不带扩展名的路径前缀，生成 matrix_real.csv 和 matrix_img.csv。
err := dataprocess.SaveComplexMatrixParts("matrix", a)
if err != nil {
    return err
}

// 第一个文件为实部，第二个为虚部，返回 [][]complex128。
b, err := dataprocess.ReadComplexMatrixParts("matrix_real.csv", "matrix_img.csv")
if err != nil {
    return err
}
_ = b
```

两个 CSV 均无表头，只包含数值，每行对应矩阵的一行。以上示例矩阵的实部文件为 `1,3` / `5,0`，虚部文件为 `2,-4` / `0,-6`（`/` 表示换行）。可用记事本打开，使用 LF 换行。前缀也可以包含目录，例如 `results/matrix`，目录须已存在。

读取时检查两份文件非空、各行等长且尺寸完全一致，再按 `complex(real, img)` 合并。支持有限值原始精度、科学计数法和 NaN/Inf；不覆盖已有文件，失败时只清理本次创建的输出。

测试命令：`go test ./test -run TestComplexMatrixParts -count=1 -v`。100×100 随机矩阵、实虚部 CSV 和差异报告保存在 `test/results` 的独立目录，可手动删除。
