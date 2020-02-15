// Package log provides logger
package log

import (
	"sync"

	cl "github.com/arutselvan15/estore-common/log"
	gLog "github.com/arutselvan15/go-utils/log"
)

var (
	once        sync.Once
	logInstance gLog.CommonLog
)

// GetLogger returns a singleton of the Log object
func GetLogger() gLog.CommonLog {
	once.Do(func() {
		logInstance = cl.GetLogger("product").SetComponent("webhook")
	})

	return logInstance
}
