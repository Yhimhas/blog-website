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

## 2026-09-21 路由分层过渡与后端协作

- 当前协作入口：../docs/backend-development-guide.md。助手实现前端，用户学习 Go 并实现后端；未创建后端服务或更改现有音乐播放方式。
- App.vue 用 RouterView slot + Vue Transition 包裹完整页面布局，mode=out-in，key 使用 path；查询参数变化不重新挂载页面，音乐页的独立布局与内容一起过渡。
- 延续 EntryIntro 的斜向 polygon 裁切、22px 内容上浮：退出 220ms，入场 460ms，总计约 680ms；内容分组延迟 40/80ms。未新增动效依赖。
- pageTransition.ts 在目标 DOM 入场后释放路由滚动；忽略已过期导航的滚动结果。历史滚动使用 instant，避免和全局 smooth 叠加。动画结束后仅当焦点仍在 body 时聚焦 main，避免抢走正在输入的搜索框。
- ArticleView 改用路由 props，退出时保留旧文章内容，避免 route.params 更新导致暂时显示 404。补齐初始首页标题的默认值和 meta 监听。
- 暂停动态与 prefers-reduced-motion 将路由及分组动画缩短为 1ms；保留正常导航。
- 验证：最终 pnpm run build -- --emptyOutDir=false 通过（vue-tsc + Vite）；浏览器验证首页→博客→文章→博客→音乐、搜索筛选、文章退出内容、暂停动态、历史返回恢复非零滚动位置、连续前进/后退最终 URL 与内容一致。1440px 桌面和 390px 窄屏检查；窄屏无横向溢出；浏览器 error/warn 日志为空。
- 未实测系统 reduced-motion 切换、真实触屏、Safari/Firefox 或设备帧率；系统偏好规则只做代码检查。纯音频和 Go API 尚未实现。
- 构建环境提示 Node v20.19.5 低于 package.json 声明的 ^22.18.0 || >=24.12.0；本次构建成功，未替用户升级环境。
- 当前构建资源：dist/assets/index-B82AM5Le.css、dist/assets/index-Dju8uhgG.js。
- 本轮中间构建残留：dist/assets/index-qsMTnISV.js（已被最终 JS 替代，保留未删）。没有新增截图、测试脚本或音频文件；原有旧 dist 资源未清理。类型检查缓存为正常构建产物。

## 2026-09-21 音乐页视频参考风格与每日推荐

- 改造范围仅 MusicView.vue 及新 music-atlus.css；首页、博客和全局路由动效未修改。视频 3:43 蓝色菜单与 6:00 附近粉色菜单为视觉参考，采用斜切导航、黑白色块、超大装饰字、蓝/青与粉色强调及方向相关的分层过渡。
- 默认进入发现：每日推荐 3 首；音乐库有现有 53 条真实 Bilibili 元数据，支持标题/作者搜索及来源过滤；收藏延续 lin-music-favorites。暂无网易云数据时显示明确空态。
- 背景默认：发现=已有 intro-p3re.jpg，音乐库=已有 music-rain-background.jpg，收藏=新增原创 SVG music-favorites-background.svg。三个栏目可各自在三个预设背景间切换，偏好保存在 yhimhas:music-backgrounds:v1；没有上传新图片或新增外部素材依赖。
- musicDaily.ts 按上海日期和曲目 ID 稳定 hash 排序、去重并取 3 条；不修改源数据，源列表重排不改变结果。30 秒间隔与页面恢复可见时检查日期。曲库变动可能改变当天结果；未实现后端近 7 天排除与持久化推荐，详见 backend-development-guide.md 的音乐页补充。
- 播放器没有封面，保留标题、作者、出处、上一首/下一首、收藏及官方 iframe 入口；切换栏目与背景不重建 iframe。明确显示“待播放/平台播放器已打开”，不模拟播放成功或进度。打开后锁定当前曲目，跨日推荐更新不替换正在打开的曲目。
- 动效：背景交叉淡入+轻微缩放，内容先退出150ms再入场400ms，列表小范围错峰入场；键盘方向键/Home/End 切换 Tab。局部暂停和系统 reduced-motion 均有降级规则。
- 验证：music-daily.test.mjs 3 项测试通过（上海跨日、同日稳定/去重/不修改输入、空与小曲库）；最终类型检查、4 篇内容检查、Vite build、git diff --check 通过。运行新测试：node --experimental-strip-types --test scripts/music-daily.test.mjs（使用支持该参数的 Node）。
- 浏览器：1440px 桌面、390px/320px 无横向溢出；3 首推荐刷新稳定；收藏保存和取消；栏目背景与手动背景偏好刷新保留；音乐库53条、网易云空态、搜索空态；方向键/Home 连续切换收敛，焦点保留选中 Tab；暂停动态可导航；打开播放器后切换栏目，iframe 数量仍为1且src不变；返回博客显示原有文章页；浏览器 error/warn 记录为空。测试添加的收藏已取消，测试背景已恢复默认。
- 未实测真实触屏、Safari/Firefox、系统 reduced-motion 开关或平台音频实际出声；iframe 本轮只验证入口/保留/关闭，不声称纯音频已实现。
- 构建的 pnpm 启动器仍报告 Node v20.19.5 引擎警告；已将子进程 PATH 指向 bundled Node，内容检查与构建实际通过。未修改用户全局 Node 配置。
- 当前构建资源：dist/assets/index-KiZR27wB.css、dist/assets/index-czEVEAIt.js。
- 本轮中间构建残留：dist/assets/index-B4zj7kXq.css、dist/assets/index-DCBVaZj7.js（被最终构建替代，保留未删）。旧 music.css 不再由音乐页导入，保留作风格参考；未删除任何文件。没有保存新增截图、音频、视频或临时测试脚本；music-daily.test.mjs 为保留的正式回归测试。

## 2026-09-21 替换为用户提供的三张背景

- 原样复制 Downloads/1789983476206.jpeg、1789983478768.jpeg、1789983473823.jpeg 到 public/music-golden-rain.jpeg、music-blue-rain.jpeg、music-pink-rain.jpeg，未修改或删除原图。
- 默认映射：发现→金色雨幕，音乐库→蓝调雨夜，收藏→绯色心动。手动切换和已有栏目偏好继续有效。
- 每张图片使用独立 object-position 保持人物裁切位置，遮罩改为中性深色以保留原图暖色与粉色。
- 验证：vue-tsc --build、git diff --check 通过；浏览器三张图片自然宽度分别为3305/3845/3259，均加载成功，栏目切换后的图片路径正确。
- 未生成试验产物，未运行额外打包。旧 music-favorites-background.svg 已不再引用，按要求保留；intro-p3re.jpg 仍用于首页，music-rain-background.jpg 仍为历史样式参考素材。

## 2026-09-21 IgnoredOne 图标与平台切换

- 按用户指定 https://www.ignoredone.space/index.php/iconasset-2/，选用 iconpack02 的播放0031、上一首0026、下一首0030和装饰星形0007；原始4个PNG保存在public/icons/ignoredone，合计7380字节。MusicIcon用CSS alpha mask继承当前颜色；搜索、收藏、关闭和返回保留原图标，不将通用图标作为平台Logo。来源与用途见同目录README.md；未声明第三方素材为MIT或项目原创。
- 平台筛选加入三等分滑动选中条（320ms），内容out-in退出120ms/进入240ms，淡入淡出配合小幅纵向移动。浏览区域保持桌面350px、手机360px，解决Bilibili列表变为空网易云时面板骤然收缩。新平台入场重置列表滚动，搜索输入不触发整组重播，播放器不在该过渡内。
- 延续motionOff、父级暂停及prefers-reduced-motion降级；不加入定时器切换或外部动效依赖。
- 验证：vue-tsc --build、4篇文章检查、Vite build --emptyOutDir=false、git diff --check通过。浏览器观察到music-source-enter-active，切换前后列表高度350px一致；Bilibili→网易云→Bilibili连续操作最终53条且选中Bilibili；暂停状态可切换；390px内容宽375px无横向溢出；图标正常显示，浏览器error/warn为空。系统reduced-motion开关和真实触屏未实测。
- 本轮构建资源：dist/assets/index-CRaMHAsF.css、dist/assets/index-BvcFwliV.js。未删除旧文件、未保存临时截图、未下载未使用图标。临时启动的5174预览已停止，沿用现有5173预览。

## 2026-09-21 完成中断任务

- 网易云歌单已按用户确认登记为「我喜欢的音乐」，入口更新为用户提供的移动歌单 URL；由于公开接口返回未授权，保持 `syncStatus: pending`，页面显示待同步说明、官方歌单播放器入口和原页面链接，不伪造曲目数据。
- 新增 `SiteIcon.vue`，将 IgnoredOne 图标库的声波、层叠和方向箭头用于全站导航、音乐导航、首页入口、文章/404 返回和回到顶部；原有音乐控制图标继续使用 `MusicIcon.vue`。素材来源和用途记录在 `frontend/public/icons/ignoredone/README.md`。
- MusicView 的待同步歌单播放器标题、来源链接和 embed URL 已支持仅有 playlist 而没有 track 的情况。
- 验证：`pnpm run build -- --emptyOutDir=false` 通过（vue-tsc、文章检查、Vite）；`music-daily.test.mjs` 3 项通过；`git diff --check` 通过；浏览器初始页面确认音乐页每日推荐 3 首及播放器入口；后续点击遇到浏览工具 shadow root 错误，未完成本轮歌单点击、图标视觉和窄屏验证。
- pnpm 启动器仍报告 Node 20.19.5 版本警告，构建子进程使用 bundled Node；本轮构建成功，未修改全局 Node。
- 本轮构建产物 `dist/assets/index-ZN8enZ7-.css`、`dist/assets/index-DdMXUjtT.js` 保留；旧构建资源均未删除。`icon-review.png` 是本轮图标核对拼图，仅用于检查，按要求保留。没有删除任何文件。
`n- 本轮被最终构建替代、保留未删的中间产物：dist/assets/index-POt5G4lD.css、dist/assets/index-CAPABa8h.js。

## 2026-09-25 音乐页人物留白布局

- 仅调整 music-atlus.css：桌面导航、曲目列表、控制区集中到左侧340–480px窄栏；背景固定在视口内，右侧留给人物。取消背景大字叠印，保留斜切菜单、黑白面板、强调色、背景及平台切换动效。
- 手机将背景拆为顶部310–460px展示区，导航和内容位于图片下方，避免长页面cover放大及人物被面板遮挡。没有修改图片或音乐数据。
- 验证：类型检查、4篇内容检查、Vite构建通过；1440px桌面左栏右边缘约530px；390px下背景底部369px、菜单顶部398px，二者不重叠；320px/390px无横向溢出；栏目和来源切换正常，浏览器error/warn为空。未实测真实触屏。
- 本轮构建资源：dist/assets/index-vN_TW_1s.css、dist/assets/index-qg37AtXT.js。没有新增无用试验产物，未删除任何文件。backend/README.md已有修改未触碰。

## 2026-09-25 原目录切换 feat/frontend 与滚动黑边修复

- 按用户要求，先将 feat/backend 的3份未提交修改保存为 stash（说明：before-frontend-switch-2026-09-25: backend notes and music layout），再在原目录切到 feat/frontend。仅将本轮前端CSS和记录的diff补回，保留目标分支已有API接入代码；CSS末尾冲突已合并。前端两文件已暂存但未提交。
- 后端README修改仍在stash中，未将后端改动带入前端分支，未drop stash。
- 背景黑边根因：fixed背景top:64px在页头滚走后仍预留64px。改为fixed + inset:0；手机独立展示区保持原规则。
- 验证：vue-tsc、Vite构建、cached diff检查通过；浏览器滚动351px后背景top=0、bottom=720，与720px视口一致，截图无顶部黑条。当前前端分支每日推荐默认请求Go API；后端不可达时显示真实错误态，本次未改为静默本地回退。
- 保留残留：D:/blog-website/frontend-worktree（已detach，不再占用feat/frontend，无改动）；D:/blog-website/frontend-layout-transfer-20260925.patch（迁移补丁，已应用）。按文件保留约束未清理。
- 本轮构建：dist/assets/index-CspBNrZS.css、dist/assets/index-BrvRNWTL.js；旧构建保留。
