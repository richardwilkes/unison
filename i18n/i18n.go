package i18n

import (
	"sync"

	"github.com/richardwilkes/toolbox/i18n"
)

var (
	Callback func(string) string
	once     sync.Once
)

func Text(text string) string {
	once.Do(func() {
		if Callback == nil {
			Callback = i18n.Text
		}
	})
	return Callback(text)
}
