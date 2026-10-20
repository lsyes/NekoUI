#ifndef NEKOUI_EGL_API_H
#define NEKOUI_EGL_API_H

#include <stdint.h>
#include <stddef.h>

typedef void* EGLDisplay;
typedef void* EGLConfig;
typedef void* EGLSurface;
typedef void* EGLContext;
typedef void* EGLNativeDisplayType;
typedef void* EGLNativeWindowType;
typedef int32_t EGLint;
typedef uint32_t EGLBoolean;
typedef uint32_t EGLenum;

#define EGL_FALSE 0
#define EGL_TRUE 1

#define EGL_DEFAULT_DISPLAY ((EGLNativeDisplayType)0)
#define EGL_NO_DISPLAY ((EGLDisplay)0)
#define EGL_NO_CONTEXT ((EGLContext)0)
#define EGL_NO_SURFACE ((EGLSurface)0)

#define EGL_SUCCESS 0x3000
#define EGL_NOT_INITIALIZED 0x3001
#define EGL_BAD_ACCESS 0x3002
#define EGL_BAD_ALLOC 0x3003
#define EGL_BAD_ATTRIBUTE 0x3004
#define EGL_BAD_CONFIG 0x3005
#define EGL_BAD_CONTEXT 0x3006
#define EGL_BAD_CURRENT_SURFACE 0x3007
#define EGL_BAD_DISPLAY 0x3008
#define EGL_BAD_MATCH 0x3009
#define EGL_BAD_NATIVE_PIXMAP 0x300A
#define EGL_BAD_NATIVE_WINDOW 0x300B
#define EGL_BAD_PARAMETER 0x300C
#define EGL_BAD_SURFACE 0x300D
#define EGL_CONTEXT_LOST 0x300E

#define EGL_VENDOR 0x3053
#define EGL_VERSION 0x3054
#define EGL_EXTENSIONS 0x3055
#define EGL_CLIENT_APIS 0x308D
#define EGL_HEIGHT 0x3056
#define EGL_WIDTH 0x3057
#define EGL_NONE 0x3038

#define EGL_RED_SIZE 0x3024
#define EGL_GREEN_SIZE 0x3023
#define EGL_BLUE_SIZE 0x3022
#define EGL_ALPHA_SIZE 0x3021
#define EGL_DEPTH_SIZE 0x3025
#define EGL_STENCIL_SIZE 0x3026
#define EGL_SAMPLE_BUFFERS 0x3032
#define EGL_SAMPLES 0x3031

#define EGL_SURFACE_TYPE 0x3033
#define EGL_WINDOW_BIT 0x0004
#define EGL_PBUFFER_BIT 0x0001
#define EGL_RENDERABLE_TYPE 0x3040
#define EGL_OPENGL_BIT 0x0008
#define EGL_NATIVE_VISUAL_ID 0x302E
#define EGL_NATIVE_RENDERABLE 0x302D

#define EGL_OPENGL_API 0x30A2
#define EGL_OPENGL_ES_API 0x30A0
#define EGL_CONTEXT_MAJOR_VERSION 0x3098
#define EGL_CONTEXT_MINOR_VERSION 0x30FB
#define EGL_CONTEXT_OPENGL_PROFILE_MASK 0x30FD
#define EGL_CONTEXT_OPENGL_CORE_PROFILE_BIT 0x00000001
#define EGL_CONTEXT_OPENGL_COMPATIBILITY_PROFILE_BIT 0x00000002

#define EGL_PLATFORM_X11_KHR 0x31D5
#define EGL_PLATFORM_WAYLAND_KHR 0x31D8
#define EGL_PLATFORM_SURFACELESS_MESA 0x31DD

#define EGL_RENDER_BUFFER 0x3086
#define EGL_BACK_BUFFER 0x3084
#define EGL_SINGLE_BUFFER 0x3085

#define EGL_SWAP_BEHAVIOR 0x3093
#define EGL_BUFFER_DESTROYED 0x3095
#define EGL_BUFFER_PRESERVED 0x3094

extern EGLDisplay eglGetDisplay(EGLNativeDisplayType display_id);
extern EGLDisplay eglGetPlatformDisplay(EGLenum platform, void *native_display, const EGLint *attrib_list);
extern EGLBoolean eglInitialize(EGLDisplay dpy, EGLint *major, EGLint *minor);
extern EGLBoolean eglTerminate(EGLDisplay dpy);
extern EGLBoolean eglBindAPI(EGLenum api);
extern const char *eglQueryString(EGLDisplay dpy, EGLint name);
extern EGLint eglGetError(void);
extern EGLBoolean eglChooseConfig(EGLDisplay dpy, const EGLint *attrib_list, EGLConfig *configs, EGLint config_size, EGLint *num_config);
extern EGLBoolean eglGetConfigAttrib(EGLDisplay dpy, EGLConfig config, EGLint attribute, EGLint *value);
extern EGLSurface eglCreateWindowSurface(EGLDisplay dpy, EGLConfig config, EGLNativeWindowType win, const EGLint *attrib_list);
extern EGLSurface eglCreatePbufferSurface(EGLDisplay dpy, EGLConfig config, const EGLint *attrib_list);
extern EGLContext eglCreateContext(EGLDisplay dpy, EGLConfig config, EGLContext share_context, const EGLint *attrib_list);
extern EGLBoolean eglDestroySurface(EGLDisplay dpy, EGLSurface surface);
extern EGLBoolean eglDestroyContext(EGLDisplay dpy, EGLContext ctx);
extern EGLBoolean eglMakeCurrent(EGLDisplay dpy, EGLSurface draw, EGLSurface read, EGLContext ctx);
extern EGLBoolean eglSwapBuffers(EGLDisplay dpy, EGLSurface surface);
extern EGLBoolean eglSwapInterval(EGLDisplay dpy, EGLint interval);
extern EGLBoolean eglReleaseThread(void);
extern void *eglGetProcAddress(const char *procname);

#endif
