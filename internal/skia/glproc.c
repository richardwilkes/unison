// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#include <stdlib.h>
#include <string.h>
#include "sk_capi.h"

gr_glfunc_ptr GlowGetProcAddress(const char* name);

static gr_glfunc_ptr getProcAddressWrapper(void *ctx, const char *name) {
	return GlowGetProcAddress(name);
}

const gr_glinterface_t *createGoGLInterface(void) {
    return gr_glmake_assembled_interface(NULL, getProcAddressWrapper);
}
