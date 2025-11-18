#if defined(__linux__)

#include "platform.h"
#include <dlfcn.h>

void* _glfwPlatformLoadModule(const char* path)
{
    return dlopen(path, RTLD_LAZY | RTLD_LOCAL);
}

void _glfwPlatformFreeModule(void* module)
{
    dlclose(module);
}

moduleFunc _glfwPlatformGetModuleSymbol(void* module, const char* name)
{
    return dlsym(module, name);
}

#endif
