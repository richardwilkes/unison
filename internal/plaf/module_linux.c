#if defined(__linux__)

#include "platform.h"
#include <dlfcn.h>

void* _plafLoadModule(const char* path)
{
    return dlopen(path, RTLD_LAZY | RTLD_LOCAL);
}

void _plafFreeModule(void* module)
{
    dlclose(module);
}

moduleFunc _plafGetModuleSymbol(void* module, const char* name)
{
    return dlsym(module, name);
}

#endif
