package concurrency

import "github.com/xyzbit/minitaskx/pkg/log"

func SafeGo(fn func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Error("%+v", err)
			}
		}()
		fn()
	}()
}

func SafeGoWithRecoverFunc(fn func(), recoverFn func(error)) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				e, _ := err.(error)
				recoverFn(e)
			}
		}()
		fn()
	}()
}
