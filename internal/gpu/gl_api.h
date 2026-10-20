#ifndef NEKOUI_GL_API_H
#define NEKOUI_GL_API_H

#include <stdint.h>
#include <stddef.h>
#include <stdlib.h>

typedef uint32_t GLenum;
typedef uint8_t GLboolean;
typedef uint32_t GLbitfield;
typedef int32_t GLint;
typedef uint32_t GLuint;
typedef int32_t GLsizei;
typedef float GLfloat;
typedef char GLchar;
typedef intptr_t GLintptr;
typedef intptr_t GLsizeiptr;
typedef uint8_t GLubyte;

#define GL_FALSE 0
#define GL_TRUE 1
#define GL_NO_ERROR 0

#define GL_COLOR_BUFFER_BIT 0x00004000
#define GL_DEPTH_TEST 0x0B71
#define GL_CULL_FACE 0x0B44
#define GL_BLEND 0x0BE2
#define GL_SCISSOR_TEST 0x0C11

#define GL_SRC_ALPHA 0x0302
#define GL_ONE_MINUS_SRC_ALPHA 0x0303
#define GL_ONE 1

#define GL_TRIANGLES 0x0004
#define GL_FLOAT 0x1406

#define GL_ARRAY_BUFFER 0x8892
#define GL_STREAM_DRAW 0x88E0
#define GL_DYNAMIC_DRAW 0x88E8

#define GL_VERTEX_SHADER 0x8B31
#define GL_FRAGMENT_SHADER 0x8B30
#define GL_COMPILE_STATUS 0x8B81
#define GL_LINK_STATUS 0x8B82
#define GL_INFO_LOG_LENGTH 0x8B84

#define GL_TEXTURE_2D 0x0DE1
#define GL_TEXTURE0 0x84C0
#define GL_TEXTURE1 0x84C1
#define GL_RED 0x1903
#define GL_R8 0x8229
#define GL_RGBA 0x1908
#define GL_UNSIGNED_BYTE 0x1401
#define GL_TEXTURE_MIN_FILTER 0x2801
#define GL_TEXTURE_MAG_FILTER 0x2800
#define GL_TEXTURE_WRAP_S 0x2802
#define GL_TEXTURE_WRAP_T 0x2803
#define GL_LINEAR 0x2601
#define GL_CLAMP_TO_EDGE 0x812F
#define GL_UNPACK_ALIGNMENT 0x0CF5
#define GL_UNPACK_ROW_LENGTH 0x0CF2
#define GL_PACK_ALIGNMENT 0x0D05

#define GL_VERSION 0x1F02
#define GL_RENDERER 0x1F01
#define GL_VENDOR 0x1F00
#define GL_SHADING_LANGUAGE_VERSION 0x8B8C
#define GL_MAX_TEXTURE_SIZE 0x0D33

#define GL_BACK 0x0405
#define GL_FRONT 0x0404

extern const GLubyte *glGetString(GLenum name);
extern void glGetIntegerv(GLenum pname, GLint *params);
extern GLenum glGetError(void);
extern void glViewport(GLint x, GLint y, GLsizei width, GLsizei height);
extern void glClearColor(GLfloat r, GLfloat g, GLfloat b, GLfloat a);
extern void glClear(GLbitfield mask);
extern void glEnable(GLenum cap);
extern void glDisable(GLenum cap);
extern void glBlendFunc(GLenum sfactor, GLenum dfactor);
extern void glScissor(GLint x, GLint y, GLsizei width, GLsizei height);
extern void glPixelStorei(GLenum pname, GLint param);
extern void glReadBuffer(GLenum mode);
extern void glReadPixels(GLint x, GLint y, GLsizei width, GLsizei height, GLenum format, GLenum type, void *pixels);
extern void glFinish(void);

extern void glGenVertexArrays(GLsizei n, GLuint *arrays);
extern void glBindVertexArray(GLuint array);
extern void glDeleteVertexArrays(GLsizei n, const GLuint *arrays);
extern void glGenBuffers(GLsizei n, GLuint *buffers);
extern void glBindBuffer(GLenum target, GLuint buffer);
extern void glBufferData(GLenum target, GLsizeiptr size, const void *data, GLenum usage);
extern void glBufferSubData(GLenum target, GLintptr offset, GLsizeiptr size, const void *data);
extern void glDeleteBuffers(GLsizei n, const GLuint *buffers);
extern void glVertexAttribPointer(GLuint index, GLint size, GLenum type, GLboolean normalized, GLsizei stride, const void *pointer);
extern void glEnableVertexAttribArray(GLuint index);

extern GLuint glCreateShader(GLenum type);
extern void glShaderSource(GLuint shader, GLsizei count, const GLchar *const *string, const GLint *length);
extern void glCompileShader(GLuint shader);
extern void glGetShaderiv(GLuint shader, GLenum pname, GLint *params);
extern void glGetShaderInfoLog(GLuint shader, GLsizei bufSize, GLsizei *length, GLchar *infoLog);
extern void glDeleteShader(GLuint shader);

extern GLuint glCreateProgram(void);
extern void glAttachShader(GLuint program, GLuint shader);
extern void glLinkProgram(GLuint program);
extern void glGetProgramiv(GLuint program, GLenum pname, GLint *params);
extern void glGetProgramInfoLog(GLuint program, GLsizei bufSize, GLsizei *length, GLchar *infoLog);
extern void glUseProgram(GLuint program);
extern void glDeleteProgram(GLuint program);

extern GLint glGetUniformLocation(GLuint program, const GLchar *name);
extern void glUniform1i(GLint location, GLint v0);
extern void glUniform2f(GLint location, GLfloat v0, GLfloat v1);

extern void glGenTextures(GLsizei n, GLuint *textures);
extern void glBindTexture(GLenum target, GLuint texture);
extern void glTexImage2D(GLenum target, GLint level, GLint internalformat, GLsizei width, GLsizei height, GLint border, GLenum format, GLenum type, const void *pixels);
extern void glTexSubImage2D(GLenum target, GLint level, GLint xoffset, GLint yoffset, GLsizei width, GLsizei height, GLenum format, GLenum type, const void *pixels);
extern void glTexParameteri(GLenum target, GLenum pname, GLint param);
extern void glActiveTexture(GLenum texture);
extern void glDeleteTextures(GLsizei n, const GLuint *textures);

extern void glDrawArrays(GLenum mode, GLint first, GLsizei count);

static void ng_vertex_attrib_pointer(GLuint index, GLint size, GLenum type, GLboolean normalized, GLsizei stride, uintptr_t offset) {
    glVertexAttribPointer(index, size, type, normalized, stride, (const void *)offset);
}

static GLuint ng_compile_shader(GLenum type, const GLchar *src, GLint *status, GLchar *log, GLsizei logSize) {
    GLuint s = glCreateShader(type);
    glShaderSource(s, 1, &src, NULL);
    glCompileShader(s);
    glGetShaderiv(s, GL_COMPILE_STATUS, status);
    if (!*status) {
        glGetShaderInfoLog(s, logSize, NULL, log);
    }
    return s;
}

static GLuint ng_link_program(GLuint vs, GLuint fs, GLint *status, GLchar *log, GLsizei logSize) {
    GLuint p = glCreateProgram();
    glAttachShader(p, vs);
    glAttachShader(p, fs);
    glLinkProgram(p);
    glGetProgramiv(p, GL_LINK_STATUS, status);
    if (!*status) {
        glGetProgramInfoLog(p, logSize, NULL, log);
    }
    return p;
}

#endif
