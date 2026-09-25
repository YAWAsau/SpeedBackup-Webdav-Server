//go:build !windows && !linux

package console

import (
	"fmt"
	"os"
)

func hideInput(*os.File) (func(), error)   { return nil, fmt.Errorf("unsupported terminal") }
func enableOutput(*os.File) (func(), bool) { return func() {}, false }
func dimensions(*os.File) (int, int)       { return 80, 24 }
