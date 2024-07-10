// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"sync"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// GlobalClipboard holds the global clipboard.
var GlobalClipboard = &Clipboard{}

// ClipboardData holds a type and data pair.
type ClipboardData struct {
	Data any
	Type string
}

// Clipboard provides access to the system clipboard as well as an internal, application-only, clipboard. Currently, due
// to limitations in the underlying glfw libraries, only strings may be set onto and retrieved from the system
// clipboard. The internal clipboard accepts any type of data and is passed around via interface. Due to this, you may
// want to consider serializing and unserializing your data into bytes to pass it through the clipboard, to avoid
// accidental mutations.
type Clipboard struct {
	data map[string]any
	lock sync.RWMutex
}

// GetText returns text from the current clipboard data. This reads from the system clipboard.
func (c *Clipboard) GetText() string {
	return glfw.GetClipboardString()
}

// SetText sets text as the current clipboard data. This modifies the system clipboard.
func (c *Clipboard) SetText(str string) {
	glfw.SetClipboardString(str)
	c.lock.Lock()
	c.data = nil
	c.lock.Unlock()
}

// GetData returns the data associated with the specified type on the application clipboard and does not examine the
// system clipboard at all.
func (c *Clipboard) GetData(dataType string) (data any, exists bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	data, exists = c.data[dataType]
	return
}

// SetData the data for a single type on the application clipboard, clearing out any others that were previously
// present. If the data can be converted to text by .(string), it will also be set onto the system clipboard, otherwise,
// the system clipboard will be cleared.
func (c *Clipboard) SetData(dataType string, data any) {
	c.lock.Lock()
	c.data = make(map[string]any)
	c.data[dataType] = data
	c.lock.Unlock()
	if s, ok := data.(string); ok {
		glfw.SetClipboardString(s)
	} else {
		glfw.SetClipboardString("")
	}
}

// SetMultipleData sets the data for multiple types onto the application clipboard, clearing out any others that were
// previously present. The first one that is convertable to text via .(string) will be used to set the system clipboard
// value. If none is found, then the system clipboard will be cleared.
func (c *Clipboard) SetMultipleData(pairs []ClipboardData) {
	var str string
	c.lock.Lock()
	c.data = make(map[string]any)
	for _, p := range pairs {
		c.data[p.Type] = p.Data
		if str == "" {
			if s, ok := p.Data.(string); ok {
				str = s
			}
		}
	}
	c.lock.Unlock()
	glfw.SetClipboardString(str)
}
