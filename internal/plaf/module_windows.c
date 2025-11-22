#if defined(_WIN32)

#include "platform.h"

void* _plafLoadModule(const char* path)
{
    return LoadLibraryA(path);
}

void _plafFreeModule(void* module)
{
    FreeLibrary((HMODULE) module);
}

moduleFunc _plafGetModuleSymbol(void* module, const char* name)
{
    return (moduleFunc) GetProcAddress((HMODULE) module, name);
}

#endif
