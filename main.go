package main

import (
	"fmt"
	"rate/rate"
)

func main() {
	var number int32 = 500 // 每second许可数量
	var second int32 = 3   // 秒

	boolRate := rate.NewRate(number, second)
	boolRate.SetDebug()

	for i:=0;i<2000;i++ {
		_, err := boolRate.GetPermission()
		if err != nil {

		}
	}

	for key, val := range boolRate.Count {
		fmt.Printf("循环批次：%d -- wait：%d -- retry：%d -- done：%d\n", key, val.Wait, val.Retry, val.Done)
	}
}
