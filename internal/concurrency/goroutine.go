package concurrency

import "fmt"

func SafeGo(fn func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("error: %+v", err)
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
