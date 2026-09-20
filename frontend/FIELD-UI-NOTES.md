# FIELD UI 前端接入说明

更新日期：2026-09-16。本文描述当前 0.1.1 接入状态。

## 运行与构建

- 在 frontend 中运行 `pnpm dev --host 127.0.0.1`。
- 类型检查与生产构建：`pnpm run build -- --emptyOutDir=false`。保留该参数，避免构建清空已有 dist 文件。
- 项目声明 Node `^22.18.0 || >=24.12.0`；本次环境为 Node 20.19.5、pnpm 10.29.3，有 engine 警告，但 vue-tsc 和 Vite 构建通过。
- `@field-lab/vue` 来自 `vendor/field-lab-vue-0.1.1.tgz`，package.json、pnpm-lock.yaml 和已安装版本一致。没有使用个人 skill 目录的绝对路径作为依赖。
- tarball SHA-256：`8a5a96036657a159cba74e6ad8a8d52860e50e64df1fb789a3721538d346b0e8`，与 skill 来源记录相符。

## 页面与组件

- `/`：首页；`/blog`：博客；`/music`：音乐；`/blog/:id`：独立文章详情；未知路径和文章均有缺失状态。
- `src/views/` 实现各页面，`src/router.ts` 管理路由，`src/App.vue` 保留共享布局、导航与动态开关。
- 文章链接支持直接访问、刷新、新标签页和浏览器前进后退。搜索和分类保存在 URL query；详情页不再使用 FieldDialog。
- 当前使用 FieldProvider、TechTabs、HoloCard、MotionReveal、TechButton。页面导航使用 RouterLink，复用 FIELD rail 样式和 0.1.1 RailNav 的交互逻辑；对应 MIT 许可保存在 `vendor/FIELD-UI-LICENSE`。
- 保留用户的 Yhimhas 站名、首页文案和已注释的主视觉、关于区域；OrbitField 目前不渲染。
- FieldProvider 基础 duration 为 580ms、accent 为 #d4ef37；音乐 HoloCard tilt 为 3。
- `src/content.ts` 保存 4 篇示例文章及 3 个音乐探索方向。音乐入口是 Bilibili 关键词搜索，收藏保存在 localStorage；尚未接入后端、真实歌单或站内音频播放。
- history 路由部署需要服务器将不存在的静态路径回退到 index.html，例如 Nginx `try_files $uri $uri/ /index.html;`。本次未修改部署配置。

## 0.1.1 升级

- 页眉辅助标记、文章日期、唱片分类等小字采用新版 `--f-detail` 圆润 sans-serif 字体栈；站名和较大的技术图形保留 monospace。
- 桌面鼠标进入侧栏自动展开，移出后收起；键盘焦点在导航内时保持展开，焦点离开后收起。
- 窄屏使用菜单按钮；选择页面后收起。Escape 关闭导航后将焦点移回菜单按钮，避免焦点留在隐藏链接或掉回 body。
- 桌面预留左侧 76px，760px 及以下预留顶部 60px。
- 系统 reduced motion 由 FIELD UI 响应；动态开关通过 paused 停用非必要动效。

## 本次实际验证

- `pnpm run build -- --emptyOutDir=false`：vue-tsc 和 Vite build 通过。
- 1280px 桌面：侧栏指针进入/移出、键盘进入/离开；计算样式确认小字使用 `--f-detail`。
- 390px：菜单按钮、路由切换后收起、Escape 焦点恢复、音乐页布局和收藏刷新保留。
- 320px：博客列表及长标题详情页、直接刷新。1280px、390px、320px 所检查页面均无横向溢出。
- 博客分类、Tabs 方向键、搜索空态及重置通过；文章渲染在独立路由中，无 dialog。
- 动态暂停后 Provider 标记为 reduced，HoloCard transition 降为 0.01ms；收藏仍可操作。验证后恢复收藏和动态默认状态。
- 新建验证标签页采集的浏览器 error/warn 日志为空。服务启动时另一个已打开页面曾报告 Vue DevTools 重复实例提示；它属于开发工具，未因此改动项目插件配置。
- 未实测系统 reduced-motion 设置切换、真实触摸设备、Safari/Firefox 或低端设备性能。窄视口检查不等同于真实触屏测试。

## 保留文件与旧产物

没有删除文件；未新建试验脚本或截图文件。原有 introduction.vue、base.css、counter.ts 及用户的既有修改保留。

当前构建使用 `dist/index.html`、`dist/assets/index-BHuHskYp.js`、`dist/assets/index-DprIRI5I.css`。其余已过期的构建资源逐项保留如下：

- `dist/assets/index--t1HpIJ3.js`
- `dist/assets/index-B9iZ-i2R.js`
- `dist/assets/index-Brn8-A9O.css`
- `dist/assets/index-BUXeZY0y.css`
- `dist/assets/index-BwUYlonw.js`
- `dist/assets/index-C97Rg89l.css`
- `dist/assets/index-D9XLWFb3.js`
- `dist/assets/index-DD1pgZFN.js`
- `dist/assets/index-dMq3UgD0.js`
- `dist/assets/index-DY0bXBN6.css`
- `dist/assets/index-Y0gABWhq.css`

`vendor/field-lab-vue-0.1.0.tgz` 也保留，当前依赖已不引用它。旧资源不影响当前构建入口；构建工具正常更新 node_modules 内的缓存。

## 2026-09-16 Logo 与 FIELD UI 0.1.2

- 组件包升级为 vendor/field-lab-vue-0.1.2.tgz，pnpm 依赖和 lockfile 已同步。
- 用户图片原样复制为 public/yhimhas-logo.jpg，应用于侧栏、页头、页脚及 favicon/apple-touch-icon；没有裁剪或修改原图。
- BlogView 搜索容器采用新版 f-input-shell：聚焦外框 2px，输入框内部 outline 为 none。
- 类型检查与构建通过；浏览器验证 3 处 Logo 均成功加载，原图 300×300；搜索 Vue 返回 2 篇文章；390px 窄屏无横向溢出。真实触摸、Safari/Firefox 未实测。
- 原 favicon.ico 与 0.1.0、0.1.1 包均保留，不再作为当前图标/依赖使用。没有生成试验脚本或截图文件。
- 此次有效构建资源为 index-CFwDTRDR.css、index-12FKwp8b.js。此前的 index-DprIRI5I.css、index-B9iZ-i2R.js 已被替代并保留；更早的旧资源见以上记录。

## 2026-09-20 音乐页参考设计替换

- /music 使用用户提供的原始 JPG 作为全屏背景，新增透明导航、搜索、玻璃播放器、音乐库和收藏面板。App.vue 仅在音乐路由隐藏原侧栏、页头、页脚；首页及博客保持独立布局。
- 新增 src/assets/music.css、src/components/MusicIcon.vue、public/music-rain-background.jpg，重做 src/views/MusicView.vue。
- 参考图里的「雨落之后 / 青空」作为视觉示例；没有对应音源时显示 00:00 和添加提示，不模拟播放。频谱为播放状态驱动的装饰动画，并非实时频率分析。
- 本地文件使用 object URL 与原生 audio 播放，不上传。支持播放/暂停、拖动进度、音量、静音、上一首/下一首、随机播放及收藏；不足两首可播放音乐时上一首/下一首禁用。
- 原音乐探索条目保留在音乐库，收藏继续使用原 localStorage key。本地音频刷新或离开音乐页后需重新选择；离页停止播放并释放 object URL。
- 验证：类型检查、Vite 构建通过；桌面视觉与 390px 窄屏检查通过；收藏、搜索空态、博客返回与独立布局通过；导入 8 秒静音 WAV，确认媒体时长 8 秒、播放结束、再次播放/暂停及键盘定位到 0.1 秒。浏览器 error 日志为空。
- 未实测真实触屏、跨浏览器音频格式兼容性与多曲目自动播放。背景直接使用原图，没有生成额外雨滴滤镜。
- 当前构建资源为 dist/assets/index-Bxn3JebB.css、dist/assets/index-BNV9YBWF.js；旧构建资源继续保留，未删除文件。
- 本次唯一测试残留：C:/Users/lin/AppData/Local/Temp/yhimhas-player-check-20260920.wav（8 秒静音测试音频，无需用于站点，按用户要求保留）。未保存截图或其他试验脚本。

## 2026-09-20 平台歌单接入（取代本地音频功能）

- 已移除 MusicView 的本地文件输入、File/Object URL、audio 和本地播放控制逻辑。原静音测试文件仍保留，没有删除文件。
- 用户提供的 Bilibili 收藏夹 3549762089「哈基米」已分页同步 53/53 条视频元数据，包含标题、UP 主、BVID 和官方播放器链接。数据保存在 src/data/bilibili-playlist.json，不保存 Cookie 或媒体文件。
- 运行 `pnpm run sync:music` 更新歌单快照；成功后重新构建发布。不是实时或定时同步。请求出错时保留旧快照。所用公开网页元数据接口不是稳定性承诺的开放 API，后续可能需要维护。
- src/musicSources.ts 管理平台配置；网易云未收到实际歌单链接，尚未接入真实数据。
- 页面支持视频列表、搜索、收藏、当前曲目标记、手动上一首/下一首。iframe 使用官方 Bilibili 播放器；声音、进度、分 P、暂停由平台控件提供，不支持自定义音频控制或自动跨视频连续播放。
- 本次验证：类型检查与构建通过；53 条列表、搜索「配方」返回 1 条、下一首切换至 BV1ggKP6yEkb、文件输入数量为 0。桌面视觉检查通过。
- 官方播放器单独打开可识别第一首视频并显示 01:20 时长及控制栏；当前应用内 iframe 仍为空白，尚未确认站内媒体实际播放成功。未绕过平台限制。保留原平台入口。此次视口工具未实际切至 390px，故不声称完成本轮手机验证。
- 新增有用文件：scripts/sync-bilibili.mjs、src/musicSources.ts、src/data/bilibili-playlist.json。没有新增试验文件。
- 最新有效构建：index-DHrfmSoD.css、index-DD9Nv3tC.js；本次被替代但保留的产物：index-xKpHSeSn.css、index-U5Cv1BcR.js、index-CXy-7CW8.js。更早产物仍保留。
- 官方播放器参数说明：https://player.bilibili.com/
