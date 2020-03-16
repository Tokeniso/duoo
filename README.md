Go的一个令牌桶，类队列派发许可，用于资源有限或者被限制请求的情况下使用。

```go
package main

import (
	"fmt"
	
	"github.com/Tokeniso/duoo"
)

func main() {
	var number int32 = 500 // 每second许可数量
	var second int32 = 3   // 秒

	boolRate := duoo.RateStartWithDebug(number, second)

	for i:=0;i<2000;i++ {
		_, err := boolRate.GetPermission()
		if err != nil {

		}
	}

	for key, val := range boolRate.Count {
		fmt.Printf("循环批次：%d -- wait：%d -- retry：%d -- done：%d\n", key, val.Wait, val.Retry, val.Done)
	}
}


```

输出

```
启动中....
启动完成.
refresh channel, tag: 0
refresh channel, tag: 1
refresh channel, tag: 2
refresh channel, tag: 3
循环批次：0 -- wait：501 -- retry：1 -- done：500
循环批次：1 -- wait：501 -- retry：1 -- done：500
循环批次：2 -- wait：501 -- retry：1 -- done：500
循环批次：3 -- wait：500 -- retry：0 -- done：500

Process finished with exit code 2

```