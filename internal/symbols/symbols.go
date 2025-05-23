package symbols

import (
	"github.com/traefik/yaegi/stdlib"
	"reflect"
)

// 全局符号注册表
var Symbols = map[string]map[string]reflect.Value{}

func init() {
	// 加载标准库符号
	for pkg, symbols := range map[string]string{
		"os/exec": "os/exec",
	} {
		Symbols[pkg] = stdlib.Symbols[symbols]
	}
}
