# NekoUI 路线图

状态：**M1 / M2 已完成**（X11 + Wayland 垂直切片）。M3/M4 的部分能力已经落地，见下方勾选。

## M1 — 基调与 X11 垂直切片（完成）

- [x] Frame-Immediate API：View 每帧重建，框架保留命中/焦点/编辑器
- [x] Column / Row / Card / Flex / Spacer / Fixed / Canvas
- [x] 圆角矩形 SDF、描边、线条、多边形、渐变三角形
- [x] EGL + OpenGL 3.3 core 渲染器，4x MSAA，scissor 裁剪
- [x] 字形图集、CJK 字体、HiDPI 缩放、无头截图
- [x] X11 窗口 / 事件 / XIM / 光标 / DPI / VSync
- [x] 文本编辑器：选择、Ctrl 操作、点击定位、插入符闪烁
- [x] 快捷键、Tab 焦点、应用内剪贴板
- [x] 单元测试 + X11 合成输入冒烟
- [x] 展示程序与设计文档

## M2 — Wayland 后端（完成，KWin 验证）

目标：同一份 View/渲染器在 KWin、GNOME Shell、sway 上正常开窗、输入、缩放。

- [x] libwayland-client 连接与 registry
- [x] xdg_wm_base / xdg_surface / xdg_toplevel 生命周期
- [x] EGL_PLATFORM_WAYLAND_KHR + wl_egl_window
- [x] wl_seat / wl_keyboard + xkbcommon 键映射与 UTF-8
- [x] wl_pointer：enter/leave/motion/button/axis
- [x] 帧节奏与 vsync（EGL swap interval；未单独接 wl_surface.frame）
- [x] text-input-v3：enable/commit/preedit/delete-surrounding-text
- [x] 光标：cursor-shape-v1 + wl_cursor shm fallback
- [x] xdg-decoration（请求 server-side）；最小尺寸/关闭
- [x] wl_output scale；fractional-scale-v1 + viewporter
- [ ] wl_data_device 系统剪贴板
- [x] 后端自动选择：Wayland session 默认 Wayland，`NEKOUI_BACKEND=x11` 可回退
- [ ] 冒烟矩阵：KWin ✅ / GNOME / sway；嵌套 compositor 测试

验收：showcase 在 Wayland 下窗口、鼠标、缩放、截图正常；text-input-v3 协议路径已接通，待交互验收。X11 行为不回归。

## M3 — 文本与输入法

- [x] Wayland text-input-v3 行内 preedit（应用内绘制）；X11 为 IM over-the-spot 绘制
- [x] 候选框跟随 caret（X11 XNSpotLocation + Wayland set_cursor_rectangle）
- [x] 拖拽选择、双击选词；单行编辑
- [ ] 系统剪贴板（X11 CLIPBOARD / Wayland data_device）
- [ ] 撤销/重做
- [ ] 多行文本编辑器（换行、垂直滚动）
- [ ] 密码输入、只读模式
- [ ] 字体 fallback 链（Latin/CJK/Emoji）
- [ ] HarfBuzz 可选 shaping；复杂脚本、连字、双向文本
- [ ] 字体子集化与图集淘汰策略

## M4 — 组件与交互系统

- [x] ScrollView（滚轮、滚动条拖动、裁剪）
- [x] 滚轮按命中路由；嵌套视口链式滚动（内层到边缘后交给外层）
- [x] Image（RGBA 纹理、Cover/Contain/Stretch/None、圆角、Tint）
- [x] Checkbox / Radio / Slider / ProgressBar
- [x] 双击选词、拖拽选择（单行编辑器）
- [ ] 虚拟列表与惯性滚动
- [ ] Popup/Overlay/Menu/Tooltip（锚定与外部点击关闭）
- [ ] 多窗口与共享 GL 资源
- [ ] hover 延迟、三击语义
- [ ] 动画原语：tween、spring、transition、布局动画
- [ ] Grid、baseline、intrinsic-size flex 增强
- [ ] Tabs/Table/Select 等更复杂控件
- [ ] 快捷键作用域（窗口/焦点/模态）与用户可配置 keymap

## M5 — 平台与性能

- [x] 帧构建：布局后单次展平 ops/hits，layer/Painter/Canvas/Option 按帧复用
- [x] 文本测量、字形 advance、shaped run 缓存；自动 ID 去掉每帧字符串分配
- [ ] damage tracking + 局部重绘
- [ ] 持久 VBO / 多缓冲 / 批处理合并
- [ ] 字形缓存按 LRU 淘汰；多图集页
- [ ] 渲染线程与 UI 线程分离（在平台允许时）
- [ ] 无障碍语义树 + AT-SPI bridge
- [ ] 输入法候选框定位协议完善
- [ ] 跨平台：Windows/macOS 后端（暂不在近期范围）
- [ ] 测试：headless 像素回归、Wayland 嵌套合成器 CI

## 非目标

- 不做 X11/Wayland compositor 或窗口管理器策略。
- 不内嵌 WebView 作为主要渲染路径。
- 不追求 CSS 全兼容；布局 API 服务于 Go 的显式风格。
- 不在 M1/M2 阶段支持移动端/浏览器。

## 版本建议

- `0.1`：M1（X11 垂直切片）。
- `0.2`：M2 Wayland + Scrollable/Image/表单控件 + 输入法定位（当前）。
- `0.3`：系统剪贴板、行内 preedit 完善、Overlay/多窗口/动画。
- `0.4`：文本 shaping/fallback、无障碍、性能。
- `1.0`：API 冻结、跨 compositor/发行版测试矩阵、文档与示例完善。
