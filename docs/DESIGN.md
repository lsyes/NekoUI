# NekoUI 设计基调

> 这一版的目标不是“功能齐全”，而是把 Go GUI 的骨架、边界和演化方向定对。

## 1. 我们在造什么

NekoUI 是一个 **Linux 桌面 GUI 框架**：

- 负责窗口生命周期、事件归一化、焦点、文本编辑、输入法桥接、布局、显示列表和 OpenGL 渲染。
- 不负责 X11/Wayland compositor、窗口贴边、工作区、合成器特效。
- 不使用 GLFW/SDL/Qt/Fyne/Gio。窗口层直接对接 Xlib / Wayland client API，GPU 层直接对接 EGL/OpenGL。
- 允许系统 ABI 依赖（Xlib、libEGL、libGL、libwayland-client），但不把“框架行为”外包给另一个 GUI 栈。

“自研窗口管理器”在这里的含义是 **windowing layer**：窗口创建、事件循环、输入法、光标、DPI、帧节奏和平台差异的抽象，而不是重写显示服务器。

### 自研边界

| 自研 | 使用系统 ABI |
| --- | --- |
| View/布局/命中/焦点/编辑器/快捷键 | Xlib / libwayland-client 窗口与事件 |
| display list、批处理、SDF、字形/图片图集 | EGL surface/context |
| 主题、控件、Canvas、Scrollable、Image、截图 | OpenGL 3.3 core 驱动 |
| X11 / Wayland / headless 后端编排 | xkbcommon 键映射、系统字体（Noto CJK） |

字体解析是唯一一个非系统 ABI 的第三方依赖：`golang.org/x/image` 的 sfnt/opentype。Wayland 协议绑定由 `wayland-scanner` 从 wayland-protocols 生成后入库，运行时只依赖 libwayland-client / wayland-egl / wayland-cursor / libxkbcommon。字体光栅化不是 GUI 框架的核心创新点，自己重写 TrueType/CFF 解析器只会推迟真正重要的工作。未来如果需要复杂脚本 shaping，会引入 HarfBuzz 或自研 shaper，但仍不会引入 GUI 工具包。

## 2. Go 的 GUI 范式选择

### 候选方案

**A. Retained OO 控件树（Fyne 风格）**

- 控件是长期存在的对象，有生命周期、父子关系、样式状态。
- Go 没有继承，只能靠嵌入和接口模拟，复杂控件的组合会很啰嗦。
- 应用状态和控件状态需要同步；撤销/重放/测试都更重。
- 优点是布局、无障碍和输入法天然有稳定的对象可挂载。

**B. 纯即时模式（Dear ImGui / Gio 风格）**

- UI = f(state)，没有控件对象，组合靠函数调用。
- 状态显式，代码短，天然适合 Go。
- 难点是布局和命中的耦合：控件在构建过程中才知道自己的最终位置，而输入事件在构建前就到了。
- 纯即时模式通常需要“上一帧几何”或者“measure/paint 双跑”。

**C. 声明式树 + diff（React 风格）**

- 每帧构造节点树，做 reconciliation。
- 需要 key、生命周期、hook 规则；对 Go 来说机器太重，且分配压力大。
- 对自绘 GUI 的收益不明显。

### 选择：帧式即时模式（Frame-Immediate）

NekoUI 选择 **B 的 API 形态 + 一层隐藏的帧元数据**：

```go
func View(c *neko.Ctx) {
    if c.Button("+") {
        count++
    }
    c.Label(strconv.Itoa(count))
}
```

- 用户只写普通 Go 函数，直接读写自己的状态。
- 框架不保留控件对象；每帧重建 display list。
- 框架保留的是 **上一帧的命中矩形、焦点、编辑器、快捷键表、滚动位置**等元数据。
- 输入在 View 之前，用上一帧的命中树解析；View 内部直接读 `Response`/`clickedID`。

这解决了即时模式最棘手的“布局未完成时如何命中”的问题：**输入命中使用上一帧的几何，绘制使用当前帧的几何**。在 60/120Hz 下，稳定 ID 让这一帧延迟不可见；窗口 resize 时最多有一帧的命中滞后。

### 五条原则

1. **UI = f(state)**：View 是纯函数，状态属于应用。
2. **身份先于对象**：控件身份由源码位置 + Scope 或显式 ID 决定，跨帧稳定。
3. **输入是帧的一部分**：命中树、焦点、编辑器状态和快捷键表是“帧元数据”，不是控件。
4. **布局只执行一次用户代码**：固定子项立即执行，flex 子项延迟到空间确定后执行一次。
5. **一个 UI 线程拥有平台和 GL**：View、Xlib、EGL、GL 在同一 goroutine，应用状态无需锁。

## 3. 帧生命周期

```text
          ┌─────────────────────────────────────────────┐
          │ 平台事件（X11 / Wayland / headless）          │
          └───────────────────┬─────────────────────────┘
                              ▼
          ┌─────────────────────────────────────────────┐
          │ 归一化 + 用上一帧 HitTree 解析指针/焦点/快捷键 │
          └───────────────────┬─────────────────────────┘
                              ▼
          ┌─────────────────────────────────────────────┐
          │ View(c)：布局、绘制指令、命中注册、编辑器更新  │
          └───────────────────┬─────────────────────────┘
                              ▼
          ┌─────────────────────────────────────────────┐
          │ Frame = DisplayList + HitTree + Focusables   │
          └───────────────────┬─────────────────────────┘
                              ▼
          ┌─────────────────────────────────────────────┐
          │ GPU: 批处理 → GL 3.3 → EGL swap / 截图        │
          └───────────────────┬─────────────────────────┘
                              ▼
                    HitTree 成为下一帧的输入几何
```

一个关键细节：**帧的开始状态是上一帧结束时的 HitTree**，而不是当前帧。输入事件批量到达后，先解析指针下的最上层 Hit，再设置 `hoverID/activeID/focusID/clickedID`，然后 View 读取这些状态。文本编辑器和快捷键在 View 内消费事件，消费标记随帧清除。

### 帧内状态

- 持久的：焦点 ID、编辑器（caret/anchor/scroll）、命中树、focusable 顺序、应用状态。
- 每帧清零的：`clickedID`、`doubleClickedID`、鼠标按下/抬起边沿、滚轮、文本事件、快捷键消费标记。
- 悬停由上一帧 HitTree + 当前指针位置解析；窗口失焦时清空按键/鼠标状态，避免“卡键”。

### 时间与动画

- 没有动画时，事件循环阻塞在平台 fd 上；没有事件就不重绘。
- 调用 `c.Animate()` 或焦点文本编辑器存在时，以约 16ms 超时重绘。
- `c.Elapsed`、`c.DT`、`c.FPS`、`c.Frame` 提供给 View。
- 动画和输入互不抢占，都由同一个循环驱动。

## 4. 分层

```text
┌────────────────────────────────────────────────────────┐
│ 用户：state + func View(*neko.Ctx)                      │
├────────────────────────────────────────────────────────┤
│ 公共 API：Ctx / 布局 / 控件 / Canvas / Theme / Shortcut  │
├────────────────────────────────────────────────────────┤
│ 帧引擎：ID、layer、布局、命中、焦点、编辑器、display list │
├────────────────────────────────────────────────────────┤
│ 渲染器：EGL/GL 3.3、SDF、字形图集、裁剪、批处理、截图      │
├────────────────────────────────────────────────────────┤
│ 平台：X11 / Wayland / headless                           │
└────────────────────────────────────────────────────────┘
```

包结构：

- `nekoui`：唯一需要用户导入的包，只保留公共别名/Option/Config/Run 和原生循环。
- `internal/ui`：帧引擎与内置控件（Ctx、布局、命中、Painter、Scrollable、编辑器）。
- `internal/core`：几何、颜色、按键、事件。无 UI/平台依赖。
- `internal/render`：display list、glyph、atlas 描述。无 GL 依赖。
- `internal/text`：字体加载、字形光栅化、atlas 打包、测量缓存。
- `internal/gpu`：EGL 绑定、GL 3.3 渲染器。
- `internal/platform`：`Window` 接口、X11、Wayland（C shim + 生成的协议绑定）、headless。
- 平台后端不 import 用户 API；渲染器不 import 平台；依赖方向单向。

### 后端契约

```go
type Window interface {
    Size() (w, h int)          // logical
    PixelSize() (w, h int)     // framebuffer
    Scale() float32
    Show()
    SetTitle(string)
    SetCursor(core.Cursor)
    MakeCurrent() error
    Swap() error
    PollEvents(timeout time.Duration) ([]core.Event, error)
    Destroy()
}
```

EGL context 由后端拥有，渲染器只要求“当前上下文已就绪”。这样 X11、Wayland、headless 可以共享同一个 GPU 渲染器，而 context/surface 的创建方式各自不同。

## 5. 布局

### 约束与层

每一帧构造一棵轻量的 `layer` 树：

- `Column`：纵向流，子项宽度由自身决定，交叉轴可对齐。
- `Row`：横向流。
- `Card`：带背景/边框/圆角/内边距的纵向面板。
- `Canvas`：固定尺寸的自绘叶子。
- `Flex`/`Spacer`：主轴剩余空间的权重分配。
- `Fixed`：固定尺寸的裁剪容器。
- 每个 layer 记录 padding、gap、对齐、可用尺寸、裁剪矩形、命中矩形和 display list。

布局是单遍的：容器先执行固定子项的闭包，拿到它们的尺寸；然后计算 flex 剩余空间，执行 flex 子项的闭包一次，再统一计算位置。**用户代码不会为了测量被执行两次**。

布局阶段不再为每个容器复制子树的 ops/hits。所有 layer 在 `finish` 时由 `flattenLayer` 一次遍历生成最终 display list 和命中树，翻译、裁剪求交都在这一遍完成。layer、Painter、Canvas、options 和字形存储按帧复用，避免“每层一次 make”带来的分配与 O(depth) 复制。

### 延迟 flex 的代价

因为固定子项先执行、flex 子项后执行，闭包的副作用顺序不完全等于源码顺序。例如：

```go
c.Row(func() {
    c.Flex(1, func() { c.Label("main") })
    c.Label("side")
})
```

`side` 的闭包会先执行，`main` 后执行。绝大多数 UI 不依赖这个顺序；但如果你在 View 里依赖“先构建 A 再构建 B”的副作用，需要显式拆分或改用固定尺寸。这个取舍换来了正确的 flex 布局和单次执行。

### 布局限制

- 没有 grid、baseline、column-gap 的完整 CSS 语义。
- `Spacer` 会吃掉剩余空间，放在它后面的固定子项可能溢出。
- 自动高度的 Column 内使用纵向 `Spacer` 时，剩余高度可能取决于父级约束；应把这类布局放进有确定高度的 `Fixed`/`Flex` 中。
- 滚动容器（Scrollable）已实现，但虚拟列表、嵌套滚动的惯性尚未实现。

## 6. 输入、焦点、输入法与快捷键

### 指针

- 平台事件归一化为 `MouseMove/Down/Up/Scroll/Enter/Leave`。
- 指针位置转换为逻辑坐标（物理像素 / scale）。
- `hitAt` 从命中树尾部向前找最上层且未被裁剪/禁用的矩形。
- 悬停和按下状态由 `hoverID/activeID` 保存；点击在释放时确认，拖动通过 `activeID` 的本地坐标差分得到。
- Canvas 暴露 `Hovered/Pressed/Clicked/DoubleClicked/Down/Drag/Wheel/Mouse`。
- 光标形状来自命中的 `Cursor`，由平台后端设置。
- 滚轮事件带着指针坐标进入命中树：先通知指针下的 Canvas，再沿 `ScrollID -> parentID` 的滚动链分配位移。内层视口先消费，到边缘后把剩余量交给外层，因此嵌套 Scrollable 不需要额外的手势协调；水平视口只消费水平轴（倾斜滚轮/触控板横向手势）。
- Wayland 的 axis 事件必须把 C shim 里的指针坐标一并写入 `core.Event`，否则命中会被解析到 (0,0)，表现为“滚轮完全无效”。

### 焦点与 Tab

- 点击可聚焦控件会设置 `focusID`，并记录点击的本地位置给文本编辑器定位 caret。
- `Focusable()` 的 Canvas/InputText 会进入 focusable 列表。
- Tab/Shift+Tab 使用**上一帧**的 focusable 顺序移动焦点，事件被标记为已消费，不会进入快捷键。
- 点击空白或不可聚焦区域会清除焦点。
- 窗口失焦会清空按键/鼠标边沿，防止卡键。

### 文本输入

- 文本编辑器状态按 widget ID 挂在 `uiState` 上：caret、anchor、水平滚动、IME 组合串。
- 值本身仍然是用户的 `*string`；框架不复制一份“控件值”。
- 支持：插入、删除、选区、Shift+方向键、Home/End、Ctrl+A/C/X/V、Ctrl+Backspace/Delete、Enter 提交、Esc 失焦。
- 剪贴板当前是应用内的，尚未接入系统 selection。
- 点击定位、左键拖拽选择、双击选词、水平滚动、插入符闪烁、占位符、聚焦边框均已实现。
- 拖拽选择使用 active hit 的本地坐标（指针离开输入框后仍持续更新），越过左右边缘会选中到行首/行尾。
- 文本输入在布局完成后把 caret 矩形发布给后端（`Core.TextInput`），平台据此定位输入法窗口；滚动容器内的编辑器也使用绝对坐标。

### 输入法

X11 使用 XIM，Wayland 使用 text-input-v3；两条路径共用 `EventText / EventPreedit / EventDeleteSurrounding` 和 `SetTextInput` 契约。

X11 关键点：

- `XMODIFIERS` 为空时，从 root 的 `XIM_SERVERS` 读取 `@server=fcitx|ibus`，合成 `@im=` 后再 `XSetLocaleModifiers`；否则 Xlib 会静默退回本地 IM，表现为“输入法完全没反应”。
- **每个事件**（包括 `_XIM_PROTOCOL` ClientMessage）都必须先经过 `XFilterEvent(event, None)`。只过滤 KeyPress/KeyRelease、或把 client window 传进去，都会丢掉 commit 和 forwarded key。
- 优先协商 `XIMPreeditPosition | XIMStatusNothing`（over-the-spot），回退 `XIMPreeditNothing`；`XCreateFontSet` 用于满足 over-the-spot 的协议要求。
- `XNSpotLocation` 使用**客户端窗口相对坐标**（不是 root 坐标）。传 root 坐标会让 fcitx 再加一次窗口原点，正是候选框整体偏移的根因。spot 取 caret 左下角。
- 只有编辑器聚焦且窗口聚焦时才 `XSetICFocus`；失焦/无输入框时取消 IC focus，避免 IME 吞掉快捷键。
- XIM 在合成期间产生的键盘 grab 会发 `FocusOut/NotifyGrab`；这类事件不能当成真实失焦，否则会在合成中途关闭 IC。
- 合成事件（`send_event`）绕过 XIM，方便自动化；真实事件走输入法。提交文本通过 `Xutf8LookupString` 的 `XLookupChars` 状态进入 `EventText`。

Wayland 关键点：

- text-input-v3 的 `enable` 需要 surface enter + keyboard focus；`set_cursor_rectangle` 使用 surface 逻辑坐标，再 `commit`。
- 收到 `preedit_string` 发 `EventPreedit`，收到 `commit_string` 发 `EventText`，`delete_surrounding_text` 发字节数给编辑器。
- preedit 由应用内绘制（组合串 + 下划线），与 fcitx 的候选窗定位解耦。
- 键盘仍用 xkbcommon 生成 UTF-8；preedit 活跃时抑制按键直出，避免和 commit 重复。

### 快捷键

```go
if c.Shortcut(neko.ModCtrl, neko.KeyT) {
    dark = !dark
}
```

- 快捷键在 View 中注册时扫描本帧未消费的 `KeyDown` 事件。
- 文本编辑器先消费自己的编辑快捷键（如 Ctrl+A/C/V/X）；全局快捷键只能看到剩余事件。因此全局快捷键建议放在 View 前部，且避开编辑器占用的组合。
- 支持 Ctrl/Alt/Shift/Super 精确匹配；不处理按住重复触发的节流（重复键事件本身会重复到达）。
- 演示：Ctrl+T 主题、Ctrl+1..4 强调色、F1 动效、Ctrl+Q 退出。

## 7. 渲染

### 后端

- EGL 作为 context 平台：X11 用 `EGL_PLATFORM_X11_KHR`，Wayland 用 `EGL_PLATFORM_WAYLAND_KHR` + `wl_egl_window`，截图用 `EGL_PLATFORM_SURFACELESS_MESA` + pbuffer。
- OpenGL 3.3 core；shader 使用 GLSL 330。
- 4x MSAA 优先，失败自动回退到无 MSAA；SDF 负责矩形/圆角边缘的解析抗锯齿。
- VSync 默认开启（`Config.DisableVSync` 可关）。
- 没有 GLFW/SDL；EGL/GL 符号通过一个很小的手写 C 头文件绑定，避免依赖开发包头文件。

### Display list

一个统一的顶点格式：

```text
position(2) uv(2) color(4) center(2) half(2) radius(1) mode(1) stroke(1)
```

- `mode=rect`：圆角矩形 SDF，`stroke>0` 时变为环/描边。
- `mode=text`：R8 字形图集采样，按 coverage 做 alpha。
- `mode=solid`：自定义三角形/多边形/线条，顶点色插值。
- `mode=image`：RGBA8 纹理采样 + 圆角 SDF mask，支持 tint；图片纹理按 `render.Image` 指针缓存，每帧根据 `Frame.Images` 淘汰未使用纹理。
- 所有 primitive 走同一个 VBO 和 shader，按 display list 顺序提交；裁剪矩形或图片纹理变化时才 flush 并改 scissor/纹理单元。
- 颜色在 shader 中预乘 alpha，blend 使用 `ONE, ONE_MINUS_SRC_ALPHA`，避免半透明边缘发黑。
- 每帧全量重建顶点并 `STREAM_DRAW`；M1 优先正确性，持久 VBO/多缓冲留给性能阶段。
- 帧构建的 display list 是“布局树 → 单次展平”的扁平切片；文本 run 的几何缓存按 `(px, string, scale, atlas generation)` 命中，绘制时只把相对字形复制进帧内 arena 并应用颜色/原点。

### 剪裁与 DPI

- 每个 op 记录 clip 矩形；容器/Canvas 的裁剪在展开 hit/op 时与父裁剪求交。
- 渲染器把逻辑 clip 转换成物理 scissor，并额外扩 1px 防止抗锯齿被裁掉。
- 所有布局尺寸是逻辑像素；EGL surface 是物理像素；顶点位置在提交前乘以 scale。
- 字体按物理像素光栅化（`size * scale`），保证 2x 截图和 HiDPI 下清晰。

### 字形图集

- `x/image/font/opentype` 解析 TTF/OTF/TTC（支持 CFF），Noto Sans CJK SC 提供 Latin+CJK。
- 每个字号缓存一个 `font.Face`；字形按 `(px, rune)` 缓存。
- 字符串测量、字形 advance 与 shaped run 各自带缓存；动态文本超过上限时整体清空，避免计数器/Timer 无限增长。
- 自动 widget ID 只取 `runtime.Callers` 的 PC 和同帧 occurrence，不再拼接 `file:line` 字符串；显式 `ID`/`Scope` 仍按路径哈希。
- 图集 2048×2048 R8，货架式打包，2px padding；只上传 dirty rect。
- 关键坑：`glTexSubImage2D` 上传子矩形时必须设置 `GL_UNPACK_ROW_LENGTH = atlasWidth`，否则 GL 会按子矩形宽度打包行，首帧之后新字形全部错位。渲染器已处理并复位。
- 图集满时重置并重新光栅化；ASCII/CJK 混合文本共用同一图集。

### 文本 shaping

- 当前是“一个 rune 一个 glyph”，不做 kerning、连字、双向文本和复杂脚本 shaping。
- 对拉丁文和 CJK 足够；阿拉伯文/天城文/emoji 组合序列不正确。
- 字体 fallback 链尚未实现；缺字会落到 .notdef。
- 未来：sfnt + HarfBuzz 可选；字形缓存键扩展为 `(font, size, cluster)`。

### 截图

- `Config.Screenshot` + headless backend 渲染 N 帧后 `glReadPixels`。
- 从 back buffer 读；单缓冲 pbuffer 回退到 front buffer。
- 结果翻转行序写 PNG；截图模式默认 2x scale，用于文档/CI。

## 8. 平台后端

### X11（已实现）

- Xlib 连接，窗口 visual 从 EGL config 的 `EGL_NATIVE_VISUAL_ID` 取得，保证 EGL surface 与 X visual 匹配。
- 事件：Expose/Configure/Key/Button/Motion/Enter/Leave/Focus/ClientMessage/Destroy。
- 窗口标题支持 UTF-8 `_NET_WM_NAME`；WM_DELETE_WINDOW 正常关闭；最小尺寸 hints。
- XIM 输入法上下文；`XkbSetDetectableAutoRepeat` 让长按由服务端合并为重复 KeyPress。
- 输入法：自动探测 XMODIFIERS、优先 over-the-spot、窗口相对 spot、grab 焦点过滤。
- DPI 读取 `Xft.dpi`，支持 `NEKOUI_SCALE` 覆盖；逻辑坐标与物理像素分离。
- 光标使用 XCreateFontCursor 缓存。
- 合成事件（`send_event`）绕过 XIM；真实事件仍走输入法。
- 所有 Xlib/EGL/GL 调用在 Run 锁定的同一 OS 线程。

### Wayland（已实现，KWin 验证）

`internal/platform/wayland` 由 C shim 和 Go 适配层组成：

- C shim（`shim.c`）持有 wl_display/registry/compositor/seat/keyboard/pointer/xdg-shell/text-input/cursor 对象、xkb 状态和事件队列；Go 只通过 `nw_*` 扁平 API 消费事件。
- 生成的协议绑定：xdg-shell、xdg-decoration、viewporter、fractional-scale-v1、cursor-shape-v1、text-input-v3、tablet-v2（cursor-shape 的接口依赖）。
- 窗口：xdg_wm_base/xdg_surface/xdg_toplevel，配置 ack/commit，server-side decoration 协商，最小尺寸/关闭，初始 configure 等待。
- EGL：`EGL_PLATFORM_WAYLAND_KHR` + `wl_egl_window_create`，与 X11 共用 GL 渲染器；`eglSwapInterval` 控制 vsync。
- 输入：wl_seat/wl_keyboard + xkbcommon（keymap fd、状态、UTF-8、modifier index）、wl_pointer（enter/leave/motion/button/axis，事件带本地坐标）、光标 cursor-shape-v1（缺失时 wl_cursor shm 回退）。
- 缩放：wl_output scale + fractional-scale-v1 + viewporter；逻辑尺寸来自 configure，物理 buffer = logical × scale，`set_destination` 负责分数缩放。
- 输入法：text-input-v3，支持 enable/disable、commit、preedit、delete-surrounding-text、cursor rectangle。
- 限制：系统剪贴板（wl_data_device）尚未接入；GNOME/sway 仍需冒烟；未显式使用 wl_surface.frame callbacks。

### Headless

- surfaceless EGL + pbuffer；无窗口、无输入。
- 用于文档截图和渲染回归；不能替代真实窗口的输入/IME 测试。

## 9. API 设计

### 为什么用 Option 函数

```go
c.Button("保存", neko.Primary(), neko.Width(120))
c.Label("提示", neko.TextSize(11), neko.TextColor(c.Theme.TextMuted))
c.Card(func() { ... }, neko.Padding(18), neko.Gap(10))
```

- Go 没有命名参数/默认参数；Option 函数是标准库和生态里最常见的可扩展模式。
- 顺序敏感（后面的覆盖前面的），简单可预测。
- 不为每个控件维护一个巨大的配置结构体。

### 为什么用 Canvas 而不是继承 Widget

```go
c.Canvas(220, 120, func(cv *neko.Canvas) {
    cv.RoundRect(cv.Bounds, 16, c.Theme.PanelAlt)
    cv.Circle(neko.Point{X: 110, Y: 60}, 30, c.Theme.Accent)
    if cv.Clicked {
        // ...
    }
})
```

- Go 的组合优于继承；Canvas 把 Painter + Response + 尺寸交给用户。
- 内置控件只是 Canvas/leaf 的便捷封装，不是必须经过的基类。
- 需要完全自定义的控件时，不需要实现一套 Widget 接口。

### 身份

- 默认 ID 来自 `runtime.Caller` 的 file:line + 同帧 occurrence。
- 循环/辅助函数中的控件使用 `neko.ID("...")` 或 `c.Scope("item-3", func(){...})`。
- 焦点、编辑器、hover、点击都按 ID 关联；ID 稳定是正确性的前提。
- 自动 ID 让普通代码无需关心 key；动态列表必须显式给 key，这与 React 的 key 思路一致但更轻。

### 状态与重绘

- 没有 `SetState` / 脏标记；View 每帧读当前状态。
- 无动画时由输入事件触发重绘；有动画时由 16ms tick 触发。
- 这避免了“状态改了但忘了 invalidate”的一整类 bug，代价是每帧重建 display list（M1 完全可接受）。

## 10. 取舍与已知限制

- **Wayland 仅 KWin 验证**：GNOME/sway/嵌套合成器矩阵尚未跑；wl_data_device 未接。
- **X11 preedit 非应用内绘制**：over-the-spot 的预编辑由输入法绘制；若 IM 只支持 PreeditNothing，候选框可能按鼠标而非 caret 定位。
- **系统剪贴板未接**：Ctrl+C/V 目前只在应用内生效。
- **单窗口、单 UI 线程**：没有多窗口共享 context/资源。
- **Scrollable 无虚拟化/惯性**：内容少时够用；超大列表需等虚拟列表。
- **无无障碍树**：Canvas 自绘对屏幕阅读器不可见；后续需要语义节点和平台 bridge。
- **无 damage tracking**：整窗重绘，适合 M1，不适合大窗口高刷。
- **字体 shaping 简化**：复杂脚本不支持；CJK/Latin 正常。
- **上一帧命中**：resize 后最多一帧的指针命中滞后；稳定 ID 下不可感知。
- **延迟 flex**：闭包副作用顺序与源码顺序不完全一致。
- **构建约束**：Linux + cgo；`-l:libEGL.so.1`/`-l:libGL.so.1` 是 GNU ld 语法，主要面向 glibc 发行版。
- **编译器注意事项**：Go 1.27/loong64 在特定内联/`-race` 组合下会破坏小聚合值。我们目前有两处显式规避：
  1. `layout.place/leaf` 加 `//go:noinline`，否则传入的 `Size` 可能丢掉一个 float32 字段（Checkbox 宽 0、Radio 高 8、Slider 宽 128）。
  2. `Button` 的三元 `Color` 返回值改为通过指针写出，否则 `-race` 构建会把 RGBA 字段写歪（正常构建不受影响）。
  3. `Radio` 的填充圆走 `circlePtr`（地址化颜色）而不是直接内联 `Circle`，否则 `-race` 下填充色会变成黑色。
  用户代码如果遇到类似“颜色/小结构体捕获后字段被清零”的现象，应优先怀疑工具链，而不是布局。

## 11. 验证方式

- `go test ./...` 与 `go test -race ./...`：几何、字体 shaping、display list、Button 颜色、控件尺寸、Slider 同帧反馈、嵌套滚轮链。
- headless 截图：像素级检查布局、字体图集、SDF、图像纹理、主题。
- X11 冒烟：真实窗口、EGL context、XIM、事件日志；`examples/ime` + XTest 验证中文提交与候选窗位置。
- Wayland 冒烟：KWin 下 `-backend wayland -screenshot` 验证 xdg-shell/EGL/缩放/指针；text-input-v3 协议路径通过 fcitx 日志验证。
- 单元测试：Scrollable 滚轮/命中/拖动、Image display list、Checkbox/Radio/Slider、拖拽/双击选择、IME caret 发布。
- 输入自动化：XSendEvent/XTest 合成鼠标/键盘，验证 Tab 聚焦、文本输入、快捷键、Canvas 点击、Ctrl+Q。
  - KDE/Wayland 下 XTest 会触发“远程控制输入设备”授权，自动化测试改用 XSendEvent；合成按键绕过 XIM。
- 未自动化：真实中文 IME 候选选择（依赖桌面输入法状态和授权）的 Wayland 交互；GNOME/sway 运行时。

## 12. 下一步

见 [ROADMAP.md](ROADMAP.md)。优先级：

1. 系统剪贴板（X11 CLIPBOARD / Wayland data_device）与 Wayland 交互验收。
2. GNOME/sway 冒烟、wl_surface.frame callbacks、CSD fallback。
3. 列表虚拟化、Popup/Overlay/Menu、多窗口。
4. 文本 shaping/fallback 与无障碍语义。
5. 性能：damage、持久 VBO、字形缓存淘汰。
