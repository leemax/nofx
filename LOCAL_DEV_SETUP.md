# 本地开发环境搭建与运行指南

本文档用于指导开发者在本地（以macOS为例）不通过Docker，直接运行和调试`nofx`项目。

## 1. 环境要求

- **macOS**
- **Homebrew**: macOS的包管理器。如果未安装，请访问 [https://brew.sh/](https://brew.sh/) 获取安装指令。
- **Node.js**: v18或更高版本。

## 2. 安装依赖

请在您的终端中，按顺序执行以下命令。

### 第一步：安装Go语言环境

```bash
brew install go
```

安装完成后，执行 `go version` 检查是否成功。您应该能看到类似 `go version go1.2x.x darwin/amd64` 的输出。

### 第二步：安装TA-Lib库

本项目使用TA-Lib进行技术指标计算，这是Go后端的一个关键依赖。

```bash
brew install ta-lib
```

### 第三步：安装前端依赖 (Node.js)

进入 `web` 目录，使用 `npm` 安装所有前端依赖包。

```bash
cd web
npm install
```

安装完成后，请回到项目根目录。

```bash
cd ..
```

### 第四步：安装/同步后端依赖 (Go)

在项目根目录下，执行以下命令来下载并同步Go模块所需的全部依赖。这一步会根据 `go.mod` 文件下载我们新增的 `go-sqlite3` 和 `gin-contrib/cors` 等库。

```bash
go mod tidy
```

或者，您也可以使用 `go get` 来确保所有依赖都已更新：

```bash
go get ./...
```

## 3. 运行项目

项目需要同时运行后端和前端两个服务。

### 第一步：运行后端服务

确保您位于项目根目录 (`/Users/lichungang/nofx/`)。

```bash
go run main.go
```

启动后，您将看到数据库初始化、AI决策周期等日志。后端API服务会运行在 `http://localhost:8080`。

### 第二步：运行前端服务

打开**另一个新**的终端窗口。

进入 `web` 目录。

```bash
cd web
npm run dev
```

启动后，Vite开发服务器会运行在 `http://localhost:3000`。

### 第三步：访问和验证

在您的浏览器中打开 `http://localhost:3000`。

您应该能看到一个功能完整、数据实时刷新的前端界面。所有对代码的修改，只需重启对应的服务即可立即生效，大大提升了开发和调试效率。
