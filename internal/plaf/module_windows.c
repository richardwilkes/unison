#include "platform.h"

#if defined(GLFW_BUILD_WIN32_MODULE)

//////////////////////////////////////////////////////////////////////////
//////                       GLFW platform API                      //////
//////////////////////////////////////////////////////////////////////////

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

#endif // GLFW_BUILD_WIN32_MODULE
