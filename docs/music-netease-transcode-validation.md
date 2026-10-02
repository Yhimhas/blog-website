# 网易云音源与 FFmpeg 转码验收

> 历史验收记录，结果仅适用于当时版本和环境。最新能力与后续验证入口见 [项目当前状态](project-status.md)；下文“未实现／未提交／连接失败”等表述不代表当前状态。

日期：2026-09-29。网易云 resolver 与 FFmpeg 已接入统一播放器；真实 FFmpeg 及公开网易云音源管道验证通过。服务器仍无法连接，已登记歌单的元数据接口返回 20001，因此不能宣称该歌单已同步、所有曲目可播或已上线。

## 实现范围

- 新增网易云 AudioResolver，输入仅限库内 netease ID，使用官方 `music.163.com/song/media/outer/url` 外链；不加载账号 Cookie，不使用付费解锁或第三方音源。
- 解析阶段与实际流传输均验证官方重定向域名。官方 HTTP CDN Location 在建立连接前升级为 HTTPS；不发送明文媒体请求。沿用 Go 的 URL、DNS、公网实际地址和重定向检查。
- 拒绝 JSON、HTML、播放列表等非音频。HTTP 404/410 返回 AUDIO_SOURCE_UNAVAILABLE；429 返回限流；风控/登录/临时拒绝返回 UPSTREAM_UNAVAILABLE，不把整张歌单永久标为不可用。
- Bilibili / 网易云共用播放会话。网易云曲目可以发起播放，失败显示明确提示，原站入口保留。旧 unavailable 状态允许用户重新尝试，避免历史检查结果永久禁止重试。
- MP3 直接转发；Bilibili 已确认 AAC 的 M4A 直接转发。FLAC、Ogg、WebM、WAV、ADTS AAC，以及需兼容的 MP4 音轨，经 FFmpeg 输出 128 kbps / 44.1 kHz / 双声道 MP3。网易云非 MP3 音频也进入转码。
- FFmpeg 只读取经 Go 校验后的 stdin 管道；固定 demuxer 与 `protocol_whitelist=pipe`，不传 URL、文件名、任意参数，不启用 cache/HLS 临时文件。只映射音轨，禁用视频/字幕/数据输出。
- 转码并发最多 2，使用有界 stderr、单线程编码、512 KiB 探测和单次内存分配上限。流首批有效输出受 15 秒预算限制；停止、断连、超时和退出均取消进程树并释放配额。保留原有流并发 4、会话上限 128。
- 获取有效转码输出后才发 audio/mpeg；响应开始前失败返回安全错误 JSON，开始后仅终止音频并更新状态，不把错误文本追加到音频。
- 普通需要反向 seek 的 MP4 不能保证从管道读取；失败会明确返回转码错误，不落盘，不启用 FFmpeg 自行请求远程 URL。`-max_alloc` 是单次分配上限，不等同于完整进程 RSS 上限。

## 配置

```dotenv
MUSIC_PLAYBACK_ENABLED=true
MUSIC_YTDLP=yt-dlp
MUSIC_FFMPEG=ffmpeg
MUSIC_FORCE_TRANSCODE=false
```

FFmpeg 应使用提供 libmp3lame 的构建。Windows 本次使用已安装的 8.0.1 essentials build；未新安装工具。Linux 恢复网络后需确认 FFmpeg 安装、版本和编码器，并在同一部署环境复验。

`MUSIC_FORCE_TRANSCODE=true` 会把原本可直接转发的音轨也转为 MP3，适用于浏览器兼容性排查，会占用转码配额。默认 false，不支持的来源仍明确报错。

官方参数依据：[FFmpeg 协议与 pipe 限制](https://ffmpeg.org/ffmpeg-protocols.html)、[FFmpeg 命令参数](https://ffmpeg.org/ffmpeg.html)。

## 已执行测试

- Windows `go test ./... -count=1`、`go vet ./...`、`go build ./...` 通过；Linux/amd64 交叉编译通过（不等同于 Linux 运行验收）。
- 真实 FFmpeg：内存生成 1 秒 440 Hz WAV，转成 16,718 字节 MP3，再解码得到 16,720 字节 PCM，验证不是静音或伪造数据。
- FLAC、Ogg/Vorbis、WebM/Opus、ADTS AAC、fragmented MP4/AAC 均以真实编码器在内存生成，再由实际转码路径成功转为 MP3。
- HTTP 媒体 handler → FFmpeg → MP3 完整链路通过；两路占满后第三路拒绝，取消后配额回收；非法输入、任意 demuxer、失败进程都不被报告为播放成功。响应开始后的上游故障用 ErrAbortHandler 中止连接，测试确认客户端收到截断错误而非正常结束，不追加 JSON。
- 网易云用可控 HTTP transport 验证官方 302、HTTPS 升级、MP3 响应、404/429、-460 JSON、HTML、HLS 及外域/私网重定向拒绝；未以此冒充真实上游可播。
- 前端 21 项音乐测试通过，含网易云进入统一播放器与拒绝后的原站入口；type-check 和生产构建通过。
- 真实公开探测：歌单 595975585 返回 code=20001。早期缺少网页请求头时，公开样本 33894312 与无效 ID 1 的外链返回 -460 JSON；加入已验证的 User-Agent 和官方 Referer 后，样本 33894312 返回 m10.music.126.net 的有效 audio/mpeg。该请求头已加入 resolver 和重定向测试，不含账号 Cookie。
- 2026-09-29 真实音源串联：官方外链返回 HTTP CDN Location，在连接前升级为 HTTPS；有界读取 196,608 字节，FFmpeg 从 stdin 读取并生成两秒、32,600 字节 MP3，退出码 0，输出 MPEG 帧头 fffb9064。原始音频和输出都只在内存中；未将样本导入数据库，也未修改用户歌单。
- 上述真实上游验证使用本机 Python HTTP 客户端与实际 FFmpeg；Go resolver、会话和媒体 handler 使用可控上游单独验证。本机 Go 安全 HTTP client 不接受 Fake-IP DNS 的 198.18.x.x 地址，未放宽检查。因此这些结果不等同于目标服务器 → Go → 浏览器的真实播放验收。
- SSH 100.96.172.0 仍超时，本次没有部署、没有更改远程进程或数据库。本次默认 DB 集成测试因未配置连接而跳过，不将之前版本的数据库验收当作当前真实部署验收。

## 保留产物

未删除任何文件。新增 Go/TypeScript 测试文件是正式回归代码。所有本次测试音频只存在于内存与 OS pipe，没有生成媒体文件或下载缓存。

本次可清理残留仅为原有 `frontend/dist/` 中新增的构建哈希资源、前端构建缓存以及工作区父目录 `.tools/go-cache/` 编译缓存，按用户要求保留。前次服务器测试目录、schema 与进程记录仍见 [此前验收记录](music-playback-validation.md)，本次未操作。
