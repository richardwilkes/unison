#if defined(PLAF_WINDOWS)

#include "platform.h"

void* _glfwPlatformLoadModule(const char* path)
{
    return LoadLibraryA(path);
}

void _glfwPlatformFreeModule(void* module)
{
    FreeLibrary((HMODULE) module);
}

moduleFunc _glfwPlatformGetModuleSymbol(void* module, const char* name)
{
    return (moduleFunc) GetProcAddress((HMODULE) module, name);
}

#endif
