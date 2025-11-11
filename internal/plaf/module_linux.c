#include "platform.h"

#if defined(GLFW_BUILD_POSIX_MODULE)

#include <dlfcn.h>

//////////////////////////////////////////////////////////////////////////
//////                       GLFW platform API                      //////
//////////////////////////////////////////////////////////////////////////

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

#endif // GLFW_BUILD_POSIX_MODULE
