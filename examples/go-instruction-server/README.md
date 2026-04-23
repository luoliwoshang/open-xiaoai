# Go Instruction Server

一个最小可用的 Go Server 示例，用来接收小爱音箱 Rust Client 发来的 `instruction` 事件，并打印原生小爱语音识别后的最终文本。

当前版本默认还会做一件事：

- 收到最终 ASR 文本后，立即通过 `run_shell` RPC 执行 `/etc/init.d/mico_aivs_lab restart`
- 也就是默认会在 ASR 之后打断原生小爱的后续流程

这个示例只做一件事：

- 你先说“`小爱同学`”
- 音箱原生系统完成唤醒和 ASR
- Rust Client 监听 `/tmp/mico_aivs_lab/instruction.log`
- Go Server 收到 `SpeechRecognizer.RecognizeResult` 的最终文本并打印

它不会：

- 接管原生小爱的对话链路
- 处理原始麦克风音频流
- 给音箱回传 TTS 或音频

## 结构

当前工程只保留三层：

- `main.go`
  负责启动参数和装配默认 `onASR` 处理逻辑
- `internal/server`
  负责 WebSocket 接入、连接会话、RPC 调用、`abort` 能力
- `internal/speaker`
  负责复用音箱本地 TTS 播放单段文字和模拟流式文字播放
- `internal/instruction`
  负责从 `instruction` 事件里提取最终 ASR 文本

这样拆分后：

- 协议和连接细节不会继续污染入口文件
- `AbortXiaoAI()` 变成了独立能力
- `PlayText()` 也变成了独立能力
- `PlayTextStream()` 用来顺序播放多段文字 chunk
- 你后面如果要接 HTTP、规则引擎或自己的 Agent，只需要替换 `main.go` 里的 `onASR` 处理函数

## 运行

```sh
cd examples/go-instruction-server
go run .
```

默认监听 `:4399`，并且默认开启 `abort-after-asr`。

也可以自定义：

```sh
go run . -addr :4399 -debug
```

如果你只想拿 ASR，不想打断原生小爱，显式关掉即可：

```sh
go run . -abort-after-asr=false
```

## 当前示例行为

当前 `main.go` 里带了一个最小示例规则：

- 当识别结果等于 `测试播放文字`
- 先打断原生小爱
- 等待 2 秒
- 然后调用 `internal/speaker` 里的 `PlayText()` 播放：
  `你好，很高兴认识你！`

- 当识别结果等于 `测试长段播放文字`
- 先打断原生小爱
- 等待 2 秒
- 然后调用 `internal/speaker` 里的 `PlayTextStream()`，按多段 chunk 顺序播放一段长回复

## 让音箱连过来

先在小爱音箱上把 Rust Client 指向你的电脑：

```sh
mkdir -p /data/open-xiaoai
echo 'ws://你的电脑局域网IP:4399' > /data/open-xiaoai/server.txt
curl -sSfL https://gitee.com/idootop/artifacts/releases/download/open-xiaoai-client/init.sh | sh
```

例如：

```sh
echo 'ws://192.168.31.227:4399' > /data/open-xiaoai/server.txt
```

## 预期输出

当你对音箱说：

```txt
小爱同学
今天天气怎么样
```

如果原生小爱已经把最终识别结果写进 `instruction.log`，你会在 Go Server 里看到类似输出：

```txt
2026/04/23 20:00:00 client connected: 192.168.31.100:54321
2026/04/23 20:00:08 xiaoai command: 今天天气怎么样
2026/04/23 20:00:08 xiaoai aborted after final ASR
```

## 后续扩展

你后面如果要接自己的逻辑，直接从这里往下改就行：

- 在 `onASR(text, abort bool)` 里把文本转发给你自己的业务处理函数
- 如果你想远程控制音箱，可以继续复用 `call()` 和 `abortXiaoAI()` 这套 RPC 调用
- 如果你想直接吃原始麦克风流，再补 `BinaryMessage` 的 `Stream` 解析
