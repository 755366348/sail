# Sail 数据透视

面向 Apple 商品入库数据的桌面数据透视工具。导入 Excel 后可选择行字段、列字段和汇总方式，确认预览后将报表自动保存到桌面。

生成的报表包含：

- `数据透视`：用户配置后的透视表、生成时间和绿色总计行。
- `数据概览`：按设备大类汇总的数量及总数量。

支持传统 `.xls`、Excel 2003 XML（扩展名为 `.xls`）和 `.xlsx` 文件。当前测试数据会默认按 `UPC` 分组、对 `数量` 求和；设备概览根据商品名称归并为手机、平板、电脑等大类。

## 开发

```powershell
wails dev
```

## 构建

在 Windows 上运行：

```powershell
wails build
```

Windows 产物位于 `build/bin/Sail.exe` 或 `build/bin/sail.exe`（取决于 Wails 配置）。

在 macOS Apple Silicon 设备上使用相同源码运行：

```bash
wails build
```

该命令会生成原生 `darwin/arm64` 应用。Wails 的原生 WebView 依赖需要在目标系统构建，因此 macOS 产物应在 Mac 上打包，Windows 产物应在 Windows 上打包。
