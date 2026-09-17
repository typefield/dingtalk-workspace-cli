/*
 * msvcrt 工具链下的 _vsnprintf 语义适配层（仅供 msvcrt 构建）。
 *
 * libsafechat.a 的成员以两种形态引用 _vsnprintf，并均按 C99/UCRT 语义
 * 使用它：截断时返回需要的长度，且保证 buffer 以 NUL 结尾：
 *   - certParser/pcnative/safechat：普通函数符号 `U _vsnprintf`；
 *   - cryptlib（OpenSSL 预编译）：dllimport 数据符号 `__imp__vsnprintf`
 *     （间接调用 `call *__imp__vsnprintf`）。
 * msvcrt.dll 的 _vsnprintf 在截断时返回 -1 且不保证 NUL 终止，直接满足
 * 引用会造成缓冲区语义不匹配与内存损坏。
 *
 * 这里把两种符号都重定向到 libmingwex 的自包含 C99 实现
 * __mingw_vsnprintf（基于 mingwex 自带的格式化引擎，不回落到 msvcrt 的
 * _vsnprintf），语义与 UCRT 原生一致。
 *
 * 同时定义 `__imp__vsnprintf` 还有另一个作用：一旦该引用由本文件满足，
 * 链接器就不会为解析它而拉入 libmsvcrt.a 中同时定义 `_vsnprintf` 的
 * 导入成员，避免与本文件的 `_vsnprintf` 定义发生重复定义冲突。
 *
 * 环路说明：实现体必须直接调用 __mingw_vsnprintf，绝不能调用 vsnprintf。
 * 在 msvcrt 工具链且 __USE_MINGW_ANSI_STDIO=0 时，vsnprintf 解析为
 * libmingwex 的 __ms_vsnprintf，其内部会回调 _vsnprintf，一旦从本文件
 * 的 _vsnprintf 实现经过 vsnprintf，就会形成
 *   _vsnprintf -> vsnprintf -> __ms_vsnprintf -> _vsnprintf
 * 的无限递归环，CGO 初始化时即耗尽线程栈（Windows 异常 0xc00000fd）。
 * 直接调用 __mingw_vsnprintf 则在任何 __USE_MINGW_ANSI_STDIO 设置下都
 * 不会成环。
 *
 * UCRT 工具链（_UCRT 已定义）下本文件为空：UCRT 原生 _vsnprintf 即
 * C99 语义，无需任何适配。
 */
#if defined(__x86_64__) && !defined(_UCRT)

#include <stdarg.h>
#include <stddef.h>

extern int __mingw_vsnprintf(char *buffer, size_t count, const char *format,
                             va_list args);

/* 满足普通函数符号引用：U _vsnprintf */
int safechat_msvcrt_vsnprintf(char *buffer, size_t count, const char *format,
                              va_list args) __asm__("_vsnprintf");
int safechat_msvcrt_vsnprintf(char *buffer, size_t count, const char *format,
                              va_list args) {
  return __mingw_vsnprintf(buffer, count, format, args);
}

/* 满足 dllimport 数据符号引用：__imp__vsnprintf */
typedef int (*safechat_vsnprintf_fn)(char *, size_t, const char *, va_list);

safechat_vsnprintf_fn safechat_imp_vsnprintf __asm__("__imp__vsnprintf") =
    safechat_msvcrt_vsnprintf;

#endif /* defined(__x86_64__) && !defined(_UCRT) */
