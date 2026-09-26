# 关于页与管理工作台交付记录

- 路由：`/about`、`/admin`，均已加入站点导航。
- FIELD UI：复用 FieldProvider（沿用全站 580ms）、MotionReveal（分组延迟 80ms）、OrbitField（orbit，1400 点，浅色点云）、TechTabs、TechButton、FieldDialog。
- 验证：生产构建、vue-tsc、文章库校验通过；浏览器确认 1280px 桌面与 390px 窄屏布局，未发现横向溢出。
- 交互：搜索无结果状态、草稿保存后刷新恢复、打开草稿聚焦标题、Markdown 预览中的 HTML 转义、Escape 关闭并恢复预览按钮焦点、Tabs 方向键切换、非法 slug 拦截、暂停动态后路由导航均已检查。
- 限制：未实测触屏、系统 reduced-motion 切换和跨浏览器表现；未验证实际下载落盘。继承 FIELD UI 的系统 reduced-motion 支持。后台是本地编辑工具，没有身份认证、云端同步、在线发布或修改现有文章源文件的接口。

## 保留的验证残留（未删除）

1. 当前内置浏览器 `http://127.0.0.1:5173` 的 localStorage：`yhimhas:editor:drafts:v1` 中有一条 `UI 验证草稿（未发布）`，slug 为 `ui-verification-draft`；仅供保存及预览验证，没有发布。
2. 首轮构建后被最终构建替代的 `dist/assets/AdminView-Dh_Zmp8N.css`。
3. 首轮构建后被最终构建替代的 `dist/assets/AboutView-CsMjRS54.js`。
4. 首轮构建后被最终构建替代的 `dist/assets/AdminView-_xCeAU0j.js`。
5. 首轮构建后被最终构建替代的 `dist/assets/index-DRuY_6lr.js`。

构建沿用项目的 `emptyOutDir: false`，没有清理历史产物。上述清单仅列出本次工作新产生且已被替代的文件，不包含原有历史构建文件。
